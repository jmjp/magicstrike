package entities

import (
	"errors"
	"regexp"
	"time"

	"github.com/oklog/ulid/v2"
)

var (
	// ErrInvalidEmail is returned when the email format is invalid.
	ErrInvalidEmail = errors.New("invalid email address")
	// ErrEmailTooLong is returned when the email is too long.
	ErrEmailTooLong = errors.New("email is too long")
	// ErrUsernameRequired is returned when the username is empty.
	ErrUsernameRequired = errors.New("username is required")
	// ErrUsernameTooShort is returned when the username is too short.
	ErrUsernameTooShort = errors.New("username is too short")
	// ErrUsernameTooLong is returned when the username is too long.
	ErrUsernameTooLong = errors.New("username is too long")
	// ErrAvatarTooLong is returned when the avatar URL is too long.
	ErrAvatarTooLong = errors.New("avatar URL is too long")
	// ErrIDTooLong is returned when the ID is too long.
	ErrIDTooLong = errors.New("ID is too long")
	// ErrUserBlocked is returned when validation is run on a blocked user.
	ErrUserBlocked = errors.New("user is blocked")
	// ErrNegativePoints is returned when points are less than zero.
	ErrNegativePoints = errors.New("points cannot be negative")
)

// Const limits for string fields
const (
	minUsernameLen = 3
	maxUsernameLen = 50
	maxEmailLen    = 254
	maxAvatarLen   = 2048
	maxIDLen       = 64
)

// emailRegex is a pre-compiled regular expression for fast, allocation-free email validation.
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

// User represents a user entity optimized for memory alignment (fields ordered by size).
type User struct {
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	Avatar    string    `json:"avatar,omitempty"`
	Points    int       `json:"points"`
	Blocked   bool      `json:"blocked"`
}

// NewUser creates a new User instance and runs basic validation.
func NewUser(email string, username, avatar string) (*User, error) {
	user := &User{
		ID:        ulid.Make().String(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Username:  username,
		Email:     email,
		Avatar:    avatar,
		Points:    0,
		Blocked:   false,
	}

	if err := user.Valid(); err != nil {
		return nil, err
	}

	return user, nil
}

// Valid checks if the user entity is in a valid state.
func (u *User) Valid() error {
	if len(u.ID) > maxIDLen {
		return ErrIDTooLong
	}

	if len(u.Email) > maxEmailLen {
		return ErrEmailTooLong
	}
	if !emailRegex.MatchString(u.Email) {
		return ErrInvalidEmail
	}

	usernameLen := len(u.Username)
	if usernameLen == 0 {
		return ErrUsernameRequired
	}
	if usernameLen < minUsernameLen {
		return ErrUsernameTooShort
	}
	if usernameLen > maxUsernameLen {
		return ErrUsernameTooLong
	}

	if len(u.Avatar) > maxAvatarLen {
		return ErrAvatarTooLong
	}

	if u.Blocked {
		return ErrUserBlocked
	}

	if u.Points < 0 {
		return ErrNegativePoints
	}

	return nil
}
