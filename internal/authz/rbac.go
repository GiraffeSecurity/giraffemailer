package authz

import (
	"context"

	"github.com/GiraffeSecurity/giraffemailer/internal/domain"
	"github.com/GiraffeSecurity/giraffemailer/internal/port"
)

func EnsureAccountAccess(ctx context.Context, accounts port.AccountRepository, accountID string) error {
	sub, ok := Subject(ctx)
	if !ok {
		return domain.ErrUnauthorized
	}
	if sub.IsAdmin() {
		return nil
	}
	ownerID, err := accounts.GetOwnerID(ctx, accountID)
	if err != nil {
		return err
	}
	if !sub.CanAccessOwner(ownerID) {
		return domain.ErrForbidden
	}
	return nil
}

func AccessibleAccountIDs(ctx context.Context, accounts port.AccountRepository) ([]string, error) {
	sub, ok := Subject(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}
	if sub.IsAdmin() {
		return nil, nil
	}
	return accounts.ListOwnedIDs(ctx, sub.UserID)
}
