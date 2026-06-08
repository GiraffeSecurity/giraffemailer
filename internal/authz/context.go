package authz

import (
	"context"

	"github.com/GiraffeSecurity/giraffemailer/internal/domain"
)

type ctxKey struct{}

func WithSubject(ctx context.Context, sub domain.Subject) context.Context {
	return context.WithValue(ctx, ctxKey{}, sub)
}

func Subject(ctx context.Context) (domain.Subject, bool) {
	sub, ok := ctx.Value(ctxKey{}).(domain.Subject)
	return sub, ok && sub.UserID != ""
}

func MustSubject(ctx context.Context) domain.Subject {
	sub, ok := Subject(ctx)
	if !ok {
		return domain.Subject{}
	}
	return sub
}
