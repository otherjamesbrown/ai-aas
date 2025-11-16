// Package oauth implements Fosite OAuth2/OpenID Connect storage and session management.
//
// Purpose:
//
//	This package provides a PostgreSQL-backed implementation of fosite.Storage,
//	enabling OAuth2 authorization code, refresh token, PKCE, and resource owner
//	password credentials flows. Sessions are cached in Redis for performance,
//	with Postgres as the source of truth. The package also includes user
//	authentication via password verification and lockout checks.
//
// Dependencies:
//   - github.com/ory/fosite: OAuth2 framework interfaces and types
//   - github.com/jackc/pgx/v5: PostgreSQL driver for direct queries
//   - internal/storage/postgres: Core data access layer (Store)
//   - internal/security: Password hashing and verification (Argon2id)
//   - SessionCache interface: Abstracts Redis vs no-op caching
//
// Key Responsibilities:
//   - Store implements fosite.Storage for token/session persistence
//   - Authenticate validates user credentials and enforces lockout policies
//   - Session caching via Redis (optional, falls back to no-op)
//   - TTL calculations honor fosite.Config when attached
//
// Requirements Reference:
//   - specs/005-user-org-service/spec.md#US-001 (User Authentication)
//   - specs/005-user-org-service/spec.md#FR-005 (OAuth2 Support)
//   - specs/005-user-org-service/spec.md#NFR-003 (Session Management)
//   - specs/005-user-org-service/spec.md#FR-008 (Account Lockout)
//
// Debugging Notes:
//   - Session lookups check Redis cache first, then Postgres (cache-aside pattern)
//   - Lockout checks prevent authentication if lockout_until > now() (silent failure)
//   - TTL calculations use fosite.Config if attached via AttachConfig, otherwise defaults
//   - Errors are wrapped with context; fosite.ErrNotFound used for auth failures (no user enumeration)
//   - OAuth sessions stored in oauth_sessions table with signature as primary key
//   - Session data includes org_id and user_id for multi-tenancy support
//
// Thread Safety:
//   - Store methods are safe for concurrent use (pgx pool handles concurrency)
//   - SessionCache implementations must be thread-safe
//   - Authenticate uses a goroutine for best-effort last_login_at updates
//
// Error Handling:
//   - Database errors are wrapped with context for traceability
//   - Authentication failures return fosite.ErrNotFound (prevents user enumeration)
//   - Cache misses fall through to Postgres lookup (graceful degradation)
//   - Expired sessions return fosite.ErrNotFound
package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ory/fosite"

	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/security"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/storage/postgres"
)

// Store implements fosite.Storage backed by Postgres and optional cache.
// All OAuth2 token types (authorize codes, access tokens, refresh tokens, PKCE)
// are persisted to the oauth_sessions table and optionally cached in Redis.
type Store struct {
	Store  *postgres.Store
	cache  SessionCache
	config *fosite.Config
}

// NewStoreWithCache constructs an OAuth store with the provided Postgres store and cache.
func NewStoreWithCache(pgStore *postgres.Store, cache SessionCache) *Store {
	return &Store{
		Store: pgStore,
		cache: cache,
	}
}

// AttachConfig wires the Fosité configuration for TTL calculations.
func (s *Store) AttachConfig(cfg *fosite.Config) {
	s.config = cfg
}

// Config exposes the currently attached Fosité configuration.
func (s *Store) Config() *fosite.Config {
	return s.config
}

const (
	tokenTypeAuthorizeCode = "authorize_code"
	tokenTypeAccessToken   = "access_token"
	tokenTypeRefreshToken  = "refresh_token"
	tokenTypePKCE          = "pkce"
)

var tokenTypeToFosite = map[string]fosite.TokenType{
	tokenTypeAuthorizeCode: fosite.AuthorizeCode,
	tokenTypeAccessToken:   fosite.AccessToken,
	tokenTypeRefreshToken:  fosite.RefreshToken,
	tokenTypePKCE:          fosite.AuthorizeCode,
}

