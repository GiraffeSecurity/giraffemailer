package authz

import (
	"context"
	"testing"

	"github.com/GiraffeSecurity/giraffemailer/internal/domain"
)

func TestSubjectCanAccessOwner(t *testing.T) {
	admin := domain.Subject{UserID: "u1", Role: domain.RoleAdmin}
	user := domain.Subject{UserID: "u2", Role: domain.RoleUser}

	if !admin.CanAccessOwner("other") {
		t.Fatal("admin should access any account")
	}
	if user.CanAccessOwner("u2") != true {
		t.Fatal("user should access own account")
	}
	if user.CanAccessOwner("u3") {
		t.Fatal("user must not access other's account")
	}
}

func TestSubjectFromContext(t *testing.T) {
	ctx := WithSubject(context.Background(), domain.Subject{UserID: "x", Role: domain.RoleUser})
	sub, ok := Subject(ctx)
	if !ok || sub.UserID != "x" {
		t.Fatalf("unexpected subject: %+v ok=%v", sub, ok)
	}
}
