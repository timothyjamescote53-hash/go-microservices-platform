package main

import (
	"testing"
	"time"
)

func newTestService() *AuthService {
	return NewAuthService(NewStore(), "test-secret")
}

func TestRegister_Success(t *testing.T) {
	svc := newTestService()
	u, err := svc.Register("test@example.com", "password123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.ID == "" {
		t.Error("expected user ID to be set")
	}
	if u.Email != "test@example.com" {
		t.Errorf("expected email test@example.com, got %s", u.Email)
	}
}

func TestRegister_DuplicateEmail(t *testing.T) {
	svc := newTestService()
	svc.Register("dup@example.com", "pass")
	_, err := svc.Register("dup@example.com", "pass")
	if err != ErrUserExists {
		t.Fatalf("expected ErrUserExists, got %v", err)
	}
}

func TestLogin_Success(t *testing.T) {
	svc := newTestService()
	svc.Register("user@example.com", "mypassword")
	access, refresh, err := svc.Login("user@example.com", "mypassword")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if access == "" {
		t.Error("expected access token")
	}
	if refresh == "" {
		t.Error("expected refresh token")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	svc := newTestService()
	svc.Register("user@example.com", "correct")
	_, _, err := svc.Login("user@example.com", "wrong")
	if err != ErrInvalidCredentials {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestLogin_UnknownUser(t *testing.T) {
	svc := newTestService()
	_, _, err := svc.Login("nobody@example.com", "pass")
	if err != ErrInvalidCredentials {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestValidate_Success(t *testing.T) {
	svc := newTestService()
	svc.Register("user@example.com", "pass")
	access, _, _ := svc.Login("user@example.com", "pass")
	claims, err := svc.Validate(access)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims.Email != "user@example.com" {
		t.Errorf("expected email user@example.com, got %s", claims.Email)
	}
}

func TestValidate_InvalidToken(t *testing.T) {
	svc := newTestService()
	_, err := svc.Validate("not.a.valid.token")
	if err != ErrInvalidToken {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

func TestValidate_ExpiredToken(t *testing.T) {
	svc := newTestService()
	// Manually create an expired token
	claims := Claims{
		UserID:    "u1",
		Email:     "a@b.com",
		TokenID:   newUUID(),
		ExpiresAt: time.Now().Add(-1 * time.Hour).Unix(), // expired
	}
	token := signToken(claims, "test-secret")
	_, err := svc.Validate(token)
	if err != ErrInvalidToken {
		t.Fatalf("expected ErrInvalidToken for expired token, got %v", err)
	}
}

func TestLogout_BlacklistsToken(t *testing.T) {
	svc := newTestService()
	svc.Register("user@example.com", "pass")
	access, refresh, _ := svc.Login("user@example.com", "pass")
	svc.Logout(access, refresh)
	_, err := svc.Validate(access)
	if err != ErrInvalidToken {
		t.Error("token should be invalid after logout")
	}
}

func TestRefresh_Success(t *testing.T) {
	svc := newTestService()
	svc.Register("user@example.com", "pass")
	_, refresh, _ := svc.Login("user@example.com", "pass")
	newAccess, newRefresh, err := svc.Refresh(refresh)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if newAccess == "" || newRefresh == "" {
		t.Error("expected new tokens")
	}
}

func TestRefresh_InvalidToken(t *testing.T) {
	svc := newTestService()
	_, _, err := svc.Refresh("invalid-refresh-token")
	if err != ErrInvalidToken {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

func TestRefresh_RotatesToken(t *testing.T) {
	svc := newTestService()
	svc.Register("user@example.com", "pass")
	_, refresh, _ := svc.Login("user@example.com", "pass")
	_, _, _ = svc.Refresh(refresh)
	// Old refresh token should no longer work
	_, _, err := svc.Refresh(refresh)
	if err != ErrInvalidToken {
		t.Error("old refresh token should be invalid after rotation")
	}
}

func TestNewUUID_Unique(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := newUUID()
		if ids[id] {
			t.Errorf("duplicate UUID generated: %s", id)
		}
		ids[id] = true
	}
}

func TestHashPassword_Deterministic(t *testing.T) {
	h1 := hashPassword("mypassword")
	h2 := hashPassword("mypassword")
	if h1 != h2 {
		t.Error("same password should produce same hash")
	}
}

func TestHashPassword_Different(t *testing.T) {
	h1 := hashPassword("password1")
	h2 := hashPassword("password2")
	if h1 == h2 {
		t.Error("different passwords should produce different hashes")
	}
}
// test register
// test login
// test validate
// test logout
// test refresh
// regression test
// regression
// register
// duplicate
// login ok
// wrong pass
// unknown user
// validate ok
// invalid token
// expired
// logout
// refresh ok
// refresh invalid
// rotation
// uuid unique
// hash deterministic
// regression
// hash diff
// concurrent
// base64
// hmac
// register
// duplicate
// login ok
// wrong pass
// unknown user
// validate ok
// invalid token
// expired
// logout
// refresh ok
// refresh invalid
// rotation
// uuid unique
// hash deterministic
// regression
// hash diff
// concurrent
// base64
// hmac
// password len
// email format
// blacklist check
// multi login
// store isolation
// version test
// register
// duplicate
// login ok
// wrong pass
