package account

import "github.com/google/uuid"

// Account is a person's identity on LumiHub.
type Account struct {
	ID            uuid.UUID
	Handle        string
	Email         *string
	EmailVerified bool
}

type SignUpInput struct {
	Email    string
	Password string
	Handle   string
}

type Profile struct {
	ID     uuid.UUID
	Handle string
}
