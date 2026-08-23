package account

import (
	"time"

	"github.com/google/uuid"
)

// Account is a person's identity on Illarin.
type Account struct {
	ID            uuid.UUID
	Handle        string
	Email         *string
	EmailVerified bool
	DiscordLinked bool
	HasPassword   bool
	Role          Role
}

type Role string

const (
	RoleUser      Role = "user"
	RoleModerator Role = "moderator"
	RoleAdmin     Role = "admin"
)

type NSFWVisibility string

const (
	NSFWHidden  NSFWVisibility = "hidden"
	NSFWBlurred NSFWVisibility = "blurred"
	NSFWShown   NSFWVisibility = "shown"
)

type SignUpInput struct {
	Email    string
	Password string
	Handle   string
}

type DiscordProfile struct {
	Subject       string
	Username      string
	Email         string
	EmailVerified bool
}

type DiscordIntent string

const (
	DiscordSignIn DiscordIntent = "sign-in"
	DiscordAttach DiscordIntent = "attach"
)

type DiscordAuthorization struct {
	URL     string
	State   string
	Expires time.Time
}

type DiscordCompletion struct {
	Account        Account
	SessionToken   string
	SessionExpires time.Time
	Intent         DiscordIntent
}

type Profile struct {
	ID                             uuid.UUID
	Handle                         string
	ShowNSFWContributionsOnProfile bool
}
