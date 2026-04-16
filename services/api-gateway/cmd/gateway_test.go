package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRateLimiter_AllowsUnderLimit(t *testing.T) {
	rl := NewRateLimiter(100, 10)
	for i := 0; i < 10; i++ {
		if !rl.Allow("1.2.3.4") {
			t.Errorf("request %d should be allowed", i)
		}
	}
}

func TestRateLimiter_BlocksOverLimit(t *testing.T) {
	rl := NewRateLimiter(1, 2)
	rl.Allow("1.2.3.4") // consume both burst tokens
	rl.Allow("1.2.3.4")
	if rl.Allow("1.2.3.4") {
		t.Error("should be rate limited after burst exhausted")
	}
}

func TestRateLimiter_DifferentIPs(t *testing.T) {
	rl := NewRateLimiter(1, 1)
	if !rl.Allow("1.1.1.1") { t.Error("1.1.1.1 should be allowed") }
	if !rl.Allow("2.2.2.2") { t.Error("2.2.2.2 should be allowed (different IP)") }
}

func TestGateway_HealthCheck(t *testing.T) {
	gw := NewGateway(map[string]string{})
	req := httptest.NewRequest("GET", "/healthz/live", nil)
	req.RemoteAddr = "1.2.3.4:5678"
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestGateway_NotFound(t *testing.T) {
	gw := NewGateway(map[string]string{})
	req := httptest.NewRequest("GET", "/nonexistent", nil)
	req.RemoteAddr = "1.2.3.4:5678"
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestGateway_RateLimited(t *testing.T) {
	gw := &Gateway{
		limiter: NewRateLimiter(1, 1),
		routes:  map[string]string{},
	}
	// Exhaust rate limit
	req1 := httptest.NewRequest("GET", "/healthz/live", nil)
	req1.RemoteAddr = "5.5.5.5:1234"
	gw.limiter.Allow("5.5.5.5") // consume the 1 burst token

	req := httptest.NewRequest("GET", "/healthz/live", nil)
	req.RemoteAddr = "5.5.5.5:1234"
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", w.Code)
	}
}

func TestMin(t *testing.T) {
	if min(3.0, 5.0) != 3.0 { t.Error("expected 3.0") }
	if min(5.0, 3.0) != 3.0 { t.Error("expected 3.0") }
	if min(4.0, 4.0) != 4.0 { t.Error("expected 4.0") }
}
// rate limit test
// routing test
// rate
// routing
// ip
