package domain

const (
	RoleAdmin = "admin"
	RoleUser  = "user"
)

type Subject struct {
	UserID string
	Role   string
}

func (s Subject) IsAdmin() bool {
	return s.Role == RoleAdmin
}

func (s Subject) CanAccessOwner(ownerID string) bool {
	if s.IsAdmin() {
		return true
	}
	return ownerID != "" && ownerID == s.UserID
}
