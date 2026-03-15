package model

import "errors"

type UserStatus string

const (
	UserStatusActive   UserStatus = "active"
	UserStatusDisabled UserStatus = "disabled"
)

var ErrPasswordTooShort = errors.New("password must have at least 8 characters")

type User struct {
	ID           int64
	Email        string
	PasswordHash string
	Name         string
	IsAdmin      bool
	Status       UserStatus
	Version      int
}

func ValidatePassword(password string) error {
	if len(password) < 8 {
		return ErrPasswordTooShort
	}
	return nil
}
