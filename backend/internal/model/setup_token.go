package model

import (
	"errors"
	"time"
)

type SetupTokenPurpose string

const (
	SetupTokenPurposeInvitation    SetupTokenPurpose = "invitation"
	SetupTokenPurposePasswordReset SetupTokenPurpose = "password_reset"
)

var (
	ErrInvalidToken   = errors.New("invalid token")
	ErrTokenNotFound  = errors.New("token not found")
	ErrTokenExpired   = errors.New("token expired")
	ErrTokenUsed      = errors.New("token used")
	ErrUserNotPending = errors.New("user not pending")
)

type SetupToken struct {
	ID        int64
	UserID    int64
	TokenHash string
	Purpose   SetupTokenPurpose
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}
