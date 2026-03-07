package token

import (
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Identity struct {
	UserID   int64
	Username string
	IsAdmin  bool
}

type AccessTokenManager struct {
	accessTokenSecret   []byte
	accessTokenDuration time.Duration
}

type accessTokenClaims struct {
	Username string `json:"username"`
	IsAdmin  bool   `json:"is_admin"`
	jwt.RegisteredClaims
}

func NewAccessTokenManager(secret string, duration time.Duration) *AccessTokenManager {
	return &AccessTokenManager{
		accessTokenSecret:   []byte(secret),
		accessTokenDuration: duration,
	}
}

func (s *AccessTokenManager) IssueAccessToken(identity Identity) (string, int64, error) {
	now := time.Now()
	expiresAt := now.Add(s.accessTokenDuration)
	claims := accessTokenClaims{
		Username: identity.Username,
		IsAdmin:  identity.IsAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatInt(identity.UserID, 10),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	accessToken, err := token.SignedString(s.accessTokenSecret)
	if err != nil {
		return "", 0, err
	}

	return accessToken, int64(s.accessTokenDuration.Seconds()), nil
}
