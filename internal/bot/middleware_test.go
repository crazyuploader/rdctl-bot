package bot

import (
	"context"
	"strings"
	"testing"

	"github.com/crazyuploader/rdctl-bot/internal/config"
)

// newTestMiddleware creates a Middleware with the given rate limit settings for testing.
func newTestMiddleware(messagesPerSecond, burst int) *Middleware {
	cfg := &config.Config{
		App: config.AppConfig{
			RateLimit: config.RateLimitConfig{
				MessagesPerSecond: messagesPerSecond,
				Burst:             burst,
			},
		},
	}
	return NewMiddleware(cfg)
}

// TestWaitForRateLimit_Succeeds verifies that WaitForRateLimit returns nil when tokens are available.
func TestWaitForRateLimit_Succeeds(t *testing.T) {
	// Large burst so wait never blocks in a test.
	m := newTestMiddleware(100, 100)

	if err := m.WaitForRateLimit(); err != nil {
		t.Errorf("WaitForRateLimit() returned unexpected error: %v", err)
	}
}

// TestWaitForRateLimit_MultipleCallsSucceed verifies that repeated calls within burst succeed.
func TestWaitForRateLimit_MultipleCallsSucceed(t *testing.T) {
	m := newTestMiddleware(100, 10)

	for i := 0; i < 5; i++ {
		if err := m.WaitForRateLimit(); err != nil {
			t.Errorf("WaitForRateLimit() call %d returned unexpected error: %v", i+1, err)
		}
	}
}

// TestWaitForRateLimitWithContext_Succeeds verifies that WaitForRateLimitWithContext returns nil
// when tokens are available and the context is not cancelled.
func TestWaitForRateLimitWithContext_Succeeds(t *testing.T) {
	m := newTestMiddleware(100, 100)

	if err := m.WaitForRateLimitWithContext(context.Background()); err != nil {
		t.Errorf("WaitForRateLimitWithContext() returned unexpected error: %v", err)
	}
}

// TestWaitForRateLimitWithContext_CancelledContext verifies that a pre-cancelled context causes
// WaitForRateLimitWithContext to return an error wrapping the context cancellation.
func TestWaitForRateLimitWithContext_CancelledContext(t *testing.T) {
	// Very low rate with no burst so any wait would block.
	m := newTestMiddleware(1, 0)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately before calling.

	err := m.WaitForRateLimitWithContext(ctx)
	if err == nil {
		t.Fatal("WaitForRateLimitWithContext() with cancelled context expected error, got nil")
	}
	if !strings.Contains(err.Error(), "rate limit error") {
		t.Errorf("WaitForRateLimitWithContext() error = %q, want it to contain %q", err.Error(), "rate limit error")
	}
}

// TestWaitForRateLimitWithContext_ErrorWrapping verifies the error message is properly wrapped.
func TestWaitForRateLimitWithContext_ErrorWrapping(t *testing.T) {
	m := newTestMiddleware(1, 0)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := m.WaitForRateLimitWithContext(ctx)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// The error should be wrapped with "rate limit error: ..."
	const wantPrefix = "rate limit error: "
	if !strings.HasPrefix(err.Error(), wantPrefix) {
		t.Errorf("error %q does not have expected prefix %q", err.Error(), wantPrefix)
	}
}

// TestWaitForRateLimitWithContext_MultipleCallsSucceed verifies repeated calls within burst succeed.
func TestWaitForRateLimitWithContext_MultipleCallsSucceed(t *testing.T) {
	m := newTestMiddleware(100, 10)

	for i := 0; i < 5; i++ {
		if err := m.WaitForRateLimitWithContext(context.Background()); err != nil {
			t.Errorf("WaitForRateLimitWithContext() call %d returned unexpected error: %v", i+1, err)
		}
	}
}

// TestCheckAuthorization_SuperAdminAllowedAnywhere verifies super admins can use bot regardless of chat.
func TestCheckAuthorization_SuperAdminAllowedAnywhere(t *testing.T) {
	cfg := &config.Config{
		Telegram: config.TelegramConfig{
			AllowedChatIDs: []int64{100},
			SuperAdminIDs:  []int64{999},
		},
		App: config.AppConfig{
			RateLimit: config.RateLimitConfig{
				MessagesPerSecond: 10,
				Burst:             5,
			},
		},
	}
	m := NewMiddleware(cfg)

	// Super admin in an unauthorized chat — should still be allowed.
	allowed, isSuperAdmin := m.CheckAuthorization(9999, 999)
	if !allowed {
		t.Error("super admin should be allowed in any chat")
	}
	if !isSuperAdmin {
		t.Error("user 999 should be recognized as super admin")
	}
}

// TestCheckAuthorization_AllowedChat verifies that users in allowed chats are permitted.
func TestCheckAuthorization_AllowedChat(t *testing.T) {
	cfg := &config.Config{
		Telegram: config.TelegramConfig{
			AllowedChatIDs: []int64{100, 200},
			SuperAdminIDs:  []int64{999},
		},
		App: config.AppConfig{
			RateLimit: config.RateLimitConfig{
				MessagesPerSecond: 10,
				Burst:             5,
			},
		},
	}
	m := NewMiddleware(cfg)

	allowed, isSuperAdmin := m.CheckAuthorization(100, 42)
	if !allowed {
		t.Error("user in allowed chat should be permitted")
	}
	if isSuperAdmin {
		t.Error("regular user should not be identified as super admin")
	}
}

// TestCheckAuthorization_UnauthorizedUserInUnauthorizedChat verifies rejection.
func TestCheckAuthorization_UnauthorizedUserInUnauthorizedChat(t *testing.T) {
	cfg := &config.Config{
		Telegram: config.TelegramConfig{
			AllowedChatIDs: []int64{100},
			SuperAdminIDs:  []int64{999},
		},
		App: config.AppConfig{
			RateLimit: config.RateLimitConfig{
				MessagesPerSecond: 10,
				Burst:             5,
			},
		},
	}
	m := NewMiddleware(cfg)

	allowed, isSuperAdmin := m.CheckAuthorization(9999, 42)
	if allowed {
		t.Error("user in unauthorized chat should not be allowed")
	}
	if isSuperAdmin {
		t.Error("non-admin user should not be identified as super admin")
	}
}