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
	scopes := []string{"banani", "polashi"}
	metadata := map[string]interface{}{"device_id": "dev-001"}

	tokenPair, err := GenerateTokenPairWithMetadata(secret, userID, tenantID, role, perms, scopes, metadata, 15*time.Minute, 7*24*time.Hour)
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
	if len(claims.Scopes) != 2 || claims.Scopes[0] != "banani" {
		t.Errorf("Scopes mismatch: got %+v", claims.Scopes)
	}
	if claims.Metadata["device_id"] != "dev-001" {
		t.Errorf("Metadata mismatch: got %+v", claims.Metadata)
	}
}
