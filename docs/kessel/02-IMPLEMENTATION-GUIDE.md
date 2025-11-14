# Kessel Integration Implementation Guide

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Code Structure](#code-structure)
3. [Implementation Steps](#implementation-steps)
4. [Service Permission Mapping](#service-permission-mapping)
5. [Testing Strategy](#testing-strategy)
6. [Deployment Checklist](#deployment-checklist)

## Architecture Overview

### Current RBAC Flow

```
Request → Identity Middleware → RBAC Middleware → Controller
                                       ↓
                                  RBAC Service (HTTP)
                                       ↓
                        Check: playbook-dispatcher:run:read
                        Filter: service = {remediations|config_manager|tasks}
```

### Target Kessel Flow

```
Request → Identity Middleware → Auth Selector → Kessel Middleware → Controller
                                       ↓                ↓
                           Feature Flag Check    RBAC Service (HTTP) → Get Workspace
                                                        ↓
                                                  Kessel Service (gRPC)
                                                        ↓
                                    Check: workspace permission + service mapping
```

### Parallel Operation Flow

During migration, both systems can run in parallel for validation:

```
Request → Identity Middleware → Auth Selector (AUTH_SYSTEM=both)
                                       ↓
                    ┌──────────────────┴──────────────────┐
                    ↓                                      ↓
            RBAC Middleware                       Kessel Middleware
           (ENFORCES - blocks)                    (LOG ONLY - validate)
                    ↓                                      ↓
           Allow/Deny Request                        Log Result
                    ↓                                      ↓
                    └──────────────────┬──────────────────┘
                                       ↓
                          RBAC decision is authoritative
                          (compare results in logs)
```

## Code Structure

### New Files to Create

```
internal/api/kessel/
├── client.go              # Kessel client interface and implementation
├── types.go               # Kessel-specific types
├── permissions.go         # Permission mapping logic
├── mock.go               # Mock client for testing
└── kessel_test.go        # Unit tests

internal/api/middleware/
├── kessel.go             # Kessel authorization middleware
├── authselector.go       # Feature flag-based auth system selector
└── kessel_test.go        # Middleware tests

internal/common/config/
└── config.go             # Add Kessel configuration fields (modify existing)

deploy/
└── clowdapp.yaml         # Add Kessel environment variables (modify existing)
```

### Modified Files

```
internal/api/main.go                    # Update middleware chain
internal/api/instrumentation/probes.go  # Add Kessel metrics
go.mod                                   # Add Kessel dependencies
```

## Implementation Steps

### Step 1: Add Dependencies

**File**: `go.mod`

```bash
go get github.com/project-kessel/inventory-api@latest
go get github.com/project-kessel/inventory-client-go@latest
go get google.golang.org/grpc@latest
```

### Step 2: Configuration

**File**: `internal/common/config/config.go`

Add configuration fields based on config-manager pattern:

```go
// Add to Config struct
type Config struct {
    // ... existing fields ...

    // Kessel Configuration
    KesselEnabled          bool   `mapstructure:"kessel_enabled"`
    KesselURL              string `mapstructure:"kessel_url"`
    KesselAuthEnabled      bool   `mapstructure:"kessel_auth_enabled"`
    KesselAuthClientID     string `mapstructure:"kessel_auth_client_id"`
    KesselAuthClientSecret string `mapstructure:"kessel_auth_client_secret"`
    KesselAuthOIDCIssuer   string `mapstructure:"kessel_auth_oidc_issuer"`
    KesselInsecure         bool   `mapstructure:"kessel_insecure"`
    KesselTimeout          int    `mapstructure:"kessel_timeout"`

    // Migration control
    AuthSystem             string `mapstructure:"auth_system"` // "rbac", "kessel", or "both"
}
```

Add to `init()` or `GetConfig()`:

```go
// Set defaults
options.SetDefault("kessel.enabled", false)
options.SetDefault("kessel.url", "localhost:9091")
options.SetDefault("kessel.auth.enabled", false)
options.SetDefault("kessel.auth.client_id", "")
options.SetDefault("kessel.auth.client_secret", "")
options.SetDefault("kessel.auth.oidc_issuer",
    "https://sso.redhat.com/auth/realms/redhat-external/protocol/openid-connect/token")
options.SetDefault("kessel.insecure", true)
options.SetDefault("kessel.timeout", 10)
options.SetDefault("auth.system", "rbac") // Default to RBAC for safety
```

### Step 3: Kessel Client Implementation

**File**: `internal/api/kessel/client.go`

```go
package kessel

import (
    "context"
    "fmt"

    kesselv2 "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
    "github.com/project-kessel/inventory-client-go/common"
    v1beta2 "github.com/project-kessel/inventory-client-go/v1beta2"
    "google.golang.org/grpc"

    "playbook-dispatcher/internal/common/config"
)

// KesselClient interface for authorization checks
type KesselClient interface {
    CheckWorkspacePermission(ctx context.Context, workspaceID, principalID, permission string) (bool, error)
    Close() error
}

type kesselClientImpl struct {
    client      *v1beta2.InventoryClient
    config      config.Config
    authEnabled bool
}

// NewKesselClient creates a new Kessel client
func NewKesselClient(cfg config.Config) (KesselClient, error) {
    options := []func(*common.Config){
        common.WithgRPCUrl(cfg.KesselURL),
        common.WithTLSInsecure(cfg.KesselInsecure),
    }

    if cfg.KesselAuthEnabled {
        options = append(options, common.WithAuthEnabled(
            cfg.KesselAuthClientID,
            cfg.KesselAuthClientSecret,
            cfg.KesselAuthOIDCIssuer,
        ))
    }

    kesselConfig := common.NewConfig(options...)
    client, err := v1beta2.New(kesselConfig)
    if err != nil {
        return nil, fmt.Errorf("failed to create Kessel client: %w", err)
    }

    return &kesselClientImpl{
        client:      client,
        config:      cfg,
        authEnabled: cfg.KesselAuthEnabled,
    }, nil
}

// CheckWorkspacePermission checks if a principal has permission on a workspace
func (k *kesselClientImpl) CheckWorkspacePermission(
    ctx context.Context,
    workspaceID string,
    principalID string,
    permission string,
) (bool, error) {
    object := &kesselv2.ResourceReference{
        ResourceType: "workspace",
        ResourceId:   workspaceID,
        Reporter: &kesselv2.ReporterReference{
            Type: "rbac",
        },
    }

    subject := &kesselv2.SubjectReference{
        Resource: &kesselv2.ResourceReference{
            ResourceType: "principal",
            ResourceId:   fmt.Sprintf("redhat/%s", principalID),
            Reporter: &kesselv2.ReporterReference{
                Type: "rbac",
            },
        },
    }

    var opts []grpc.CallOption
    if k.authEnabled {
        tokenOpts, err := k.client.GetTokenCallOption()
        if err != nil {
            return false, fmt.Errorf("failed to get auth token: %w", err)
        }
        opts = tokenOpts
    }

    request := &kesselv2.CheckRequest{
        Object:   object,
        Relation: permission,
        Subject:  subject,
    }

    response, err := k.client.KesselInventoryService.Check(ctx, request, opts...)
    if err != nil {
        return false, fmt.Errorf("Kessel check failed: %w", err)
    }

    return response.GetAllowed() == kesselv2.Allowed_ALLOWED_TRUE, nil
}

func (k *kesselClientImpl) Close() error {
    // Client cleanup if needed
    return nil
}
```

**File**: `internal/api/kessel/permissions.go`

```go
package kessel

import (
    "fmt"
)

// ServicePermissionMap maps service names to Kessel permissions
// Based on PR #699 schema
var ServicePermissionMap = map[string]string{
    "remediations":   "playbook_dispatcher_remediations_run_view",
    "tasks":          "playbook_dispatcher_tasks_run_view",
    "config_manager": "playbook_dispatcher_config_manager_run_view",
    "config-manager": "playbook_dispatcher_config_manager_run_view", // Support both formats
}

// GetKesselPermissionForService returns the Kessel permission for a service
func GetKesselPermissionForService(service string) (string, error) {
    perm, ok := ServicePermissionMap[service]
    if !ok {
        return "", fmt.Errorf("unknown service: %s", service)
    }
    return perm, nil
}

// GetServicesForPermissions returns list of services the user can access
// based on Kessel permission checks
func GetServicesForPermissions(
    allowedPermissions map[string]bool,
) []string {
    var services []string

    for service, permission := range ServicePermissionMap {
        if allowedPermissions[permission] {
            // Normalize to underscore format used in database
            normalizedService := service
            if service == "config-manager" {
                normalizedService = "config_manager"
            }
            services = append(services, normalizedService)
        }
    }

    return services
}
```

**File**: `internal/api/kessel/mock.go`

```go
package kessel

import (
    "context"
)

type mockKesselClient struct{}

// NewMockKesselClient creates a mock client for testing
func NewMockKesselClient() KesselClient {
    return &mockKesselClient{}
}

func (m *mockKesselClient) CheckWorkspacePermission(
    ctx context.Context,
    workspaceID string,
    principalID string,
    permission string,
) (bool, error) {
    // Mock: grant all permissions for testing
    return true, nil
}

func (m *mockKesselClient) Close() error {
    return nil
}
```

### Step 4: RBAC Client Helper

We need to get workspace IDs from RBAC service, similar to config-manager.

**File**: `internal/api/kessel/rbac.go`

```go
package kessel

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "time"

    "playbook-dispatcher/internal/common/config"
)

type RBACClient interface {
    GetDefaultWorkspaceID(ctx context.Context, orgID string) (string, error)
}

type rbacClient struct {
    baseURL string
    client  http.Client
    timeout time.Duration
}

func NewRBACClient(cfg config.Config) RBACClient {
    return &rbacClient{
        baseURL: cfg.GetString("rbac.host"),
        client:  http.Client{Timeout: time.Duration(cfg.KesselTimeout) * time.Second},
        timeout: time.Duration(cfg.KesselTimeout) * time.Second,
    }
}

type workspace struct {
    ID string `json:"id"`
}

type workspaceResponse struct {
    Data []workspace `json:"data"`
}

func (r *rbacClient) GetDefaultWorkspaceID(ctx context.Context, orgID string) (string, error) {
    url := fmt.Sprintf("%s/api/rbac/v2/workspaces/?type=default", r.baseURL)

    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return "", fmt.Errorf("error creating request: %w", err)
    }

    req.Header.Add("x-rh-rbac-org-id", orgID)

    resp, err := r.client.Do(req)
    if err != nil {
        return "", fmt.Errorf("error making request: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
    }

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return "", fmt.Errorf("error reading response: %w", err)
    }

    var response workspaceResponse
    if err := json.Unmarshal(body, &response); err != nil {
        return "", fmt.Errorf("error unmarshalling response: %w", err)
    }

    if len(response.Data) != 1 {
        return "", fmt.Errorf("unexpected number of default workspaces: %d", len(response.Data))
    }

    return response.Data[0].ID, nil
}
```

### Step 5: Kessel Middleware

**File**: `internal/api/middleware/kessel.go`

```go
package middleware

import (
    "context"
    "fmt"

    "github.com/labstack/echo/v4"
    "github.com/redhatinsights/platform-go-middlewares/identity"

    "playbook-dispatcher/internal/api/instrumentation"
    "playbook-dispatcher/internal/api/kessel"
    "playbook-dispatcher/internal/common/config"
)

type kesselAuthContext struct {
    WorkspaceID      string
    PrincipalID      string
    AllowedServices  []string
    AllowedPermissions map[string]bool
}

const kesselAuthContextKey = "kessel_auth"

// GetKesselAuthContext retrieves Kessel auth context from Echo context
func GetKesselAuthContext(ctx echo.Context) *kesselAuthContext {
    if val := ctx.Get(kesselAuthContextKey); val != nil {
        if authCtx, ok := val.(*kesselAuthContext); ok {
            return authCtx
        }
    }
    return nil
}

// EnforceKesselPermissions creates middleware that checks Kessel permissions
func EnforceKesselPermissions(cfg config.Config) echo.MiddlewareFunc {
    var kesselClient kessel.KesselClient
    var rbacClient kessel.RBACClient
    var err error

    // Initialize clients based on config
    if cfg.GetString("kessel.impl") == "impl" {
        kesselClient, err = kessel.NewKesselClient(cfg)
        if err != nil {
            panic(fmt.Errorf("failed to create Kessel client: %w", err))
        }
        rbacClient = kessel.NewRBACClient(cfg)
    } else {
        kesselClient = kessel.NewMockKesselClient()
        rbacClient = &mockRBACClient{}
    }

    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            // Skip if Kessel is disabled
            if !cfg.KesselEnabled {
                return next(c)
            }

            // Get identity from context (set by identity middleware)
            id := identity.GetIdentity(c.Request().Context())
            if id.Identity.OrgID == "" {
                instrumentation.KesselError(c, fmt.Errorf("missing org_id"))
                return echo.NewHTTPError(403, "Forbidden")
            }

            // Get default workspace ID
            workspaceID, err := rbacClient.GetDefaultWorkspaceID(
                c.Request().Context(),
                id.Identity.OrgID,
            )
            if err != nil {
                instrumentation.WorkspaceLookupError(err, id.Identity.OrgID)
                return echo.NewHTTPError(500, "Error performing authorization check")
            }

            instrumentation.WorkspaceLookupOK(id.Identity.OrgID, workspaceID)

            // Extract principal ID
            principalID, err := extractPrincipalID(id)
            if err != nil {
                instrumentation.KesselError(c, err)
                return echo.NewHTTPError(403, "Unsupported identity type")
            }

            // Check permissions for all known services
            allowedPermissions := make(map[string]bool)
            for service, permission := range kessel.ServicePermissionMap {
                allowed, err := kesselClient.CheckWorkspacePermission(
                    c.Request().Context(),
                    workspaceID,
                    principalID,
                    permission,
                )
                if err != nil {
                    instrumentation.KesselError(c, err)
                    return echo.NewHTTPError(500, "Error performing authorization check")
                }

                if allowed {
                    allowedPermissions[permission] = true
                    instrumentation.KesselPermissionGranted(c, principalID, service)
                } else {
                    instrumentation.KesselPermissionDenied(c, principalID, service)
                }
            }

            // Get list of allowed services
            allowedServices := kessel.GetServicesForPermissions(allowedPermissions)

            // Reject if no permissions granted
            if len(allowedServices) == 0 {
                instrumentation.KesselRejected(c)
                return echo.NewHTTPError(403, "Forbidden")
            }

            // Store auth context for use in controllers
            authCtx := &kesselAuthContext{
                WorkspaceID:        workspaceID,
                PrincipalID:        principalID,
                AllowedServices:    allowedServices,
                AllowedPermissions: allowedPermissions,
            }
            c.Set(kesselAuthContextKey, authCtx)

            return next(c)
        }
    }
}

func extractPrincipalID(id identity.XRHID) (string, error) {
    switch id.Identity.Type {
    case "User":
        return id.Identity.User.UserID, nil
    case "ServiceAccount":
        return id.Identity.ServiceAccount.UserId, nil
    default:
        return "", fmt.Errorf("unsupported identity type: %s", id.Identity.Type)
    }
}

type mockRBACClient struct{}

func (m *mockRBACClient) GetDefaultWorkspaceID(ctx context.Context, orgID string) (string, error) {
    return "mock-workspace-id", nil
}
```

### Step 6: Authorization Selector

**File**: `internal/api/middleware/authselector.go`

```go
package middleware

import (
    "github.com/labstack/echo/v4"

    "playbook-dispatcher/internal/common/config"
)

// SelectAuthorizationMiddleware returns the appropriate auth middleware based on config
func SelectAuthorizationMiddleware(cfg config.Config) echo.MiddlewareFunc {
    authSystem := cfg.GetString("auth.system")

    switch authSystem {
    case "kessel":
        return EnforceKesselPermissions(cfg)
    case "rbac":
        return EnforcePermissions(cfg, DispatcherPermission("run", "read"))
    case "both":
        // Run both for comparison, use RBAC for actual enforcement
        return chainMiddleware(
            EnforcePermissions(cfg, DispatcherPermission("run", "read")),
            // Kessel runs but doesn't block
            logOnlyKesselMiddleware(cfg),
        )
    default:
        // Default to RBAC for safety
        return EnforcePermissions(cfg, DispatcherPermission("run", "read"))
    }
}

// chainMiddleware combines multiple middlewares
func chainMiddleware(middlewares ...echo.MiddlewareFunc) echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        for i := len(middlewares) - 1; i >= 0; i-- {
            next = middlewares[i](next)
        }
        return next
    }
}

// logOnlyKesselMiddleware runs Kessel checks but only logs, doesn't enforce
func logOnlyKesselMiddleware(cfg config.Config) echo.MiddlewareFunc {
    kesselMiddleware := EnforceKesselPermissions(cfg)

    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            // Try Kessel check
            err := kesselMiddleware(func(c echo.Context) error {
                // Don't actually call next, just return nil
                return nil
            })(c)

            // Log result but don't enforce
            if err != nil {
                c.Logger().Warnf("Kessel would have rejected: %v", err)
            } else {
                c.Logger().Info("Kessel would have allowed")
            }

            // Always continue to next middleware
            return next(c)
        }
    }
}
```

### Step 7: Update Controllers

**File**: `internal/api/controllers/public/runsList.go`

Add Kessel support alongside existing RBAC filtering:

```go
// In the RunsList function, after existing RBAC filtering:

func RunsList(c echo.Context) error {
    // ... existing code ...

    // Check which auth system is in use
    authSystem := config.Get().GetString("auth.system")

    switch authSystem {
    case "kessel":
        // Use Kessel auth context
        if kesselAuth := middleware.GetKesselAuthContext(c); kesselAuth != nil {
            if len(kesselAuth.AllowedServices) > 0 {
                queryBuilder.Where("service IN ?", kesselAuth.AllowedServices)
            }
        }
    case "rbac", "both":
        // Use existing RBAC logic
        permissions := middleware.GetPermissions(c)
        if allowedServices := rbac.GetPredicateValues(permissions, "service"); len(allowedServices) > 0 {
            queryBuilder.Where("service IN ?", allowedServices)
        }
    }

    // ... rest of existing code ...
}
```

Apply same pattern to `runHostsList.go`.

### Step 8: Instrumentation

**File**: `internal/api/instrumentation/probes.go`

Add Kessel-specific metrics:

```go
var (
    // ... existing metrics ...

    // Kessel metrics
    kesselErrorTotal = promauto.NewCounter(prometheus.CounterOpts{
        Name: "api_kessel_error_total",
        Help: "The total number of errors from Kessel",
    })

    kesselRejectedTotal = promauto.NewCounter(prometheus.CounterOpts{
        Name: "api_kessel_rejected_total",
        Help: "The total number of requests rejected due to Kessel",
    })

    kesselWorkspaceLookupDuration = promauto.NewHistogram(prometheus.HistogramOpts{
        Name: "api_kessel_workspace_lookup_duration_seconds",
        Help: "Time to lookup workspace ID from RBAC",
    })

    kesselCheckDuration = promauto.NewHistogram(prometheus.HistogramOpts{
        Name: "api_kessel_check_duration_seconds",
        Help: "Time to perform Kessel permission check",
    })
)

func KesselError(ctx echo.Context, err error) {
    kesselErrorTotal.Inc()
    ctx.Logger().Errorf("Kessel error: %v", err)
}

func KesselRejected(ctx echo.Context) {
    kesselRejectedTotal.Inc()
    ctx.Logger().Warn("Kessel rejected request")
}

func WorkspaceLookupOK(orgID, workspaceID string) {
    // Log successful workspace lookup
}

func WorkspaceLookupError(err error, orgID string) {
    ctx.Logger().Errorf("Workspace lookup failed for org %s: %v", orgID, err)
}

func KesselPermissionGranted(ctx echo.Context, principalID, service string) {
    ctx.Logger().Debugf("Kessel granted %s permission to %s", service, principalID)
}

func KesselPermissionDenied(ctx echo.Context, principalID, service string) {
    ctx.Logger().Debugf("Kessel denied %s permission to %s", service, principalID)
}
```

### Step 9: Update Main

**File**: `internal/api/main.go`

Replace hard-coded RBAC middleware with auth selector:

```go
// Before:
// public.Use(middleware.EnforcePermissions(cfg, rbac.DispatcherPermission("run", "read")))

// After:
public.Use(middleware.SelectAuthorizationMiddleware(cfg))
```

### Step 10: Deployment Configuration

**File**: `deploy/clowdapp.yaml`

Add Kessel configuration:

```yaml
# In the deployment's env section:
- name: KESSEL_ENABLED
  value: ${KESSEL_ENABLED}
- name: KESSEL_URL
  value: ${KESSEL_URL}
- name: KESSEL_AUTH_ENABLED
  value: ${KESSEL_AUTH_ENABLED}
- name: KESSEL_AUTH_CLIENT_ID
  value: ${KESSEL_AUTH_CLIENT_ID}
- name: KESSEL_AUTH_CLIENT_SECRET
  valueFrom:
    secretKeyRef:
      name: kessel-auth
      key: client-secret
- name: KESSEL_AUTH_OIDC_ISSUER
  value: ${KESSEL_AUTH_OIDC_ISSUER}
- name: KESSEL_INSECURE
  value: ${KESSEL_INSECURE}
- name: KESSEL_TIMEOUT
  value: ${KESSEL_TIMEOUT}
- name: AUTH_SYSTEM
  value: ${AUTH_SYSTEM}

# In parameters section:
- name: KESSEL_ENABLED
  value: "false"
- name: KESSEL_URL
  required: false
- name: KESSEL_AUTH_ENABLED
  value: "false"
- name: KESSEL_AUTH_CLIENT_ID
  required: false
- name: KESSEL_AUTH_OIDC_ISSUER
  value: "https://sso.redhat.com/auth/realms/redhat-external/protocol/openid-connect/token"
- name: KESSEL_INSECURE
  value: "true"
- name: KESSEL_TIMEOUT
  value: "10"
- name: AUTH_SYSTEM
  value: "rbac"
```

## Service Permission Mapping

### Service to Permission Mapping

| Service | RBAC V1 Permission | Kessel V2 Permission |
|---------|-------------------|---------------------|
| remediations | `playbook-dispatcher:run:read` with `service=remediations` | `playbook_dispatcher_remediations_run_view` |
| tasks | `playbook-dispatcher:run:read` with `service=tasks` | `playbook_dispatcher_tasks_run_view` |
| config_manager | `playbook-dispatcher:run:read` with `service=config_manager` | `playbook_dispatcher_config_manager_run_view` |

### Migration Period Permissions

During migration, the V1-only permissions are maintained:

- `playbook_dispatcher_run_read` - Legacy read permission
- `playbook_dispatcher_run_write` - Legacy write permission

These will be removed after migration is complete.

## Testing Strategy

### Unit Tests

```bash
# Test Kessel client
go test ./internal/api/kessel/...

# Test middleware
go test ./internal/api/middleware/...

# Test permission mapping
go test ./internal/api/kessel/permissions_test.go
```

### Integration Tests

Create tests that verify:

1. RBAC mode works identically to before
2. Kessel mode correctly filters by service
3. Both mode runs both checks and compares

**File**: `internal/api/tests/kessel_integration_test.go`

### Manual Testing Checklist

- [ ] User with remediations permission can only see remediations runs
- [ ] User with tasks permission can only see tasks runs
- [ ] User with config_manager permission can only see config_manager runs
- [ ] User with no permissions gets 403
- [ ] Service account authentication works
- [ ] Metrics are recorded correctly
- [ ] Logs show auth decisions

## Deployment Checklist

### Pre-Deployment

- [ ] All tests passing
- [ ] Code review completed
- [ ] Security review completed
- [ ] Documentation updated
- [ ] Runbook created for rollback

### Stage Deployment

- [ ] Deploy with `AUTH_SYSTEM=rbac` (no change)
- [ ] Verify existing functionality works
- [ ] Switch to `AUTH_SYSTEM=both` for comparison
- [ ] Review logs for any discrepancies
- [ ] Switch to `AUTH_SYSTEM=kessel`
- [ ] Verify Kessel authorization works
- [ ] Monitor metrics for 24 hours
- [ ] Switch back to `rbac` if issues found

### Production Deployment

- [ ] Gradual rollout: 5% → 25% → 50% → 100%
- [ ] Monitor error rates at each stage
- [ ] Monitor latency at each stage
- [ ] Keep RBAC available for rollback
- [ ] Document any issues encountered

### Post-Deployment

- [ ] Monitor for 1 week with Kessel
- [ ] Confirm no increase in errors
- [ ] Confirm acceptable latency
- [ ] Remove RBAC code (after stability period)
- [ ] Update documentation
