package service

import (
	"errors"

	"github.com/jonathanhu237/rota/api/internal/auth/domain"
)

var (
	ErrInvalidInput           = errors.New("invalid input")
	ErrUserNotFound           = errors.New("user not found")
	ErrUserDisabled           = errors.New("user disabled")
)

func validUserID(id string) bool {
	return domain.IsCanonicalUUID(id)
}

func normalizePagination(page, pageSize int) (int, int, error) {
	if page < 0 || pageSize < 0 {
		return 0, 0, ErrInvalidInput
	}
	if page == 0 {
		page = 1
	}
	if pageSize == 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize, nil
}
