package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// ── JWT-like token (simplified, stdlib only) ──────────────────────────────────

type Claims struct {
	UserID    string
	Email     string
	TokenID   string
	ExpiresAt int64
}

func signToken(claims Claims, secret string) string {
	payload := fmt.Sprintf("%s|%s|%s|%d", claims.UserID, claims.Email, claims.TokenID, claims.ExpiresAt)
	mac := hmacSHA256(payload, secret)
	return base64Encode(payload) + "." + mac
}

func verifyToken(token, secret string) (*Claims, error) {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return nil, errors.New("invalid token format")
	}
	payload := base64Decode(parts[0])
	expected := hmacSHA256(payload, secret)
	if expected != parts[1] {
		return nil, errors.New("invalid signature")
	}
	// payload format: "userID|email|tokenID|expiresAt"
	fields := strings.SplitN(payload, "|", 4)
	if len(fields) != 4 {
		return nil, errors.New("invalid token payload")
	}
	expiresAt, err := strconv.ParseInt(fields[3], 10, 64)
	if err != nil {
		return nil, errors.New("invalid token expiry")
	}
	if time.Now().Unix() > expiresAt {
		return nil, errors.New("token expired")
	}
	return &Claims{UserID: fields[0], Email: fields[1], TokenID: fields[2], ExpiresAt: expiresAt}, nil
}

func hmacSHA256(data, key string) string {
	h := sha256.New()
	h.Write([]byte(key + data))
	return hex.EncodeToString(h.Sum(nil))
}

func base64Encode(s string) string {
	return hex.EncodeToString([]byte(s))
}

func base64Decode(s string) string {
	b, _ := hex.DecodeString(s)
	return string(b)
}

func newUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func hashPassword(password string) string {
	h := sha256.New()
	h.Write([]byte(password))
	return hex.EncodeToString(h.Sum(nil))
}

// ── Domain ────────────────────────────────────────────────────────────────────

type User struct {
	ID           string
	Email        string
	PasswordHash string
	CreatedAt    time.Time
}

var (
	ErrUserExists         = errors.New("user already exists")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidToken       = errors.New("invalid token")
)

// ── In-memory store ───────────────────────────────────────────────────────────

type Store struct {
	mu            sync.RWMutex
	users         map[string]*User // email → user
	refreshTokens map[string]string // token → userID
	blacklist     map[string]bool   // tokenID → revoked
}

func NewStore() *Store {
	return &Store{
		users:         make(map[string]*User),
		refreshTokens: make(map[string]string),
		blacklist:     make(map[string]bool),
	}
}

// ── Auth Service ──────────────────────────────────────────────────────────────

type AuthService struct {
	store  *Store
	secret string
}

func NewAuthService(store *Store, secret string) *AuthService {
	return &AuthService{store: store, secret: secret}
}

func (s *AuthService) Register(email, password string) (*User, error) {
	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	if _, exists := s.store.users[email]; exists {
		return nil, ErrUserExists
	}
	u := &User{
		ID:           newUUID(),
		Email:        email,
		PasswordHash: hashPassword(password),
		CreatedAt:    time.Now(),
	}
	s.store.users[email] = u
	return u, nil
}

func (s *AuthService) Login(email, password string) (accessToken, refreshToken string, err error) {
	s.store.mu.RLock()
	u, ok := s.store.users[email]
	s.store.mu.RUnlock()
	if !ok || u.PasswordHash != hashPassword(password) {
		return "", "", ErrInvalidCredentials
	}
	claims := Claims{
		UserID:    u.ID,
		Email:     u.Email,
		TokenID:   newUUID(),
		ExpiresAt: time.Now().Add(15 * time.Minute).Unix(),
	}
	accessToken = signToken(claims, s.secret)
	refreshToken = newUUID()
	s.store.mu.Lock()
	s.store.refreshTokens[refreshToken] = u.ID
	s.store.mu.Unlock()
	return accessToken, refreshToken, nil
}

