package main

import "testing"

func TestSend_CreatesNotification(t *testing.T) {
	svc := &Service{store: NewStore()}
	n := svc.Send("user-1", TypeEmail, "Hello", "World")
	if n.ID == "" { t.Error("expected ID") }
	if n.UserID != "user-1" { t.Error("wrong user ID") }
	if n.SentAt == nil { t.Error("expected SentAt") }
}

func TestGetByUser_ReturnsCorrect(t *testing.T) {
	svc := &Service{store: NewStore()}
	svc.Send("user-1", TypeEmail, "A", "B")
	svc.Send("user-1", TypeSMS, "C", "D")
	svc.Send("user-2", TypePush, "E", "F")
	notifs := svc.store.GetByUser("user-1")
	if len(notifs) != 2 { t.Errorf("expected 2, got %d", len(notifs)) }
}

func TestGetByUser_Empty(t *testing.T) {
	svc := &Service{store: NewStore()}
	notifs := svc.store.GetByUser("nobody")
	if len(notifs) != 0 { t.Errorf("expected 0, got %d", len(notifs)) }
}
// tests
// tests
// send
// list
// empty
// types
// multiple users
// sent at
// send
// list
// empty
