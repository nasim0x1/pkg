package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nasim0x1/pkg/jwt"
	"github.com/nasim0x1/pkg/rbac"
)

func TestMiddlewareStack(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /test", func(w http.ResponseWriter, r *http.Request) {
		tenant := GetTenantID(r.Context())
		if tenant != "tenant-1" {
			t.Errorf("expected tenant-1, got %s", tenant)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	handler := Logger(Recoverer(CORS(TenantResolver("default")(mux))))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Tenant-ID", "tenant-1")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("expected CORS header *")
	}
}

func TestAuthGuardAndRBAC(t *testing.T) {
	secret := "test-secret-key-32chars-length-ok!"
	token, err := jwt.GenerateTokenPair(secret, "u1", "t1", "sr", []string{"order:create"}, []string{"m1"}, time.Hour, 24*time.Hour)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	engine, err := rbac.LoadEngineFromYAML([]byte(`
default_role: sr
roles:
  sr:
    permissions:
      - order:create
`))
	if err != nil {
		t.Fatalf("failed to load rbac: %v", err)
	}

	nextCalled := false
	handler := AuthGuard(secret)(RequirePermission(engine, "order:create")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest("POST", "/orders", nil)
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if !nextCalled {
		t.Errorf("expected next handler to be called")
	}
}
