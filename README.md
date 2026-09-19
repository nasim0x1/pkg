# Aarot Go Core Platform Kit (`pkg`)

A production-grade, zero-external-routing-framework Go toolkit built strictly with the Go standard library (`net/http`) for high-throughput, multi-tenant microservices.

---

## 📦 Modules Included

### 1. `pkg/dynamicschema`
A zero-dependency Dynamic Schema Engine that allows storing, validating, and sanitizing custom fields on any entity without schema migrations or third-party validation packages:
- **Field Types**: `string`, `number`, `boolean`, `date`, `enum`, `array`, `reference`.
- **Semantic Validators**: Hand-written format validators for `email`, `phone`, `url`, `currency`.
- **Validation Rules**: `required`, `min`, `max`, `min_length`, `max_length`, `pattern`, `options`.
- **Sanitization**: Strips undefined custom fields, injects defaults, and ensures JSON safety.
- **In-Memory Cache**: High-concurrency `sync.RWMutex` cache keyed by `entity_type:tenant_id`.
- **YAML Synchronization**: Syncs schema definitions from YAML files directly into PostgreSQL.

### 2. `pkg/rbac`
An enterprise Role-Based Access Control (RBAC) engine:
- **Generic Default Roles**: `super_admin`, `admin`, `manager`, `member`, `user` (default role: `user`).
- **Hierarchical Inheritance**: Roles inherit permissions from parent roles.
- **Wildcard Matching**: Supports `*` full super-admin access as well as scoped permissions (e.g. `order:create`, `user:manage_roles`).
- **Dual Engine & DB Persistence**: Atomically syncs catalog definitions into memory and PostgreSQL tables (`roles`, `permissions`, `role_permissions`).

### 3. `pkg/jwt`
Secure JWT issuance and token validation based on `golang-jwt/jwt/v5`:
- Issues Access and Refresh tokens with custom claims: `user_id`, `tenant_id`, `role`, `first_name`, `last_name`, `username`.
- Validates tokens, verifies HMAC-SHA256 signatures, and checks token expiration.

### 4. `pkg/cache`
Redis client integration using standard library conventions:
- **Session Revocation**: `RevokeToken(tokenID, ttl)` on logout.
- **User Blocklist**: `RevokeUser(userID, ttl)` on user suspension.
- **Multi-Device Invalidation**: `SetRevokedBefore(userID, timestamp, ttl)` when password changes on a new device, invalidating all older active sessions.

### 5. `pkg/database`
PostgreSQL connection pool management with `lib/pq`:
- Automatic connection configuration (`MaxOpenConns`, `MaxIdleConns`, `ConnMaxLifetime`).
- Context-aware Ping check on startup.
- Transaction helper `WithTx(ctx, func(tx *sql.Tx) error)` with automated rollback on panic/error and commit on success.

### 6. `pkg/middleware`
Standard `func(http.Handler) http.Handler` middleware chain:
- **`Logger`**: Structured JSON logging with request duration and HTTP status.
- **`Recoverer`**: Catches panics and returns clean 500 error envelopes.
- **`CORS`**: Standard CORS headers.
- **`TenantResolver`**: Extracts tenant context from `X-Tenant-ID` header or JWT claims.
- **`AuthGuard`**: Authenticates Bearer JWT and injects claims into `r.Context()`.
- **`RequirePermission`**: Enforces RBAC permissions via the RBAC Engine.

### 7. `pkg/response`
Standardized JSON API envelope response format:
```json
{
  "success": true,
  "message": "Operation completed",
  "data": { ... }
}
```
Provides type-safe helper functions: `Success`, `Created`, `JSON`, `BadRequest`, `Unauthorized`, `Forbidden`, `NotFound`, `InternalServerError`, `ServiceUnavailable`.

### 8. `pkg/swagger`
Zero-dependency embedded Swagger UI handler that serves standalone OpenAPI 3.0 specs:
- Accessible at `/api/v1/docs` or `/docs`.
- Conditionally enabled in `dev` mode.

### 9. `pkg/config` & `pkg/logger`
- **`config`**: Environment variable parsing (`GetEnv`, `GetEnvInt`, `GetEnvBool`) with defaults.
- **`logger`**: Structured JSON logging with service names and key-value attributes.

---

## 🧪 Running Unit Tests

```bash
cd pkg
go test -v ./...
```