// storedRequest captures the fosite request payload for persistence.
type storedRequest struct {
	RequestID         string              `json:"request_id"`
	RequestedAt       time.Time           `json:"requested_at"`
	ClientID          string              `json:"client_id"`
	Form              map[string][]string `json:"form"`
	RequestedScope    []string            `json:"requested_scope"`
	GrantedScope      []string            `json:"granted_scope"`
	RequestedAudience []string            `json:"requested_audience"`
	GrantedAudience   []string            `json:"granted_audience"`
	Session           *Session            `json:"session"`
}

// GetClient is a placeholder until client persistence exists.
func (s *Store) GetClient(_ context.Context, _ string) (fosite.Client, error) {
	return nil, fosite.ErrNotFound
}

func (s *Store) ClientAssertionJWTValid(context.Context, string) error {
	return fosite.ErrNotFound
}

func (s *Store) SetClientAssertionJWT(context.Context, string, time.Time) error {
	return nil
}

// Authenticate validates user credentials and returns the user ID if successful.
// Implements fosite.ResourceOwnerPasswordCredentialsGrantStorage.Authenticate.
//
// This method:
//   - Looks up user by email (case-insensitive)
//   - Checks account lockout status (lockout_until > now())
//   - Verifies account status is "active"
//   - Validates password using Argon2id hashing
//   - Updates last_login_at asynchronously (best-effort, non-blocking)
//
// Security considerations:
//   - All failures return fosite.ErrNotFound to prevent user enumeration
//   - Lockout status is not revealed to callers
//   - Password verification uses constant-time comparison
//
// Returns:
//   - user ID as string on success
//   - fosite.ErrNotFound on authentication failure (user not found, wrong password, locked, inactive)
//   - wrapped database error on query failures
//
// Side effects:
//   - Updates last_login_at in background goroutine (non-blocking)
func (s *Store) Authenticate(ctx context.Context, username, password string) (string, error) {
	fmt.Printf("[AUTHENTICATE START] username=%s, ctx_done=%v\n", username, ctx.Err())
	if username == "" || password == "" {
		fmt.Printf("[AUTHENTICATE] Empty username or password\n")
		return "", fosite.ErrNotFound
	}

	var (
		userID       uuid.UUID
		orgID        uuid.UUID
		status       string
		passwordHash string
		lockoutUntil *time.Time
	)

	fmt.Printf("[AUTHENTICATE] Executing database query for username=%s\n", username)
	err := s.Store.Pool().QueryRow(ctx, `
		SELECT user_id, org_id, status, password_hash, lockout_until
		FROM users
		WHERE email = LOWER($1) AND deleted_at IS NULL
		ORDER BY created_at ASC
		LIMIT 1
	`, username).Scan(&userID, &orgID, &status, &passwordHash, &lockoutUntil)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			fmt.Printf("[AUTHENTICATE] User not found: username=%s\n", username)
			return "", fosite.ErrNotFound
		}
		// Log database errors that cause server_error (non-ErrNotFound errors)
		// This helps debug why browser requests fail while curl succeeds
		// Using fmt.Printf as fallback since Store doesn't have direct logger access
		fmt.Printf("[AUTHENTICATE ERROR] Database query failed for username=%s, error=%v, error_type=%T, ctx_done=%v\n",
			username, err, err, ctx.Err())
		return "", fmt.Errorf("authenticate database query failed: %w", err)
	}
	fmt.Printf("[AUTHENTICATE] Database query succeeded for username=%s, user_id=%s, status=%s\n", username, userID.String(), status)

	// Check if account is locked out (specs/005-user-org-service/spec.md#FR-008)
	if lockoutUntil != nil && lockoutUntil.After(time.Now()) {
		fmt.Printf("[AUTHENTICATE] Account locked for username=%s, lockout_until=%v\n", username, lockoutUntil)
		return "", fosite.ErrNotFound // Don't reveal lockout status
	}

	if !strings.EqualFold(status, "active") {
		fmt.Printf("[AUTHENTICATE] Account not active for username=%s, status=%s\n", username, status)
		return "", fosite.ErrNotFound
	}

	fmt.Printf("[AUTHENTICATE] About to verify password for username=%s, ctx_done=%v\n", username, ctx.Err())
	ok, err := security.VerifyPassword(password, passwordHash)
	if err != nil {
		fmt.Printf("[AUTHENTICATE ERROR] Password verification failed for username=%s, error=%v, error_type=%T, ctx_done=%v\n",
			username, err, err, ctx.Err())
		// Return the actual error (not ErrNotFound) so Fosite wraps it as server_error
		// This helps us see what's actually failing
		return "", fmt.Errorf("password verification error: %w", err)
	}
	if !ok {
		fmt.Printf("[AUTHENTICATE] Password mismatch for username=%s\n", username)
		return "", fosite.ErrNotFound
	}
	fmt.Printf("[AUTHENTICATE] Password verified successfully for username=%s, user_id=%s\n", username, userID.String())

	// Best effort touch of last_login_at (non-blocking, fire-and-forget).
	// This allows the auth flow to complete quickly while still tracking login times.
	go func(id uuid.UUID) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, _ = s.Store.Pool().Exec(ctx, `
			UPDATE users
			SET last_login_at = now(), version = version + 1
			WHERE user_id = $1 AND deleted_at IS NULL
		`, id)
	}(userID)

	return userID.String(), nil
}

