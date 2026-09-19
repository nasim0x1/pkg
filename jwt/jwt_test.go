package jwt

import (
	"testing"
	"time"
)

func TestJWTTokenGenerationAndValidation(t *testing.T) {
	secret := "super-secret-key-12345"
	userID := "user-123"
	tenantID := "tenant-456"
	role := "sr"
	perms := []string{"order:create", "order:read"}
	markets := []string{"banani", "polashi"}

	tokenPair, err := GenerateTokenPair(secret, userID, tenantID, role, perms, markets, 15*time.Minute, 7*24*time.Hour)
	if err != nil {
		t.Fatalf("Failed to generate token pair: %v", err)
	}

	if tokenPair.AccessToken == "" || tokenPair.RefreshToken == "" {
		t.Fatalf("Generated empty access or refresh token")
	}

	claims, err := ValidateToken(secret, tokenPair.AccessToken)
	if err != nil {
		t.Fatalf("Failed to validate token: %v", err)
	}

	if claims.UserID != userID || claims.TenantID != tenantID || claims.Role != role {
		t.Errorf("Claims mismatch: got %+v", claims)
	}
}
