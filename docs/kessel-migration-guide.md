# Kessel RBAC Migration Guide

This guide explains how to migrate playbook-dispatcher from the legacy RBAC service to Kessel for authorization.

## Overview

Kessel is Red Hat's next-generation authorization service based on Google Zanzibar. It provides:
- **Fine-grained authorization**: Resource-level permissions instead of just role-based
- **Relationship-based access control**: Define permissions through relationships
- **Better performance**: gRPC-based with efficient caching
- **Scalability**: Designed for high-throughput authorization checks

## Architecture Changes

### Current RBAC Flow
1. Request arrives with identity header
2. Middleware calls RBAC service REST API to fetch all permissions
3. Permissions are filtered based on required permission strings
4. Request proceeds if matching permission found

### New Kessel Flow
1. Request arrives with identity header
2. Middleware extracts subject (user) and resource from request
3. Middleware calls Kessel gRPC API to check specific permission
4. Request proceeds if check returns allowed

## Key Differences

| Aspect | Legacy RBAC | Kessel |
|--------|-------------|--------|
| Protocol | HTTP/REST | gRPC |
| Permission Model | String-based (`app:resource:verb`) | Tuple-based (`subject, relation, resource`) |
| Check Type | Fetch all + filter | Direct check |
| Resource Granularity | Type-level | Instance-level |
| Response | List of permissions | Boolean allowed/denied |

## Configuration

### Step 1: Add Kessel Dependencies

Add to `go.mod`:
```go
github.com/project-kessel/relations-api v0.x.x
google.golang.org/grpc v1.x.x
```

Run:
```bash
go get github.com/project-kessel/relations-api
go mod tidy
```

### Step 2: Update Configuration

In your deployment configuration, add Kessel settings:

```yaml
# ClowdApp configuration
dependencies:
  - kessel-relations

# Environment variables
KESSEL_ENABLED=true
KESSEL_IMPL=impl
KESSEL_HOSTNAME=kessel-relations
KESSEL_PORT=9000
KESSEL_INSECURE=false
KESSEL_TIMEOUT=10
```

For local development:
```bash
export KESSEL_ENABLED=true
export KESSEL_IMPL=mock  # Use mock for local testing
```

### Step 3: Initialize Kessel Client

In your application initialization (e.g., `internal/api/main.go`), initialize the Kessel client:

```go
import "playbook-dispatcher/internal/api/kessel"

// Add to config initialization
kessel.ConfigureDefaults(cfg)

// Create Kessel client
kesselClient, err := kessel.NewKesselClientFromConfig(cfg)
if err != nil {
    log.Fatal("Failed to create Kessel client", err)
}
defer kesselClient.Close()
```

## Migrating Middleware

### Legacy RBAC Middleware

```go
// Old approach
middleware.EnforcePermissions(cfg,
    rbac.DispatcherPermission("run", "read"))
```

### Kessel Middleware

```go
// New approach - org level permissions (list all runs)
middleware.EnforceKesselOrgPermissions(cfg,
    kessel.RelationRead,
    kessel.ResourceTypeRun)
```

Or for resource-specific checks:

```go
// Resource-specific permissions (get specific run)
middleware.EnforceKesselPermissions(cfg,
    kessel.RelationRead,
    func(c echo.Context) (kessel.Resource, error) {
        runID := c.Param("run_id")
        identity := identityMiddleware.GetIdentity(c.Request().Context())

        return kessel.Resource{
            Type:   kessel.ResourceTypeRun,
            ID:     runID,
            Tenant: identity.Identity.OrgID,
        }, nil
    })
```

## Controller Examples

### Example 1: List Runs (Org-Level Permission)

```go
// In your route registration
e.GET("/api/playbook-dispatcher/v1/runs",
    controllers.ApiRunsList,
    middleware.EnforceKesselOrgPermissions(cfg, kessel.RelationRead, kessel.ResourceTypeRun))
```

### Example 2: Get Specific Run (Resource-Level Permission)

```go
// Resource extractor for run_id parameter
runResourceExtractor := func(c echo.Context) (kessel.Resource, error) {
    runID := c.Param("run_id")
    identity := identityMiddleware.GetIdentity(c.Request().Context())

    return kessel.Resource{
        Type:   kessel.ResourceTypeRun,
        ID:     runID,
        Tenant: identity.Identity.OrgID,
    }, nil
}

e.GET("/api/playbook-dispatcher/v1/runs/:run_id",
    controllers.ApiRunsGet,
    middleware.EnforceKesselPermissions(cfg, kessel.RelationRead, runResourceExtractor))
```

### Example 3: Cancel Run (Write Permission)

```go
e.POST("/api/playbook-dispatcher/v1/runs/:run_id/cancel",
    controllers.ApiRunsCancel,
    middleware.EnforceKesselPermissions(cfg, kessel.RelationCancel, runResourceExtractor))
```

