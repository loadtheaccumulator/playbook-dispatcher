# Kessel Quick Start Guide

This guide provides a quick reference for implementing Kessel authorization in playbook-dispatcher.

## File Structure

```
internal/api/
├── kessel/
│   ├── types.go          # Core types and interfaces
│   ├── client.go         # Kessel gRPC client implementation
│   ├── mock.go           # Mock client for testing
│   ├── config.go         # Configuration helpers
│   └── utils.go          # Helper functions
├── middleware/
│   └── kessel.go         # Kessel authorization middleware
└── controllers/
    └── public/
        ├── runsList_kessel_example.go  # Example controller implementation
        └── main_kessel_example.go      # Example route setup
```

## Quick Implementation Steps

### 1. Add Dependencies

```bash
cd ~/dev/git/RedHatInsights/playbook-dispatcher
go get github.com/project-kessel/relations-api
go mod tidy
```

### 2. Update Configuration File

Add to your environment or config:

```bash
# For development (uses mock)
export KESSEL_ENABLED=true
export KESSEL_IMPL=mock

# For production
export KESSEL_ENABLED=true
export KESSEL_IMPL=impl
export KESSEL_HOSTNAME=kessel-relations
export KESSEL_PORT=9000
```

### 3. Update Main Application Code

```go
import "playbook-dispatcher/internal/api/kessel"

// In your main() or initialization function:
kessel.ConfigureDefaults(cfg)
```

### 4. Update Route Registration

Replace RBAC middleware with Kessel middleware:

**Before (RBAC):**
```go
api.GET("/runs",
    controllers.ApiRunsList,
    middleware.EnforcePermissions(cfg, rbac.DispatcherPermission("run", "read")))
```

**After (Kessel):**
```go
api.GET("/runs",
    controllers.ApiRunsList,
    middleware.EnforceKesselOrgPermissions(cfg, kessel.RelationRead, kessel.ResourceTypeRun))
```

### 5. For Resource-Specific Checks

```go
runExtractor := func(c echo.Context) (kessel.Resource, error) {
    runID := c.Param("run_id")
    identity := identityMiddleware.GetIdentity(c.Request().Context())
    return kessel.Resource{
        Type:   kessel.ResourceTypeRun,
        ID:     runID,
        Tenant: identity.Identity.OrgID,
    }, nil
}

api.GET("/runs/:run_id",
    controllers.ApiRunsGet,
    middleware.EnforceKesselPermissions(cfg, kessel.RelationRead, runExtractor))
```

## Common Patterns

### Pattern 1: List Endpoint (Org-Level Permission)

```go
// Route
api.GET("/runs", controllers.ApiRunsList,
    middleware.EnforceKesselOrgPermissions(cfg, kessel.RelationRead, kessel.ResourceTypeRun))

// Controller - middleware already checked permission
func (c *controllers) ApiRunsList(ctx echo.Context) error {
    // Fetch and return runs
    // No additional permission check needed
}
```

### Pattern 2: Get/Update/Delete (Resource-Level Permission)

```go
// Route
api.GET("/runs/:run_id", controllers.ApiRunsGet,
    middleware.EnforceKesselPermissions(cfg, kessel.RelationRead, runExtractor))

// Controller - middleware already checked permission for this specific run
func (c *controllers) ApiRunsGet(ctx echo.Context, runID string) error {
    // Fetch and return the specific run
    // No additional permission check needed
}
```

### Pattern 3: In-Controller Fine-Grained Filtering

```go
// Route - org-level permission
api.GET("/runs", controllers.ApiRunsList,
    middleware.EnforceKesselOrgPermissions(cfg, kessel.RelationRead, kessel.ResourceTypeRun))

// Controller - filter results by resource-level permissions
func (c *controllers) ApiRunsList(ctx echo.Context) error {
    subject, _ := kessel.SubjectFromContext(ctx.Request().Context())

    runs := fetchAllRunsFromDB()
    runIDs := extractIDs(runs)

    authorizedIDs, _ := kessel.FilterAuthorizedResources(
        ctx.Request().Context(),
        kesselClient,
        subject,
        kessel.RelationRead,
        kessel.ResourceTypeRun,
        runIDs,
    )

    return ctx.JSON(http.StatusOK, filterRuns(runs, authorizedIDs))
}
```

## Permission Mapping

| Legacy RBAC | Kessel Equivalent |
|-------------|-------------------|
| `playbook-dispatcher:run:read` | Subject can `read` resource `playbook-dispatcher/run` |
| `playbook-dispatcher:run:write` | Subject can `write` resource `playbook-dispatcher/run` |
| `playbook-dispatcher:run:execute` | Subject can `execute` resource `playbook-dispatcher/run` |

## Testing

### Unit Test with Mock Client

```go
import "playbook-dispatcher/internal/api/kessel"

func TestWithMockKessel(t *testing.T) {
    client := kessel.NewMockKesselClient(true) // Allow all

    subject := kessel.Subject{
        Type:   kessel.SubjectTypeUser,
        ID:     "test-user",
        Tenant: "test-org",
    }

    allowed, err := kessel.CheckRunAccess(
        context.Background(),
        client,
        subject,
        "run-123",
        kessel.RelationRead,
    )

    assert.NoError(t, err)
    assert.True(t, allowed)
}
```

### Integration Test

```go
func TestKesselMiddlewareIntegration(t *testing.T) {
    cfg := viper.New()
    cfg.Set("kessel.impl", "mock")

    e := echo.New()

    e.GET("/runs/:run_id",
        handler,
        middleware.EnforceKesselPermissions(cfg, kessel.RelationRead, runExtractor))

    req := httptest.NewRequest(http.MethodGet, "/runs/123", nil)
    req.Header.Set("x-rh-identity", createTestIdentity())
    rec := httptest.NewRecorder()

    e.ServeHTTP(rec, req)
    assert.Equal(t, http.StatusOK, rec.Code)
}
```

## Available Relations

- `kessel.RelationRead` - View/read access
- `kessel.RelationWrite` - Create/update access
- `kessel.RelationExecute` - Execute/run access
- `kessel.RelationDelete` - Delete access
- `kessel.RelationCancel` - Cancel operation access

## Available Resource Types

- `kessel.ResourceTypeRun` - playbook-dispatcher/run
- `kessel.ResourceTypeRunHost` - playbook-dispatcher/run_host
- `kessel.ResourceTypeOrg` - playbook-dispatcher/org

## Troubleshooting

**Mock not working?**
```bash
export KESSEL_IMPL=mock
```

**Can't connect to Kessel?**
Check `KESSEL_HOSTNAME` and `KESSEL_PORT` match your deployment.

**Getting 403 Forbidden?**
- Verify identity header is present
- Check Kessel has the proper relationship tuples defined
- Use mock client to verify middleware is working

## Next Steps

1. See `docs/kessel-migration-guide.md` for detailed migration strategy
2. Review example files in `internal/api/controllers/public/runsList_kessel_example.go`
3. Check Kessel schema definition requirements
4. Plan gradual rollout strategy