// CreateAuthorizeCodeSession stores an authorization code request.
func (s *Store) CreateAuthorizeCodeSession(ctx context.Context, signature string, request fosite.Requester) error {
	return s.storeRequest(ctx, tokenTypeAuthorizeCode, signature, request)
}

// GetAuthorizeCodeSession retrieves an authorization code request.
func (s *Store) GetAuthorizeCodeSession(ctx context.Context, signature string, _ fosite.Requester) (fosite.Requester, error) {
	if entry, err := s.sessionCache().Get(ctx, tokenTypeAuthorizeCode, signature); err == nil && entry != nil {
		if entry.ExpiresAt.IsZero() || entry.ExpiresAt.After(time.Now().UTC()) {
			stored := entry.Request
			return buildFositeRequest(&stored), nil
		}
		_ = s.sessionCache().Delete(ctx, tokenTypeAuthorizeCode, signature)
	}
	return s.fetchRequest(ctx, tokenTypeAuthorizeCode, signature)
}

// InvalidateAuthorizeCodeSession marks the code as used.
func (s *Store) InvalidateAuthorizeCodeSession(ctx context.Context, signature string) error {
	if err := s.deactivateSignature(ctx, tokenTypeAuthorizeCode, signature); err != nil {
		return err
	}
	return s.sessionCache().Delete(ctx, tokenTypeAuthorizeCode, signature)
}

func (s *Store) CreateAccessTokenSession(ctx context.Context, signature string, request fosite.Requester) error {
	return s.storeRequest(ctx, tokenTypeAccessToken, signature, request)
}

func (s *Store) GetAccessTokenSession(ctx context.Context, signature string, _ fosite.Requester) (fosite.Requester, error) {
	if entry, err := s.sessionCache().Get(ctx, tokenTypeAccessToken, signature); err == nil && entry != nil {
		if entry.ExpiresAt.IsZero() || entry.ExpiresAt.After(time.Now().UTC()) {
			stored := entry.Request
			return buildFositeRequest(&stored), nil
		}
		_ = s.sessionCache().Delete(ctx, tokenTypeAccessToken, signature)
	}
	return s.fetchRequest(ctx, tokenTypeAccessToken, signature)
}

func (s *Store) DeleteAccessTokenSession(ctx context.Context, signature string) error {
	if err := s.deactivateSignature(ctx, tokenTypeAccessToken, signature); err != nil {
		return err
	}
	return s.sessionCache().Delete(ctx, tokenTypeAccessToken, signature)
}

