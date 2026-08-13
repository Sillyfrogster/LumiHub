package media

type Role string

const (
	Avatar           Role = "avatar"
	Expression       Role = "expression"
	Gallery          Role = "gallery"
	AvatarAlt        Role = "avatar_alt"
	PerspectiveLayer Role = "perspective_layer"
)

func (role Role) Valid() bool {
	switch role {
	case Avatar, Expression, Gallery, AvatarAlt, PerspectiveLayer:
		return true
	default:
		return false
	}
}
