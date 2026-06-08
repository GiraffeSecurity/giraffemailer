package api

import "context"

type contextKey int

const ctxUserID contextKey = 1

func UserID(ctx context.Context) string {
	id, _ := ctx.Value(ctxUserID).(string)
	return id
}

func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, ctxUserID, userID)
}