func (s *Store) CreateRefreshTokenSession(ctx context.Context, signature string, request fosite.Requester) error {
	return s.storeRequest(ctx, tokenTypeRefreshToken, signature, request)
}

func (s *Store) GetRefreshTokenSession(ctx context.Context, signature string, _ fosite.Requester) (fosite.Requester, error) {
	if entry, err := s.sessionCache().Get(ctx, tokenTypeRefreshToken, signature); err == nil && entry != nil {
		if entry.ExpiresAt.IsZero() || entry.ExpiresAt.After(time.Now().UTC()) {
			stored := entry.Request
			return buildFositeRequest(&stored), nil
		}
		_ = s.sessionCache().Delete(ctx, tokenTypeRefreshToken, signature)
	}
	return s.fetchRequest(ctx, tokenTypeRefreshToken, signature)
}

func (s *Store) DeleteRefreshTokenSession(ctx context.Context, signature string) error {
	if err := s.deactivateSignature(ctx, tokenTypeRefreshToken, signature); err != nil {
		return err
	}
	return s.sessionCache().Delete(ctx, tokenTypeRefreshToken, signature)
}

func (s *Store) RevokeRefreshToken(ctx context.Context, requestID string) error {
	rid, err := uuid.Parse(requestID)
	if err != nil {
		return err
	}
	_, execErr := s.Store.Pool().Exec(ctx, `
		UPDATE oauth_sessions
		SET active = FALSE
		WHERE token_type = $1 AND request_id = $2
	`, tokenTypeRefreshToken, rid)
	if execErr != nil {
		return execErr
	}
	return s.sessionCache().DeleteByRequestID(ctx, tokenTypeRefreshToken, rid)
}

func (s *Store) RevokeRefreshTokenMaybeGracePeriod(ctx context.Context, _ string, requestID string) error {
	return s.RevokeRefreshToken(ctx, requestID)
}

func (s *Store) RevokeAccessToken(ctx context.Context, requestID string) error {
	rid, err := uuid.Parse(requestID)
	if err != nil {
		return err
	}
	_, execErr := s.Store.Pool().Exec(ctx, `
		UPDATE oauth_sessions
		SET active = FALSE
		WHERE token_type = $1 AND request_id = $2
	`, tokenTypeAccessToken, rid)
	if execErr != nil {
		return execErr
	}
	return s.sessionCache().DeleteByRequestID(ctx, tokenTypeAccessToken, rid)
}

func (s *Store) CreatePKCERequestSession(ctx context.Context, signature string, request fosite.Requester) error {
	return s.storeRequest(ctx, tokenTypePKCE, signature, request)
}

func (s *Store) GetPKCERequestSession(ctx context.Context, signature string, _ fosite.Requester) (fosite.Requester, error) {
	if entry, err := s.sessionCache().Get(ctx, tokenTypePKCE, signature); err == nil && entry != nil {
		if entry.ExpiresAt.IsZero() || entry.ExpiresAt.After(time.Now().UTC()) {
			stored := entry.Request
			return buildFositeRequest(&stored), nil
		}
		_ = s.sessionCache().Delete(ctx, tokenTypePKCE, signature)
	}
	return s.fetchRequest(ctx, tokenTypePKCE, signature)
}

func (s *Store) DeletePKCERequestSession(ctx context.Context, signature string) error {
	if err := s.deactivateSignature(ctx, tokenTypePKCE, signature); err != nil {
		return err
	}
	return s.sessionCache().Delete(ctx, tokenTypePKCE, signature)
}

