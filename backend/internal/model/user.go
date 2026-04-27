package model

import (
	"errors"
	"unicode/utf8"
)

type UserStatus string
type LanguagePreference string
type ThemePreference string

const (
	UserStatusPending  UserStatus = "pending"
	UserStatusActive   UserStatus = "active"
	UserStatusDisabled UserStatus = "disabled"

	LanguagePreferenceZH LanguagePreference = "zh"
	LanguagePreferenceEN LanguagePreference = "en"

	ThemePreferenceLight  ThemePreference = "light"
	ThemePreferenceDark   ThemePreference = "dark"
	ThemePreferenceSystem ThemePreference = "system"
)

var ErrPasswordTooShort = errors.New("password must have at least 8 characters")

type User struct {
	ID                 int64
	Email              string
	PasswordHash       string
	Name               string
	IsAdmin            bool
	Status             UserStatus
	Version            int
	LanguagePreference *LanguagePreference
	ThemePreference    *ThemePreference
}

func ValidatePassword(password string) error {
	if utf8.RuneCountInString(password) < 8 {
		return ErrPasswordTooShort
	}
	return nil
}
