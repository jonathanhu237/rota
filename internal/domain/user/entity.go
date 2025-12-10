package user

import "time"

type User struct {
	ID           string
	Username     string
	PasswordHash string
	Name         string
	Email        string
	IsAdmin      bool
	IsActive     bool
	Version      int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