// storeRequest persists a fosite request for the given token type and signature.
func (s *Store) storeRequest(ctx context.Context, tokenType, signature string, requester fosite.Requester) error {
	if requester == nil {
		return errors.New("requester cannot be nil")
	}
	req := requester.Sanitize(nil)
	session, err := toSession(req)
	if err != nil {
		return err
	}

	if req.GetClient() == nil {
		return errors.New("client must be set on request")
	}

	requestID := req.GetID()
	if requestID == "" {
		requestID = uuid.NewString()
		req.SetID(requestID)
	}

	formMap := map[string][]string{}
	for key, values := range req.GetRequestForm() {
		formCopy := make([]string, len(values))
		copy(formCopy, values)
		formMap[key] = formCopy
	}

	stored := storedRequest{
		RequestID:         requestID,
		RequestedAt:       req.GetRequestedAt(),
		ClientID:          req.GetClient().GetID(),
		Form:              formMap,
		RequestedScope:    append([]string{}, req.GetRequestedScopes()...),
		GrantedScope:      append([]string{}, req.GetGrantedScopes()...),
		RequestedAudience: append([]string{}, req.GetRequestedAudience()...),
		GrantedAudience:   append([]string{}, req.GetGrantedAudience()...),
	}
	if sessionClone, ok := session.Clone().(*Session); ok {
		stored.Session = sessionClone
	} else {
		return errors.New("session clone must be *Session")
	}

	formJSON, err := json.Marshal(stored.Form)
	if err != nil {
		return err
	}
	sessionJSON, err := json.Marshal(stored.Session)
	if err != nil {
		return err
	}
	scopesJSON, err := marshalJSONBytes(stored.RequestedScope)
	if err != nil {
		return err
	}
	grantedScopesJSON, err := marshalJSONBytes(stored.GrantedScope)
	if err != nil {
		return err
	}
	audienceJSON, err := marshalJSONBytes(stored.RequestedAudience)
	if err != nil {
		return err
	}
	grantedAudienceJSON, err := marshalJSONBytes(stored.GrantedAudience)
	if err != nil {
		return err
	}
	scopesArg := jsonOrNil(scopesJSON)
	grantedScopesArg := jsonOrNil(grantedScopesJSON)
	audienceArg := jsonOrNil(audienceJSON)
	grantedAudienceArg := jsonOrNil(grantedAudienceJSON)
	formArg := jsonOrNil(formJSON)
	sessionArg := jsonOrNil(sessionJSON)

	var orgUUID, userUUID any
	if session.OrgID != "" {
		if parsed, err := uuid.Parse(session.OrgID); err == nil {
			orgUUID = parsed
		} else {
			return err
		}
	}
	if session.UserID != "" {
		if parsed, err := uuid.Parse(session.UserID); err == nil {
			userUUID = parsed
		} else {
			return err
		}
	}

	expiresAt := stored.Session.GetExpiresAt(tokenTypeToFosite[tokenType])

	_, err = s.Store.Pool().Exec(ctx, `
		INSERT INTO oauth_sessions (
			signature,
			token_type,
			request_id,
			client_id,
			subject,
			org_id,
			user_id,
			scopes,
			granted_scopes,
			audience,
			granted_audience,
			form_data,
			session_data,
			requested_at,
			expires_at,
			active
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,TRUE)
		ON CONFLICT (signature) DO UPDATE SET
			token_type = EXCLUDED.token_type,
			request_id = EXCLUDED.request_id,
			client_id = EXCLUDED.client_id,
			subject = EXCLUDED.subject,
			org_id = EXCLUDED.org_id,
			user_id = EXCLUDED.user_id,
			scopes = EXCLUDED.scopes,
			granted_scopes = EXCLUDED.granted_scopes,
			audience = EXCLUDED.audience,
			granted_audience = EXCLUDED.granted_audience,
			form_data = EXCLUDED.form_data,
			session_data = EXCLUDED.session_data,
			requested_at = EXCLUDED.requested_at,
			expires_at = EXCLUDED.expires_at,
			active = TRUE
	`, signature, tokenType, requestID, stored.ClientID, session.Subject, orgUUID, userUUID,
		scopesArg, grantedScopesArg, audienceArg, grantedAudienceArg,
		formArg, sessionArg, stored.RequestedAt, nullTime(expiresAt))
	if err != nil {
		return err
	}

	ttl := s.ttlFor(tokenType, expiresAt)
	if ttl <= 0 {
		return nil
	}
	cacheExpiresAt := expiresAt
	if cacheExpiresAt.IsZero() {
		cacheExpiresAt = time.Now().UTC().Add(ttl)
	}
	return s.sessionCache().Set(ctx, tokenType, signature, &stored, cacheExpiresAt, ttl)
}

