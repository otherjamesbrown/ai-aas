package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// Policy describes authorization rules keyed by method:path → allowed roles.
type Policy struct {
	Rules map[string][]string `json:"rules"`
}

// Engine evaluates requests against an in-memory policy.
type Engine struct {
	allowed map[string]map[string]struct{}
}

// LoadPolicyFromFile loads a JSON policy bundle from disk.
func LoadPolicyFromFile(path string) (*Engine, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open policy bundle: %w", err)
	}
	defer file.Close()
	return LoadPolicy(file)
}

// LoadPolicy loads a policy from any reader.
func LoadPolicy(r io.Reader) (*Engine, error) {
	var policy Policy
	if err := json.NewDecoder(r).Decode(&policy); err != nil {
		return nil, fmt.Errorf("decode policy bundle: %w", err)
	}
	engine := &Engine{allowed: map[string]map[string]struct{}{}}
	for resource, roles := range policy.Rules {
		key := strings.ToUpper(resource)
		set := engine.allowed[key]
		if set == nil {
			set = map[string]struct{}{}
			engine.allowed[key] = set
		}
		for _, role := range roles {
			set[strings.ToLower(role)] = struct{}{}
		}
	}
	return engine, nil
}

// Allowed returns true when the supplied roles satisfy the rule for the action.
func (e *Engine) Allowed(action string, roles []string) bool {
	if e == nil {
		return false
	}
	set := e.allowed[strings.ToUpper(action)]
	if len(set) == 0 {
		return false
	}
	for _, role := range roles {
		if _, ok := set[strings.ToLower(strings.TrimSpace(role))]; ok {
			return true
		}
	}
	return false
}
