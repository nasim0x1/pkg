package jwt

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var (
	ErrInvalidToken = errors.New("invalid or expired token")
	ErrEmptySecret  = errors.New("jwt secret cannot be empty")
)

type Claims struct {
	UserID      string                 `json:"user_id"`
	TenantID    string                 `json:"tenant_id"`
	Role        string                 `json:"role"`
	Permissions []string               `json:"permissions,omitempty"`
	Scopes      []string               `json:"scopes,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	jwt.RegisteredClaims
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"` // in seconds
	TokenType    string `json:"token_type"`
}

func GenerateTokenPair(secret string, userID, tenantID, role string, permissions, scopes []string, accessTTL, refreshTTL time.Duration) (*TokenPair, error) {
	return GenerateTokenPairWithMetadata(secret, userID, tenantID, role, permissions, scopes, nil, accessTTL, refreshTTL)
}

func GenerateTokenPairWithMetadata(secret string, userID, tenantID, role string, permissions, scopes []string, metadata map[string]interface{}, accessTTL, refreshTTL time.Duration) (*TokenPair, error) {
	if secret == "" {
		return nil, ErrEmptySecret
	}

	if tenantID == "" {
		tenantID = "default"
	}

	now := time.Now().UTC()
	accessExpiry := now.Add(accessTTL)
	refreshExpiry := now.Add(refreshTTL)

	accessClaims := Claims{
		UserID:      userID,
		TenantID:    tenantID,
		Role:        role,
		Permissions: permissions,
		Scopes:      scopes,
		Metadata:    metadata,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(accessExpiry),
			ID:        uuid.New().String(),
		},
	}

	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessString, err := accessToken.SignedString([]byte(secret))
	if err != nil {
		return nil, err
	}

	refreshClaims := Claims{
		UserID:   userID,
		TenantID: tenantID,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(refreshExpiry),
			ID:        uuid.New().String(),
		},
	}

	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshString, err := refreshToken.SignedString([]byte(secret))
	if err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  accessString,
		RefreshToken: refreshString,
		ExpiresIn:    int64(accessTTL.Seconds()),
		TokenType:    "Bearer",
	}, nil
}

func ValidateToken(secret, tokenString string) (*Claims, error) {
	if secret == "" {
		return nil, ErrEmptySecret
	}

	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return []byte(secret), nil
	})

	if err != nil {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}
