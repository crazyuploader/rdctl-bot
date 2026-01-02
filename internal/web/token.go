package web

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// Role represents the access level of a token
type Role string

const (
	RoleAdmin  Role = "admin"  // Full access - can delete
	RoleViewer Role = "viewer" // View only - cannot delete
)

// Token represents an authentication token for dashboard access
type Token struct {
	ID        string    `json:"id"`
	UserID    int64     `json:"user_id"`
	Username  string    `json:"username"`
	FirstName string    `json:"first_name"`
	Role      Role      `json:"role"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// ExchangeCode represents a short-lived code to exchange for a real token
type ExchangeCode struct {
	Code      string
	TokenID   string
	ExpiresAt time.Time
}

// IsExpired returns true if the token has expired
func (t *Token) IsExpired() bool {
	return time.Now().After(t.ExpiresAt)
}

// IsAdmin returns true if the token has admin role
func (t *Token) IsAdmin() bool {
	return t.Role == RoleAdmin
}

// TokenStore manages in-memory token storage
type TokenStore struct {
	tokens        map[string]*Token
	exchangeCodes map[string]*ExchangeCode
	mu            sync.RWMutex
	expiry        time.Duration
	cleanupTicker *time.Ticker
	stopCleanup   chan struct{}
	stopOnce      sync.Once
}

// NewTokenStore creates a new token store with the specified expiry duration
func NewTokenStore(expiryMinutes int) *TokenStore {
	if expiryMinutes <= 0 {
		expiryMinutes = 60 // Default 1 hour
	}

	ts := &TokenStore{
		tokens:        make(map[string]*Token),
		exchangeCodes: make(map[string]*ExchangeCode),
		expiry:        time.Duration(expiryMinutes) * time.Minute,
		stopCleanup:   make(chan struct{}),
	}

	// Start cleanup goroutine
	ts.cleanupTicker = time.NewTicker(5 * time.Minute)
	go ts.cleanupLoop()

	return ts
}

// GenerateToken creates a new token for the given user
func (ts *TokenStore) GenerateToken(userID int64, username string, firstName string, isAdmin bool) (string, error) {
	// Generate random token ID
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	tokenID := hex.EncodeToString(bytes)

	role := RoleViewer
	if isAdmin {
		role = RoleAdmin
	}

	token := &Token{
		ID:        tokenID,
		UserID:    userID,
		Username:  username,
		FirstName: firstName,
		Role:      role,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(ts.expiry),
	}

	ts.mu.Lock()
	ts.tokens[tokenID] = token
	ts.mu.Unlock()

	return tokenID, nil
}

// GenerateExchangeCode creates a short-lived code to exchange for a token
func (ts *TokenStore) GenerateExchangeCode(tokenID string) (string, error) {
	// Verify token exists first
	ts.mu.RLock()
	_, exists := ts.tokens[tokenID]
	ts.mu.RUnlock()
	if !exists {
		return "", fmt.Errorf("token %s not found", tokenID)
	}

	bytes := make([]byte, 16) // Shorter code for URL
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	code := hex.EncodeToString(bytes)

	ec := &ExchangeCode{
		Code:      code,
		TokenID:   tokenID,
		ExpiresAt: time.Now().Add(1 * time.Minute), // Short lived
	}

	ts.mu.Lock()
	ts.exchangeCodes[code] = ec
	ts.mu.Unlock()

	return code, nil
}

// ExchangeToken validates an exchange code and returns the associated tokenID
func (ts *TokenStore) ExchangeToken(code string) (string, error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	ec, exists := ts.exchangeCodes[code]
	if !exists {
		return "", nil // Not found
	}

	// Delete immediately (one-time use)
	delete(ts.exchangeCodes, code)

	if time.Now().After(ec.ExpiresAt) {
		return "", nil // Expired
	}

	return ec.TokenID, nil
}

// ValidateToken checks if a token is valid and returns it
func (ts *TokenStore) ValidateToken(tokenID string) (*Token, bool) {
	ts.mu.RLock()
	token, exists := ts.tokens[tokenID]
	ts.mu.RUnlock()

	if !exists {
		return nil, false
	}

	if token.IsExpired() {
		// Clean up expired token
		ts.mu.Lock()
		delete(ts.tokens, tokenID)
		ts.mu.Unlock()
		return nil, false
	}

	return token, true
}

// RevokeToken removes a token from the store
func (ts *TokenStore) RevokeToken(tokenID string) {
	ts.mu.Lock()
	delete(ts.tokens, tokenID)
	ts.mu.Unlock()
}

// cleanupLoop periodically removes expired tokens
func (ts *TokenStore) cleanupLoop() {
	for {
		select {
		case <-ts.cleanupTicker.C:
			ts.cleanupExpired()
		case <-ts.stopCleanup:
			ts.cleanupTicker.Stop()
			return
		}
	}
}

// cleanupExpired removes all expired tokens and exchange codes
func (ts *TokenStore) cleanupExpired() {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	now := time.Now()
	for id, token := range ts.tokens {
		if now.After(token.ExpiresAt) {
			delete(ts.tokens, id)
		}
	}

	for code, ec := range ts.exchangeCodes {
		if now.After(ec.ExpiresAt) {
			delete(ts.exchangeCodes, code)
		}
	}
}

// Stop stops the cleanup goroutine
func (ts *TokenStore) Stop() {
	ts.stopOnce.Do(func() {
		close(ts.stopCleanup)
	})
}

// Count returns the number of active tokens
func (ts *TokenStore) Count() int {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	return len(ts.tokens)
}
