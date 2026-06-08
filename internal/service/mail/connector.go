package mail

import (
	"context"
	"fmt"

	"github.com/GiraffeSecurity/giraffemailer/internal/mailconn"
	"github.com/GiraffeSecurity/giraffemailer/internal/port"
)

type Connector struct {
	accounts port.AccountRepository
	vault    port.CredentialVault
}

func NewConnector(accounts port.AccountRepository, vault port.CredentialVault) *Connector {
	return &Connector{accounts: accounts, vault: vault}
}

func (c *Connector) Open(ctx context.Context, accountID string) (*mailconn.Client, error) {
	creds, err := c.accounts.GetIMAPCredentials(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("account lookup: %w", err)
	}
	password, err := c.vault.DecryptPassword(creds.CredEnc)
	if err != nil {
		return nil, fmt.Errorf("decrypt creds: %w", err)
	}
	cl, err := mailconn.Connect(creds.Host, creds.Port, creds.UseTLS, creds.Username, password)
	if err != nil {
		return nil, err
	}
	return cl, nil
}
