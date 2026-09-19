package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/nasim0x1/pkg/jwt"
	"github.com/nasim0x1/pkg/logger"
	"github.com/nasim0x1/pkg/metrics"
	"github.com/nasim0x1/pkg/rbac"
	"github.com/nasim0x1/pkg/response"
	"github.com/google/uuid"
)

type contextKey string

const (
	ClaimsKey    contextKey = "jwt_claims"
	TenantIDKey  contextKey = "tenant_id"
	RequestIDKey contextKey = "request_id"
	UserIDKey    contextKey = "user_id"
	UserRoleKey  contextKey = "user_role"
)

func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		reqID := r.Header.Get("X-Request-ID")
		if reqID == "" {
			reqID = uuid.New().String()
		}

		ctx := context.WithValue(r.Context(), RequestIDKey, reqID)
		w.Header().Set("X-Request-ID", reqID)

		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(wrapped, r.WithContext(ctx))

		duration := time.Since(start)
		metrics.GetRegistry().RecordRequest(wrapped.statusCode)
		logger.Info("HTTP Request", map[string]interface{}{
			"method":      r.Method,
			"path":        r.URL.Path,
			"status":      wrapped.statusCode,
			"duration_ms": duration.Milliseconds(),
			"request_id":  reqID,
			"remote_addr": r.RemoteAddr,
		})
	})
}

func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				logger.Error("Panic recovered", map[string]interface{}{
					"error": rec,
					"path":  r.URL.Path,
				})
				response.InternalServerError(w, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Tenant-ID, X-Request-ID")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func TenantResolver(defaultTenant string) func(http.Handler) http.Handler {
	if defaultTenant == "" {
		defaultTenant = "default"
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantID := r.Header.Get("X-Tenant-ID")
			if tenantID == "" {
				tenantID = r.Header.Get("X-Tenant-Id")
			}
			if tenantID == "" {
				tenantID = r.Header.Get("Tenant-ID")
			}
			if tenantID == "" {
				tenantID = defaultTenant
			}
			ctx := context.WithValue(r.Context(), TenantIDKey, tenantID)
			w.Header().Set("X-Tenant-ID", tenantID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func AuthGuard(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				response.Unauthorized(w, "missing authorization header")
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				response.Unauthorized(w, "invalid authorization token format")
				return
			}

			claims, err := jwt.ValidateToken(jwtSecret, parts[1])
			if err != nil {
				response.Unauthorized(w, "invalid or expired token")
				return
			}

			if claims.TenantID == "" {
				claims.TenantID = "default"
			}

			ctx := context.WithValue(r.Context(), ClaimsKey, claims)
			ctx = context.WithValue(ctx, TenantIDKey, claims.TenantID)
			w.Header().Set("X-Tenant-ID", claims.TenantID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequirePermission(engine *rbac.Engine, permission string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := r.Context().Value(ClaimsKey).(*jwt.Claims)
			if !ok || claims == nil {
				response.Unauthorized(w, "unauthenticated request")
				return
			}

			if !engine.HasPermission(claims.Role, permission) {
				response.Forbidden(w, "insufficient permissions for resource")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func HeaderIdentityResolver(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if uid := r.Header.Get("X-User-ID"); uid != "" {
			ctx = context.WithValue(ctx, UserIDKey, uid)
		}
		if role := r.Header.Get("X-User-Role"); role != "" {
			ctx = context.WithValue(ctx, UserRoleKey, role)
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GetClaims(ctx context.Context) *jwt.Claims {
	if claims, ok := ctx.Value(ClaimsKey).(*jwt.Claims); ok {
		return claims
	}
	return nil
}

func GetTenantID(ctx context.Context) string {
	if tenantID, ok := ctx.Value(TenantIDKey).(string); ok && tenantID != "" {
		return tenantID
	}
	if claims := GetClaims(ctx); claims != nil && claims.TenantID != "" {
		return claims.TenantID
	}
	return "default"
}

func GetUserID(ctx context.Context) string {
	if claims := GetClaims(ctx); claims != nil && claims.UserID != "" {
		return claims.UserID
	}
	if uid, ok := ctx.Value(UserIDKey).(string); ok && uid != "" {
		return uid
	}
	return ""
}

func GetUserRole(ctx context.Context) string {
	if claims := GetClaims(ctx); claims != nil && claims.Role != "" {
		return claims.Role
	}
	if role, ok := ctx.Value(UserRoleKey).(string); ok && role != "" {
		return role
	}
	return ""
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
