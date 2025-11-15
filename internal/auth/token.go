package auth

import (
	"github.com/golang-jwt/jwt/v5"
	"github.com/pkg/errors"
	"os"
	"time"
)

type TokenType string

const (
	TokenTypeUndefined TokenType = ""
	TokenTypeUser      TokenType = "user"
	TokenTypeAdmin     TokenType = "admin"
)

var TokenSecretKey = os.Getenv("TOKEN_AUTH_SECRET")

type TokenClaims struct {
	Type TokenType `json:"type"`
	jwt.RegisteredClaims
}

func GenerateToken(tokenType TokenType, dur time.Duration) (string, error) {
	claims := TokenClaims{
		Type: tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(dur)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(TokenSecretKey))
}

func VerifyToken(tokenString string) (*TokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.Wrap(ErrInvalidSigningMethod, token.Header["alg"].(string))
		}
		return []byte(TokenSecretKey), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*TokenClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, ErrInvalidToken
}

func IsValidToken(tokenString string) (TokenType, bool) {
	claims, err := VerifyToken(tokenString)
	if err != nil {
		return "", false
	}
	return claims.Type, true
}
