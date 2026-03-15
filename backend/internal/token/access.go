package token

import (
	"fmt"
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

func (m *AccessTokenManager) IssueAccessToken(identity Identity) (string, int64, error) {
	now := time.Now()
	expiresAt := now.Add(m.accessTokenDuration)
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
	accessToken, err := token.SignedString(m.accessTokenSecret)
	if err != nil {
		return "", 0, err
	}

	return accessToken, int64(m.accessTokenDuration.Seconds()), nil
}

func (m *AccessTokenManager) ParseAccessToken(accessToken string) (*Identity, error) {
	token, err := jwt.ParseWithClaims(accessToken, &accessTokenClaims{}, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method: %s", token.Method.Alg())
		}
		return m.accessTokenSecret, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*accessTokenClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid access token")
	}

	userID, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil {
		return nil, err
	}

	return &Identity{
		UserID:   userID,
		Username: claims.Username,
		IsAdmin:  claims.IsAdmin,
	}, nil
}