func (s *Store) fetchRequest(ctx context.Context, expectedType, signature string) (fosite.Requester, error) {
	row := s.Store.Pool().QueryRow(ctx, `
		SELECT token_type, request_id, client_id, subject, org_id, user_id,
		       scopes, granted_scopes, audience, granted_audience,
		       form_data, session_data, requested_at, expires_at, active
		FROM oauth_sessions
		WHERE signature = $1
	`, signature)

	var (
		tokenType           string
		requestID           uuid.UUID
		clientID            string
		subject             *string
		orgID               pgtype.UUID
		userID              pgtype.UUID
		scopesJSON          []byte
		grantedScopesJSON   []byte
		audienceJSON        []byte
		grantedAudienceJSON []byte
		formJSON            []byte
		sessionJSON         []byte
		requestedAt         time.Time
		expiresAt           pgtype.Timestamptz
		active              bool
	)

	if err := row.Scan(&tokenType, &requestID, &clientID, &subject, &orgID, &userID,
		&scopesJSON, &grantedScopesJSON, &audienceJSON, &grantedAudienceJSON,
		&formJSON, &sessionJSON, &requestedAt, &expiresAt, &active); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fosite.ErrNotFound
		}
		return nil, err
	}

	if tokenType != expectedType || !active {
		return nil, fosite.ErrNotFound
	}
	if expiresAt.Valid && !expiresAt.Time.After(time.Now().UTC()) {
		return nil, fosite.ErrNotFound
	}

	storedReq, err := decodeStoredRequest(clientID, requestID.String(), requestedAt, scopesJSON, grantedScopesJSON, audienceJSON, grantedAudienceJSON, formJSON, sessionJSON)
	if err != nil {
		return nil, err
	}

	if subject != nil {
		storedReq.Session.Subject = *subject
	}

	if ttl := s.ttlFor(expectedType, expiresAt.Time); ttl > 0 {
		cacheExpires := expiresAt.Time
		if !expiresAt.Valid {
			cacheExpires = time.Now().UTC().Add(ttl)
		}
		_ = s.sessionCache().Set(ctx, expectedType, signature, storedReq, cacheExpires, ttl)
	}

	req := buildFositeRequest(storedReq)
	return req, nil
}

func (s *Store) deactivateSignature(ctx context.Context, tokenType, signature string) error {
	_, err := s.Store.Pool().Exec(ctx, `
		UPDATE oauth_sessions
		SET active = FALSE
		WHERE signature = $1 AND token_type = $2
	`, signature, tokenType)
	return err
}

func (s *Store) sessionCache() SessionCache {
	if s.cache == nil {
		return noopSessionCache{}
	}
	return s.cache
}

func (s *Store) ttlFor(tokenType string, expiresAt time.Time) time.Duration {
	if !expiresAt.IsZero() {
		if ttl := time.Until(expiresAt); ttl > 0 {
			return ttl
		}
	}

	if s != nil && s.config != nil {
		ctx := context.Background()
		switch tokenType {
		case tokenTypeAuthorizeCode, tokenTypePKCE:
			if d := s.config.GetAuthorizeCodeLifespan(ctx); d > 0 {
				return d
			}
		case tokenTypeAccessToken:
			if d := s.config.GetAccessTokenLifespan(ctx); d > 0 {
				return d
			}
		case tokenTypeRefreshToken:
			if d := s.config.GetRefreshTokenLifespan(ctx); d > 0 {
				return d
			}
		}
	}

	switch tokenType {
	case tokenTypeAuthorizeCode, tokenTypePKCE:
		return 10 * time.Minute
	case tokenTypeAccessToken:
		return time.Hour
	case tokenTypeRefreshToken:
		return 24 * time.Hour
	default:
		return time.Hour
	}
}

