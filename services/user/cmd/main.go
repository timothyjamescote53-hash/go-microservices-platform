package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

func newUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// ── Domain ────────────────────────────────────────────────────────────────────

type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	AvatarURL string    `json:"avatar_url"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

var ErrNotFound = errors.New("user not found")

// ── In-memory store ───────────────────────────────────────────────────────────

type Store struct {
	mu    sync.RWMutex
	users map[string]*User // id → user
}

func NewStore() *Store { return &Store{users: make(map[string]*User)} }

func (s *Store) Create(u *User) { s.mu.Lock(); s.users[u.ID] = u; s.mu.Unlock() }

func (s *Store) GetByID(id string) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if u, ok := s.users[id]; ok {
		cp := *u
		return &cp, nil
	}
	return nil, ErrNotFound
}

func (s *Store) Update(u *User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[u.ID]; !ok {
		return ErrNotFound
	}
	s.users[u.ID] = u
	return nil
}

func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[id]; !ok {
		return ErrNotFound
	}
	delete(s.users, id)
	return nil
}

// ── Service ───────────────────────────────────────────────────────────────────

type UserService struct{ store *Store }

func (s *UserService) CreateUser(email, firstName, lastName string) *User {
	u := &User{
		ID: newUUID(), Email: email,
		FirstName: firstName, LastName: lastName,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	s.store.Create(u)
	return u
}

func (s *UserService) GetUser(id string) (*User, error) { return s.store.GetByID(id) }

func (s *UserService) UpdateUser(id, firstName, lastName string) (*User, error) {
	u, err := s.store.GetByID(id)
	if err != nil {
		return nil, err
	}
	u.FirstName, u.LastName, u.UpdatedAt = firstName, lastName, time.Now()
	return u, s.store.Update(u)
}

func (s *UserService) DeleteUser(id string) error { return s.store.Delete(id) }

// ── Handlers ──────────────────────────────────────────────────────────────────

type handler struct{ svc *UserService }

func userIDFromPath(r *http.Request) string {
	return strings.TrimPrefix(r.URL.Path, "/api/v1/users/")
}

func (h *handler) getMe(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	u, err := h.svc.GetUser(userID)
	if errors.Is(err, ErrNotFound) {
		// Auto-create profile on first access
		u = h.svc.CreateUser(r.Header.Get("X-User-Email"), "", "")
	}
	writeJSON(w, http.StatusOK, u)
}

func (h *handler) updateMe(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	var req struct {
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	u, err := h.svc.UpdateUser(userID, req.FirstName, req.LastName)
	if errors.Is(err, ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, u)
}

func (h *handler) getUser(w http.ResponseWriter, r *http.Request) {
	id := userIDFromPath(r)
	u, err := h.svc.GetUser(id)
	if errors.Is(err, ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, u)
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

func main() {
	svc := &UserService{store: NewStore()}
	h := &handler{svc: svc}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/users/me", h.getMe)
	mux.HandleFunc("PUT /api/v1/users/me", h.updateMe)
	mux.HandleFunc("GET /api/v1/users/", h.getUser)
	mux.HandleFunc("GET /healthz/live", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "alive"})
	})
	mux.HandleFunc("GET /healthz/ready", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})

	port := getEnv("HTTP_PORT", "8081")
	srv := &http.Server{Addr: net.JoinHostPort("", port), Handler: mux}
	go func() {
		slog.Info("User service started", "port", port)
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
}
// scaffold
// user domain
// get me
// update me
// delete
// scaffold
// domain
// get me
// update
// delete
// get by id
// scaffold
// domain
// store
// create
// get by id
// update
// delete
// get me
// update me
// get user
// health
// log update
// method handler
// writeJSON
// getenv
// net join
// slog
// metrics endpoint
// context timeout
// signal notify
// scaffold
// domain
// store
// create
// get by id
// update
// delete
// get me
// update me
// get user
// health
// log update
// method handler
// writeJSON
// getenv
// net join
// slog
// metrics endpoint
// context timeout
// signal notify
// avatar url
// auto create
// updated at
// not found msg
// cors header
// version endpoint
// scaffold
// domain
// store
// create
// get by id
// update
