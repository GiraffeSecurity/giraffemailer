package domain

type MailAccount struct {
	ID           string
	Name         string
	EmailAddress string
	IMAPHost     string
	IMAPPort     int
	UseTLS       bool
	Username     string
	SyncEnabled  bool
	LastSyncAt   *string
}

type CreateAccountInput struct {
	Name         string
	EmailAddress string
	IMAPHost     string
	IMAPPort     int
	UseTLS       bool
	Username     string
	Password     string
}

type IMAPCredentials struct {
	Host     string
	Port     int
	UseTLS   bool
	Username string
	CredEnc  string
}
