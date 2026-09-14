package model

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

type User struct {
	ID                 string
	Email              string
	Name               string
	IsAdmin            bool
	Status             UserStatus
	Version            int
	LanguagePreference *LanguagePreference
	ThemePreference    *ThemePreference
}