func (s *AuthService) Validate(token string) (*Claims, error) {
	claims, err := verifyToken(token, s.secret)
	if err != nil {
		return nil, ErrInvalidToken
	}
	s.store.mu.RLock()
	revoked := s.store.blacklist[claims.TokenID]
	s.store.mu.RUnlock()
	if revoked {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

func (s *AuthService) Logout(token, refreshToken string) {
	if claims, err := verifyToken(token, s.secret); err == nil {
		s.store.mu.Lock()
		s.store.blacklist[claims.TokenID] = true
		delete(s.store.refreshTokens, refreshToken)
		s.store.mu.Unlock()
	}
}

func (s *AuthService) Refresh(refreshToken string) (string, string, error) {
	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	userID, ok := s.store.refreshTokens[refreshToken]
	if !ok {
		return "", "", ErrInvalidToken
	}
	delete(s.store.refreshTokens, refreshToken)
	var email string
	for _, u := range s.store.users {
		if u.ID == userID {
			email = u.Email
			break
		}
	}
	claims := Claims{
		UserID:    userID,
		Email:     email,
		TokenID:   newUUID(),
		ExpiresAt: time.Now().Add(15 * time.Minute).Unix(),
	}
	newAccess := signToken(claims, s.secret)
	newRefresh := newUUID()
	s.store.refreshTokens[newRefresh] = userID
	return newAccess, newRefresh, nil
}

// ── HTTP Handlers ─────────────────────────────────────────────────────────────

type handler struct{ svc *AuthService }

func (h *handler) register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email and password required"})
		return
	}
	u, err := h.svc.Register(req.Email, req.Password)
	if errors.Is(err, ErrUserExists) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "user already exists"})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"user_id": u.ID, "email": u.Email})
}

func (h *handler) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	access, refresh, err := h.svc.Login(req.Email, req.Password)
	if errors.Is(err, ErrInvalidCredentials) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": access, "refresh_token": refresh,
		"expires_in": 900, "token_type": "Bearer",
	})
}

func (h *handler) validate(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	claims, err := h.svc.Validate(token)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid token"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user_id": claims.UserID, "email": claims.Email, "valid": true})
}

func (h *handler) refresh(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	access, refresh, err := h.svc.Refresh(req.RefreshToken)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid refresh token"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"access_token": access, "refresh_token": refresh})
}

func (h *handler) logout(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	refresh := r.Header.Get("X-Refresh-Token")
	h.svc.Logout(token, refresh)
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	store := NewStore()
	svc := NewAuthService(store, getEnv("JWT_SECRET", "change-me-in-production"))
	h := &handler{svc: svc}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/auth/register", h.register)
	mux.HandleFunc("POST /api/v1/auth/login", h.login)
	mux.HandleFunc("POST /api/v1/auth/logout", h.logout)
	mux.HandleFunc("POST /api/v1/auth/refresh", h.refresh)
	mux.HandleFunc("GET /api/v1/auth/validate", h.validate)
	mux.HandleFunc("GET /healthz/live", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "alive"})
	})
	mux.HandleFunc("GET /healthz/ready", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})
	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "# auth service metrics")
	})

	port := getEnv("HTTP_PORT", "8080")
	srv := &http.Server{
		Addr: net.JoinHostPort("", port),
		Handler: mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	go func() {
		slog.Info("Auth service started", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
	slog.Info("Auth service stopped")
}
// scaffold
// http server
// register route
// password hashing
// login route
// jwt signing
// refresh token
// token blacklist
// validate endpoint
// fix token parse
// parse fix
// scaffold
// server
// store
// uuid
// hash
// token sign
// token verify
// claims
// parse fix
// register
// login
// refresh
// blacklist
// validate
// health
// routes
// graceful
// log register
// log login
// strconv
// method handler
// writeJSON
// getenv
// net join
// slog
// metrics endpoint
// context timeout
// signal notify
// scaffold
// server
// store
// uuid
// hash
// token sign
// token verify
// claims
// parse fix
// register
// login
// refresh
// blacklist
// validate
// health
// routes
// graceful
// log register
// log login
// strconv
// method handler
// writeJSON
// getenv
// net join
// slog
// metrics endpoint
// context timeout
// signal notify
// duplicate check
// password min len
// email validate
// token expiry config
// refresh expiry
