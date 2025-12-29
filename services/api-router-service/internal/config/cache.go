// Package config provides BoltDB-based configuration caching.
//
// Purpose:
//
//	This package implements persistent configuration caching using BoltDB.
//	It provides fallback when Config Service is unavailable and enables
//	fast local lookups of routing policies.
//
// Dependencies:
//   - go.etcd.io/bbolt: Embedded key-value database
//
// Key Responsibilities:
//   - Store routing policies persistently
//   - Load policies on startup
//   - Provide fast lookups by organization and model
//   - Handle cache invalidation on updates
package config

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.etcd.io/bbolt"
)

// Cache provides persistent storage for routing policies.
type Cache struct {
	db *bbolt.DB
}

// NewCache creates a new configuration cache using BoltDB.
func NewCache(path string) (*Cache, error) {
	db, err := bbolt.Open(path, 0600, &bbolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("open cache db: %w", err)
	}

	// Create buckets if they don't exist
	err = db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("policies"))
		return err
	})
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("create buckets: %w", err)
	}

	return &Cache{db: db}, nil
}

// Close closes the cache database.
func (c *Cache) Close() error {
	if c.db != nil {
		return c.db.Close()
	}
	return nil
}

// StorePolicy stores a routing policy in the cache.
func (c *Cache) StorePolicy(ctx context.Context, policy *RoutingPolicy) error {
	return c.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("policies"))
		if bucket == nil {
			return fmt.Errorf("policies bucket not found")
		}

		key := cacheKey(policy.OrganizationID, policy.Model)
		data, err := json.Marshal(policy)
		if err != nil {
			return fmt.Errorf("marshal policy: %w", err)
		}

		return bucket.Put([]byte(key), data)
	})
}

// GetPolicy retrieves a routing policy from the cache.
func (c *Cache) GetPolicy(organizationID, model string) (*RoutingPolicy, error) {
	var policy *RoutingPolicy
	err := c.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("policies"))
		if bucket == nil {
			return fmt.Errorf("policies bucket not found")
		}

		key := cacheKey(organizationID, model)
		data := bucket.Get([]byte(key))
		if data == nil {
			return fmt.Errorf("policy not found")
		}

		var p RoutingPolicy
		if err := json.Unmarshal(data, &p); err != nil {
			return fmt.Errorf("unmarshal policy: %w", err)
		}
		policy = &p
		return nil
	})

	return policy, err
}

// LoadPolicies loads all policies from the cache.
func (c *Cache) LoadPolicies(ctx context.Context) ([]*RoutingPolicy, error) {
	var policies []*RoutingPolicy
	err := c.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("policies"))
		if bucket == nil {
			return fmt.Errorf("policies bucket not found")
		}

		return bucket.ForEach(func(k, v []byte) error {
			var policy RoutingPolicy
			if err := json.Unmarshal(v, &policy); err != nil {
				return fmt.Errorf("unmarshal policy: %w", err)
			}
			policies = append(policies, &policy)
			return nil
		})
	})

	return policies, err
}

// GetPolicyByAlias searches for a policy by alias (model name suffix).
// For example, alias "gpt-oss-20b" matches policy with model "unsloth/gpt-oss-20b".
func (c *Cache) GetPolicyByAlias(organizationID, alias string) (*RoutingPolicy, error) {
	var policy *RoutingPolicy
	suffix := "/" + alias
	keyPrefix := organizationID + ":"

	err := c.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("policies"))
		if bucket == nil {
			return fmt.Errorf("policies bucket not found")
		}

		return bucket.ForEach(func(k, v []byte) error {
			key := string(k)
			// Check if this key is for the right org and model ends with alias
			if strings.HasPrefix(key, keyPrefix) {
				var p RoutingPolicy
				if err := json.Unmarshal(v, &p); err != nil {
					return nil // Skip malformed entries
				}
				if strings.HasSuffix(p.Model, suffix) {
					policy = &p
					return fmt.Errorf("found") // Use error to break iteration
				}
			}
			return nil
		})
	})

	// "found" error means we found the policy
	if err != nil && err.Error() == "found" {
		return policy, nil
	}
	if policy != nil {
		return policy, nil
	}
	return nil, fmt.Errorf("no policy found with alias %s", alias)
}

// GetPolicyByExternalName searches for a policy by external name.
// The external name is the name exposed in OpenAI-compatible APIs.
func (c *Cache) GetPolicyByExternalName(organizationID, externalName string) (*RoutingPolicy, error) {
	var policy *RoutingPolicy
	keyPrefix := organizationID + ":"

	err := c.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("policies"))
		if bucket == nil {
			return fmt.Errorf("policies bucket not found")
		}

		return bucket.ForEach(func(k, v []byte) error {
			key := string(k)
			// Check if this key is for the right org
			if strings.HasPrefix(key, keyPrefix) {
				var p RoutingPolicy
				if err := json.Unmarshal(v, &p); err != nil {
					return nil // Skip malformed entries
				}
				// Match by ExternalName if set, otherwise fall back to suffix matching
				if p.ExternalName != "" && p.ExternalName == externalName {
					policy = &p
					return fmt.Errorf("found") // Use error to break iteration
				}
			}
			return nil
		})
	})

	// "found" error means we found the policy
	if err != nil && err.Error() == "found" {
		return policy, nil
	}
	if policy != nil {
		return policy, nil
	}
	return nil, fmt.Errorf("no policy found with external name %s", externalName)
}

// cacheKey generates a cache key for a policy.
func cacheKey(organizationID, model string) string {
	return fmt.Sprintf("%s:%s", organizationID, model)
}