### Example 4: In-Controller Checks

For more complex scenarios, perform checks within the controller:

```go
func (this *controllers) ApiRunsList(ctx echo.Context, params ApiRunsListParams) error {
    subject, _ := kessel.SubjectFromContext(ctx.Request().Context())

    // Get runs from database
    runs := fetchRunsFromDB()

    // Filter by authorization
    runIDs := make([]string, len(runs))
    for i, run := range runs {
        runIDs[i] = run.ID.String()
    }

    authorizedIDs, err := kessel.FilterAuthorizedResources(
        ctx.Request().Context(),
        kesselClient,
        subject,
        kessel.RelationRead,
        kessel.ResourceTypeRun,
        runIDs,
    )

    // Filter runs to only authorized ones
    authorizedRuns := filterRuns(runs, authorizedIDs)

    return ctx.JSON(http.StatusOK, authorizedRuns)
}
```

## Resource Type Definitions in Kessel

You need to define your resource types in Kessel's schema. This is typically done via Kessel's management API or configuration:

```yaml
resourceTypes:
  - name: playbook-dispatcher/run
    relations:
      - name: read
        description: Can view run details
      - name: write
        description: Can modify run
      - name: cancel
        description: Can cancel run
      - name: execute
        description: Can execute/create run

  - name: playbook-dispatcher/run_host
    relations:
      - name: read
        description: Can view run host details
      - name: write
        description: Can modify run host

  - name: playbook-dispatcher/org
    relations:
      - name: admin
        description: Full access to org resources
      - name: viewer
        description: Read-only access to org resources
```

## Migration Strategy

### Phase 1: Parallel Running (Recommended)
1. Deploy code with both RBAC and Kessel support
2. Set `KESSEL_ENABLED=false` initially
3. Monitor RBAC metrics

### Phase 2: Gradual Rollout
1. Enable Kessel in staging environment
2. Run both systems in parallel, log discrepancies
3. Validate Kessel behavior matches RBAC

### Phase 3: Full Migration
1. Set `KESSEL_ENABLED=true` in production
2. Use Kessel for authorization decisions
3. Keep RBAC as fallback initially

### Phase 4: Cleanup
1. Remove RBAC client code
2. Remove RBAC middleware
3. Update documentation

## Testing

### Unit Tests

```go
func TestKesselAuthorization(t *testing.T) {
    client := kessel.NewMockKesselClient(true) // Allow all

    subject := kessel.Subject{
        Type:   kessel.SubjectTypeUser,
        ID:     "test-user",
        Tenant: "test-org",
    }

    check := kessel.DispatcherRunCheck(subject, kessel.RelationRead, "run-123")
    allowed, err := client.Check(context.Background(), check)

    assert.NoError(t, err)
    assert.True(t, allowed)
}
```

### Integration Tests

```go
func TestKesselMiddleware(t *testing.T) {
    e := echo.New()
    cfg := config.Get()
    cfg.Set("kessel.impl", "mock")

    e.GET("/runs/:run_id",
        handler,
        middleware.EnforceKesselPermissions(cfg, kessel.RelationRead, runExtractor))

    // Test with valid identity
    req := httptest.NewRequest(http.MethodGet, "/runs/123", nil)
    req.Header.Set("x-rh-identity", encodedIdentity)
    rec := httptest.NewRecorder()

    e.ServeHTTP(rec, req)
    assert.Equal(t, http.StatusOK, rec.Code)
}
```

## Troubleshooting

### Issue: "Failed to connect to Kessel"
- Verify Kessel service is running and accessible
- Check `KESSEL_HOSTNAME` and `KESSEL_PORT` configuration
- Verify network policies allow gRPC traffic

### Issue: "Unauthorized" errors
- Verify identity header is present
- Check Kessel has proper relationship tuples defined
- Use Kessel's admin API to inspect relationships

### Issue: Performance degradation
- Enable Kessel client connection pooling
- Use batch checks where possible
- Monitor Kessel service metrics

## Performance Considerations

1. **Connection Pooling**: gRPC connections are reused automatically
2. **Batch Checks**: Use `CheckBatch()` for multiple resources
3. **Caching**: Consider caching check results for frequently accessed resources
4. **Async Checks**: For non-critical paths, consider async authorization

## Security Considerations

1. **Always validate identity**: Ensure identity middleware runs before Kessel checks
2. **Audit logging**: Log authorization decisions for compliance
3. **Fail closed**: If Kessel is unavailable, deny access (already implemented)
4. **Resource ownership**: Validate resource belongs to tenant before check

## References

- [Kessel Documentation](https://github.com/project-kessel/relations-api)
- [Google Zanzibar Paper](https://research.google/pubs/pub48190/)
- [playbook-dispatcher RBAC Implementation](./internal/api/rbac/)
