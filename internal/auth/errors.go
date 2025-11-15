package auth

import "fmt"

var (
	ErrInvalidToken         = fmt.Errorf("invalid token")
	ErrExpiredToken         = fmt.Errorf("expired token")
	ErrInvalidSigningMethod = fmt.Errorf("invalid signing method")
)
