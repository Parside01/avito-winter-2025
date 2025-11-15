package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSecretKey = "test-secret-key-for-predictable-results"

func TestGenerateToken(t *testing.T) {
	TokenSecretKey = testSecretKey

	tests := []struct {
		name      string
		tokenType TokenType
		duration  time.Duration
	}{
		{
			name:      "success: generate valid user token",
			tokenType: TokenTypeUser,
			duration:  time.Hour,
		},
		{
			name:      "success: generate valid admin token",
			tokenType: TokenTypeAdmin,
			duration:  30 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenString, err := GenerateToken(tt.tokenType, tt.duration)
			require.NoError(t, err)
			require.NotEmpty(t, tokenString)

			claims, err := VerifyToken(tokenString)
			require.NoError(t, err)
			assert.Equal(t, tt.tokenType, claims.Type)
			assert.WithinDuration(t, time.Now().Add(tt.duration), claims.ExpiresAt.Time, time.Second*5)
		})
	}
}

func TestVerifyToken(t *testing.T) {
	TokenSecretKey = testSecretKey

	validUserToken, _ := GenerateToken(TokenTypeUser, time.Hour)

	expiredToken, _ := GenerateToken(TokenTypeUser, -time.Hour)

	claimsWithWrongMethod := TokenClaims{
		Type: TokenTypeUser,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	tokenWithWrongMethod := jwt.NewWithClaims(jwt.SigningMethodNone, claimsWithWrongMethod)
	wrongMethodTokenString, _ := tokenWithWrongMethod.SignedString(jwt.UnsafeAllowNoneSignatureType)

	tests := []struct {
		name              string
		tokenString       string
		secretSetup       func()
		secretRollback    func()
		expectError       bool
		expectedErrorType error
		expectedTokenType TokenType
	}{
		{
			name:              "success: verify valid token",
			tokenString:       validUserToken,
			expectError:       false,
			expectedTokenType: TokenTypeUser,
		},
		{
			name:              "failure: verify expired token",
			tokenString:       expiredToken,
			expectError:       true,
			expectedErrorType: jwt.ErrTokenExpired,
		},
		{
			name:              "failure: verify token with invalid signature",
			tokenString:       validUserToken,
			secretSetup:       func() { TokenSecretKey = "different-secret-key" },
			secretRollback:    func() { TokenSecretKey = testSecretKey },
			expectError:       true,
			expectedErrorType: jwt.ErrTokenSignatureInvalid,
		},
		{
			name:              "failure: verify malformed token",
			tokenString:       "not-a-valid-jwt-token",
			expectError:       true,
			expectedErrorType: jwt.ErrTokenMalformed,
		},
		{
			name:              "failure: verify token with wrong signing method",
			tokenString:       wrongMethodTokenString,
			expectError:       true,
			expectedErrorType: ErrInvalidSigningMethod,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.secretSetup != nil {
				tt.secretSetup()
			}
			if tt.secretRollback != nil {
				defer tt.secretRollback()
			}

			claims, err := VerifyToken(tt.tokenString)

			if tt.expectError {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.expectedErrorType)
				assert.Nil(t, claims)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, claims)
				assert.Equal(t, tt.expectedTokenType, claims.Type)
			}
		})
	}
}

func TestIsValidToken(t *testing.T) {
	TokenSecretKey = testSecretKey

	validAdminToken, _ := GenerateToken(TokenTypeAdmin, time.Hour)
	expiredUserToken, _ := GenerateToken(TokenTypeUser, -time.Hour)

	tests := []struct {
		name              string
		tokenString       string
		expectedOK        bool
		expectedTokenType TokenType
	}{
		{
			name:              "success: valid token",
			tokenString:       validAdminToken,
			expectedOK:        true,
			expectedTokenType: TokenTypeAdmin,
		},
		{
			name:              "failure: expired token",
			tokenString:       expiredUserToken,
			expectedOK:        false,
			expectedTokenType: "",
		},
		{
			name:              "failure: invalid token string",
			tokenString:       "invalid-token",
			expectedOK:        false,
			expectedTokenType: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenType, ok := IsValidToken(tt.tokenString)
			assert.Equal(t, tt.expectedOK, ok)
			assert.Equal(t, tt.expectedTokenType, tokenType)
		})
	}
}
