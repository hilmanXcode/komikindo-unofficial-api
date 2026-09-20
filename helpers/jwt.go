package helpers

import (
	"errors"
	"fmt"
	"komikindo-scraper/config"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var ErrInvalidToken = errors.New("token tidak valid")

// JWTClaims adalah isi access token. Sengaja minimal: identitas dan role saja,
// sisanya diambil dari database saat request diproses.
type JWTClaims struct {
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// GenerateAccessToken membuat access token HS256 yang berlaku selama
// config.ACCESS_TOKEN_TTL.
func GenerateAccessToken(userID uint, username, role string) (string, time.Time, error) {
	now := time.Now()
	expiresAt := now.Add(config.ACCESS_TOKEN_TTL)

	claims := JWTClaims{
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatUint(uint64(userID), 10),
			Issuer:    "komikindo-api",
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(config.JWT_SECRET)
	if err != nil {
		return "", time.Time{}, err
	}

	return signed, expiresAt, nil
}

// ParseAccessToken memvalidasi signature, masa berlaku, dan issuer dari token.
func ParseAccessToken(token string) (*JWTClaims, error) {
	claims := &JWTClaims{}

	parsed, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("metode signing tidak didukung: %v", t.Header["alg"])
		}

		return config.JWT_SECRET, nil
	}, jwt.WithIssuer("komikindo-api"), jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))

	if err != nil || !parsed.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// UserID membaca subject token sebagai ID user.
func (c *JWTClaims) UserID() (uint, error) {
	id, err := strconv.ParseUint(c.Subject, 10, 64)
	if err != nil {
		return 0, ErrInvalidToken
	}

	return uint(id), nil
}