func decodeStoredRequest(clientID, requestID string, requestedAt time.Time,
	scopesJSON, grantedScopesJSON, audienceJSON, grantedAudienceJSON, formJSON, sessionJSON []byte) (*storedRequest, error) {

	var (
		scopes          []string
		grantedScopes   []string
		audience        []string
		grantedAudience []string
		form            map[string][]string
		session         Session
		err             error
	)

	if scopes, err = jsonToStringSlice(scopesJSON); err != nil {
		return nil, err
	}
	if grantedScopes, err = jsonToStringSlice(grantedScopesJSON); err != nil {
		return nil, err
	}
	if audience, err = jsonToStringSlice(audienceJSON); err != nil {
		return nil, err
	}
	if grantedAudience, err = jsonToStringSlice(grantedAudienceJSON); err != nil {
		return nil, err
	}
	if form, err = jsonToForm(formJSON); err != nil {
		return nil, err
	}
	if len(sessionJSON) > 0 {
		if err = json.Unmarshal(sessionJSON, &session); err != nil {
			return nil, err
		}
	} else {
		session = Session{}
	}

	return &storedRequest{
		RequestID:         requestID,
		RequestedAt:       requestedAt,
		ClientID:          clientID,
		Form:              form,
		RequestedScope:    scopes,
		GrantedScope:      grantedScopes,
		RequestedAudience: audience,
		GrantedAudience:   grantedAudience,
		Session:           &session,
	}, nil
}

func buildFositeRequest(stored *storedRequest) *fosite.Request {
	request := &fosite.Request{
		ID:                stored.RequestID,
		RequestedAt:       stored.RequestedAt,
		Client:            &fosite.DefaultClient{ID: stored.ClientID},
		Form:              urlValuesFromMap(stored.Form),
		RequestedScope:    fosite.Arguments(stored.RequestedScope),
		GrantedScope:      fosite.Arguments(stored.GrantedScope),
		RequestedAudience: fosite.Arguments(stored.RequestedAudience),
		GrantedAudience:   fosite.Arguments(stored.GrantedAudience),
		Session:           stored.Session,
	}
	if request.Session == nil {
		request.Session = &Session{}
	}
	return request
}

func marshalJSONBytes(v any) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return json.Marshal(v)
}

func jsonToStringSlice(value []byte) ([]string, error) {
	if len(value) == 0 {
		return []string{}, nil
	}
	var out []string
	if err := json.Unmarshal(value, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func jsonToForm(value []byte) (map[string][]string, error) {
	if len(value) == 0 {
		return map[string][]string{}, nil
	}
	var out map[string][]string
	if err := json.Unmarshal(value, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func jsonOrNil(data []byte) any {
	if len(data) == 0 {
		return nil
	}
	return string(data)
}

func urlValuesFromMap(m map[string][]string) url.Values {
	values := url.Values{}
	for k, v := range m {
		copyVals := make([]string, len(v))
		copy(copyVals, v)
		values[k] = copyVals
	}
	return values
}

func nullTime(t time.Time) pgtype.Timestamptz {
	if t.IsZero() {
		return pgtype.Timestamptz{Valid: false}
	}
	return pgtype.Timestamptz{Time: t.UTC(), Valid: true}
}

// Helper to build session from fosite.Requester
func toSession(r fosite.Requester) (*Session, error) {
	session, ok := r.GetSession().(*Session)
	if !ok {
		return nil, fmt.Errorf("unexpected session type %T", r.GetSession())
	}
	return session, nil
}
