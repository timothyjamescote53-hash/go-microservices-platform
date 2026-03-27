package main

import "testing"

func TestCreateUser(t *testing.T) {
	svc := &UserService{store: NewStore()}
	u := svc.CreateUser("test@example.com", "John", "Doe")
	if u.ID == "" { t.Error("expected ID") }
	if u.Email != "test@example.com" { t.Error("wrong email") }
	if u.FirstName != "John" { t.Error("wrong first name") }
}

func TestGetUser_NotFound(t *testing.T) {
	svc := &UserService{store: NewStore()}
	_, err := svc.GetUser("nonexistent")
	if err != ErrNotFound { t.Fatalf("expected ErrNotFound, got %v", err) }
}

func TestGetUser_Found(t *testing.T) {
	svc := &UserService{store: NewStore()}
	u := svc.CreateUser("a@b.com", "A", "B")
	got, err := svc.GetUser(u.ID)
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	if got.ID != u.ID { t.Error("ID mismatch") }
}

func TestUpdateUser(t *testing.T) {
	svc := &UserService{store: NewStore()}
	u := svc.CreateUser("a@b.com", "Old", "Name")
	updated, err := svc.UpdateUser(u.ID, "New", "Name")
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	if updated.FirstName != "New" { t.Error("first name not updated") }
}

func TestUpdateUser_NotFound(t *testing.T) {
	svc := &UserService{store: NewStore()}
	_, err := svc.UpdateUser("bad-id", "A", "B")
	if err != ErrNotFound { t.Fatalf("expected ErrNotFound, got %v", err) }
}

func TestDeleteUser(t *testing.T) {
	svc := &UserService{store: NewStore()}
	u := svc.CreateUser("a@b.com", "A", "B")
	if err := svc.DeleteUser(u.ID); err != nil { t.Fatalf("unexpected error: %v", err) }
	_, err := svc.GetUser(u.ID)
	if err != ErrNotFound { t.Error("expected user to be deleted") }
}

func TestDeleteUser_NotFound(t *testing.T) {
	svc := &UserService{store: NewStore()}
	err := svc.DeleteUser("bad-id")
	if err != ErrNotFound { t.Fatalf("expected ErrNotFound, got %v", err) }
}
// tests
// not found
// crud
// errors
// create
// get found
// get not found
// update
