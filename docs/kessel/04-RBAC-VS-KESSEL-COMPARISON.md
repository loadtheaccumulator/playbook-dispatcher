# RBAC vs Kessel: Technical Comparison

## Executive Summary

This document provides a detailed technical comparison between the current RBAC implementation and the target Kessel authorization system for playbook-dispatcher.

## High-Level Comparison

| Aspect | RBAC | Kessel |
|--------|------|--------|
| **Protocol** | HTTP REST | gRPC |
| **Authorization Model** | Role-Based Access Control | Relationship-Based Access Control (ReBAC) |
| **Permission Granularity** | Attribute filters on permissions | Service-specific permissions |
| **Workspace Support** | Via attribute filtering | Native workspace concept |
| **Service Integration** | Direct HTTP calls | gRPC with client library |
| **Authentication** | Identity header passthrough | OIDC token-based |
| **Caching** | None (each request calls RBAC) | Possible with gRPC connection pooling |

## Authorization Models

### RBAC: Attribute-Based Filtering

**Permission Structure**:
```
playbook-dispatcher:run:read
```

**With Attribute Filter**:
```json
{
  "permission": "playbook-dispatcher:run:read",
  "resourceDefinitions": [
    {
      "attributeFilter": {
        "key": "service",
        "operation": "equal",
        "value": "remediations"
      }
    }
  ]
}
```

**Characteristics**:
- Single permission with filters
- Attribute filtering done at query time
- Filters can use `equal` or `in` operations
- Limited to service-level granularity

### Kessel: Service-Specific Permissions

**Permission Structure**:
```
playbook_dispatcher_remediations_run_view
playbook_dispatcher_tasks_run_view
playbook_dispatcher_config_manager_run_view
```

**Characteristics**:
- Distinct permission per service
- Workspace-based authorization
- Built on Zanzibar/SpiceDB relationship model
- More granular and explicit

## API Comparison

### RBAC API Call

**Endpoint**:
```
GET /api/rbac/v1/access/?application=playbook-dispatcher
```

**Headers**:
```
x-rh-identity: <base64-encoded-identity>
```

**Response**:
```json
{
  "data": [
    {
      "permission": "playbook-dispatcher:run:read",
      "resourceDefinitions": [
        {
          "attributeFilter": {
            "key": "service",
            "operation": "equal",
            "value": "remediations"
          }
        }
      ]
    },
    {
      "permission": "playbook-dispatcher:run:read",
      "resourceDefinitions": [
        {
          "attributeFilter": {
            "key": "service",
            "operation": "in",
            "value": ["tasks", "config_manager"]
          }
        }
      ]
    }
  ]
}
```

**Client Code**:
```go
resp, err := rbacClient.GetAccess(ctx, &GetAccessParams{
    Application: "playbook-dispatcher",
})

// Parse response to extract allowed services
allowedServices := rbac.GetPredicateValues(resp.Data, "service")
```

### Kessel API Call

**Step 1: Get Workspace ID** (via RBAC v2 API)
```
GET /api/rbac/v2/workspaces/?type=default
```

**Headers**:
```
x-rh-rbac-org-id: <org-id>
authorization: Bearer <oidc-token>  # If auth enabled
```

**Response**:
```json
{
  "data": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000"
    }
  ]
}
```

**Step 2: Check Permission** (via Kessel gRPC)
```protobuf
message CheckRequest {
  ResourceReference object = {
    resource_type: "workspace"
    resource_id: "550e8400-e29b-41d4-a716-446655440000"
    reporter: {
      type: "rbac"
    }
  }
  string relation = "playbook_dispatcher_remediations_run_view"
  SubjectReference subject = {
    resource: {
      resource_type: "principal"
      resource_id: "redhat/user-12345"
      reporter: {
        type: "rbac"
      }
    }
  }
}
```

**Response**:
```protobuf
message CheckResponse {
  Allowed allowed = ALLOWED_TRUE  // or ALLOWED_FALSE
}
```

**Client Code**:
```go
// Step 1: Get workspace
workspaceID, err := rbacClient.GetDefaultWorkspaceID(ctx, orgID)

// Step 2: Check each service permission
for service, permission := range servicePermissionMap {
    allowed, err := kesselClient.CheckWorkspacePermission(
        ctx,
        workspaceID,
        principalID,
        permission,
    )
    if allowed {
        allowedServices = append(allowedServices, service)
    }
}
```

## Performance Comparison

### RBAC

**Latency Breakdown**:
1. HTTP request setup: ~1ms
2. Network round-trip: ~5-20ms (internal cluster)
3. RBAC service processing: ~10-30ms
4. JSON parsing: ~1-2ms
5. **Total**: ~20-50ms per request

**Advantages**:
- Single HTTP call returns all permissions
- Simple JSON parsing
- Well-established infrastructure

**Disadvantages**:
- No connection pooling (new HTTP conn per request)
- Large JSON response even for small permission set
- No caching strategy

### Kessel

**Latency Breakdown**:
1. Workspace lookup (RBAC v2): ~20-50ms (cached after first call)
2. gRPC connection (pooled): ~1ms
3. Per-service check: ~5-15ms
4. 3 services × 5-15ms: ~15-45ms
5. **Total**: ~35-95ms first request, ~15-45ms subsequent

**Advantages**:
- gRPC connection pooling reduces overhead
- Binary protocol more efficient than JSON
- Workspace lookup can be cached
- Parallel permission checks possible

**Disadvantages**:
- Requires multiple checks (one per service)
- Initial workspace lookup overhead
- More complex client setup

### Optimization Strategies

#### RBAC
- ✅ Already optimized (single call)
- ⚠️ Limited caching options

#### Kessel
- ✅ Cache workspace IDs per org (30-60 minute TTL)
- ✅ gRPC connection pooling (built-in)
- ✅ Parallel permission checks (check all services concurrently)
- ✅ Request-level caching (store auth context in request)

**Optimized Kessel Flow**:
```go
// Cache workspace lookup
workspaceID := cache.Get("workspace:" + orgID)
if workspaceID == "" {
    workspaceID, _ = rbacClient.GetDefaultWorkspaceID(ctx, orgID)
    cache.Set("workspace:" + orgID, workspaceID, 30*time.Minute)
}

// Check all services in parallel
var wg sync.WaitGroup
results := make(chan PermissionResult, len(services))

for service, permission := range servicePermissionMap {
    wg.Add(1)
    go func(s, p string) {
        defer wg.Done()
        allowed, _ := kesselClient.CheckWorkspacePermission(ctx, workspaceID, principalID, p)
        results <- PermissionResult{service: s, allowed: allowed}
    }(service, permission)
}

wg.Wait()
close(results)
```

## Error Handling

### RBAC Error Scenarios

| Error | HTTP Status | Handling |
|-------|-------------|----------|
| RBAC service down | 503 Service Unavailable | Return 503 to client |
| Invalid identity | 403 Forbidden | Return 403 to client |
| Network timeout | 500 Internal Server Error | Return 500 to client |
| Permission denied | 200 OK (empty access list) | Return 403 to client |

**Code**:
```go
access, err := rbacClient.GetAccess(ctx, params)
if err != nil {
    if errors.Is(err, ErrServiceUnavailable) {
        return echo.NewHTTPError(503, "Authorization service unavailable")
    }
    return echo.NewHTTPError(500, "Error performing authorization check")
}

if len(access) == 0 {
    return echo.NewHTTPError(403, "Forbidden")
}
```

### Kessel Error Scenarios

| Error | Handling |
|-------|----------|
| Kessel service down | Return 500, log error, (optional: fallback to RBAC) |
| Workspace lookup fails | Return 500, log error |
| gRPC connection error | Return 500, retry with backoff |
| Authentication failure | Return 500, check credentials |
| Permission denied | Return 403 to client |

**Code**:
```go
workspaceID, err := rbacClient.GetDefaultWorkspaceID(ctx, orgID)
if err != nil {
    instrumentation.WorkspaceLookupError(err, orgID)
    return echo.NewHTTPError(500, "Error performing authorization check")
}

allowed, err := kesselClient.CheckWorkspacePermission(ctx, workspaceID, principalID, permission)
if err != nil {
    instrumentation.KesselError(ctx, err)

    // Optional: fallback to RBAC if configured
    if cfg.AuthFallbackEnabled {
        return fallbackToRBAC(ctx)
    }

    return echo.NewHTTPError(500, "Error performing authorization check")
}

if !allowed {
    return echo.NewHTTPError(403, "Forbidden")
}
```

## Permission Mapping

### Service Name Normalization

Both RBAC and Kessel need to handle service name variations:

| Database Value | RBAC Filter | Kessel Permission |
|----------------|-------------|-------------------|
| `remediations` | `service = "remediations"` | `playbook_dispatcher_remediations_run_view` |
| `tasks` | `service = "tasks"` | `playbook_dispatcher_tasks_run_view` |
| `config_manager` | `service = "config_manager"` OR `service = "config-manager"` | `playbook_dispatcher_config_manager_run_view` |

**Normalization Logic**:
```go
func normalizeServiceName(service string) string {
    // Always use underscore format for database
    return strings.ReplaceAll(service, "-", "_")
}

func getKesselPermission(service string) string {
    normalized := normalizeServiceName(service)
    return fmt.Sprintf("playbook_dispatcher_%s_run_view", normalized)
}
```

### Permission Equivalence

| RBAC Permission | RBAC Filter | Kessel Permission | Purpose |
|-----------------|-------------|-------------------|---------|
| `playbook-dispatcher:run:read` | `service=remediations` | `playbook_dispatcher_remediations_run_view` | View remediation runs |
| `playbook-dispatcher:run:read` | `service=tasks` | `playbook_dispatcher_tasks_run_view` | View task runs |
| `playbook-dispatcher:run:read` | `service=config_manager` | `playbook_dispatcher_config_manager_run_view` | View config-manager runs |
| `playbook-dispatcher:run:write` | N/A | `playbook_dispatcher_run_write` (V1-only) | Create runs (internal API) |

## Code Impact Analysis

### Files Modified

| File | Change Type | Complexity | Risk |
|------|-------------|------------|------|
| `internal/common/config/config.go` | Modify | Low | Low |
| `internal/api/main.go` | Modify | Medium | Medium |
| `internal/api/controllers/public/runsList.go` | Modify | Low | Low |
| `internal/api/controllers/public/runHostsList.go` | Modify | Low | Low |
| `internal/api/instrumentation/probes.go` | Modify | Low | Low |
| `deploy/clowdapp.yaml` | Modify | Low | Low |

### Files Created

| File | Purpose | Complexity | Lines of Code (est.) |
|------|---------|------------|----------------------|
| `internal/api/kessel/client.go` | Kessel client | Medium | ~150 |
| `internal/api/kessel/permissions.go` | Permission mapping | Low | ~50 |
| `internal/api/kessel/rbac.go` | Workspace lookup | Low | ~100 |
| `internal/api/kessel/mock.go` | Mock client | Low | ~30 |
| `internal/api/kessel/types.go` | Type definitions | Low | ~30 |
| `internal/api/middleware/kessel.go` | Kessel middleware | High | ~200 |
| `internal/api/middleware/authselector.go` | Auth system selector | Medium | ~100 |
| `internal/api/kessel/kessel_test.go` | Unit tests | Medium | ~200 |
| `internal/api/middleware/kessel_test.go` | Middleware tests | High | ~300 |

**Total New Code**: ~1,160 lines

### Dependency Changes

**New Dependencies**:
```
github.com/project-kessel/inventory-api v0.x.x
github.com/project-kessel/inventory-client-go v0.x.x
google.golang.org/grpc v1.x.x
```

**Impact**:
- Binary size increase: ~5-10 MB (gRPC and protobuf)
- No breaking changes to existing dependencies
- All new dependencies are stable

## Migration Impact

### Database Schema
- **No changes required** ✅
- Service column continues to use same values
- Org ID isolation unchanged

### API Contracts
- **No changes required** ✅
- Same endpoints
- Same request/response formats
- Same HTTP status codes

### Client Libraries
- **No changes required** ✅
- Authorization is server-side only
- API clients unaffected

### Monitoring and Alerting
- **New metrics added**:
  - `api_kessel_error_total`
  - `api_kessel_rejected_total`
  - `api_kessel_check_duration_seconds`
  - `api_kessel_workspace_lookup_duration_seconds`

- **Existing metrics continue**:
  - `api_rbac_error_total`
  - `api_rbac_rejected_total`

### Logging
- **New log patterns**:
  - `Kessel granted <service> permission to <principal>`
  - `Kessel denied <service> permission to <principal>`
  - `Workspace lookup failed for org <org-id>`
  - `Kessel check failed: <error>`

## Testing Strategy Comparison

### RBAC Testing

**Unit Tests**:
```go
func TestRBACPermissionParsing(t *testing.T) {
    response := `{"data": [{"permission": "playbook-dispatcher:run:read", ...}]}`
    access := parseRBACResponse(response)
    services := getPredicateValues(access, "service")
    assert.Contains(t, services, "remediations")
}
```

**Integration Tests**:
```go
func TestRBACMiddleware(t *testing.T) {
    mockRBAC := httptest.NewServer(...)
    req := httptest.NewRequest("GET", "/runs", nil)
    req.Header.Set("x-rh-identity", encodeIdentity(...))

    middleware.EnforcePermissions(...)(handler)(req)

    assert.Equal(t, 200, resp.StatusCode)
}
```

### Kessel Testing

**Unit Tests**:
```go
func TestKesselPermissionMapping(t *testing.T) {
    perm, err := GetKesselPermissionForService("remediations")
    assert.NoError(t, err)
    assert.Equal(t, "playbook_dispatcher_remediations_run_view", perm)
}

func TestKesselClient(t *testing.T) {
    client := NewMockKesselClient()
    allowed, err := client.CheckWorkspacePermission(ctx, "ws-123", "user-456", "permission")
    assert.NoError(t, err)
    assert.True(t, allowed)
}
```

**Integration Tests**:
```go
func TestKesselMiddleware(t *testing.T) {
    mockKessel := grpc.NewServer(...)
    mockRBAC := httptest.NewServer(...)

    req := httptest.NewRequest("GET", "/runs", nil)
    req.Header.Set("x-rh-identity", encodeIdentity(...))

    middleware.EnforceKesselPermissions(...)(handler)(req)

    assert.Equal(t, 200, resp.StatusCode)
}
```

**Comparison Tests**:
```go
func TestRBACKesselEquivalence(t *testing.T) {
    // Same identity, same expected result
    identity := encodeIdentity(...)

    // Test with RBAC
    rbacResult := testWithRBAC(identity)

    // Test with Kessel
    kesselResult := testWithKessel(identity)

    // Should produce same result
    assert.Equal(t, rbacResult.AllowedServices, kesselResult.AllowedServices)
}
```

## Rollback Comparison

### RBAC Rollback
Not applicable - already current state.

### Kessel Rollback

**Ease of Rollback**: ⭐⭐⭐⭐⭐ (Very Easy)

**Method 1**: Environment variable change
```bash
kubectl set env deployment/playbook-dispatcher AUTH_SYSTEM=rbac
# Pods restart with RBAC authorization
# Rollback time: ~2 minutes
```

**Method 2**: Deployment rollback
```bash
kubectl rollout undo deployment/playbook-dispatcher
# Reverts to previous deployment
# Rollback time: ~3 minutes
```

**Method 3**: Feature flag (if implemented)
```bash
# Update ConfigMap or external feature flag service
# Rollback time: ~immediate (if using external service)
```

## Recommendation

### Short Term (Weeks 1-4)
- Implement Kessel alongside RBAC
- Use `AUTH_SYSTEM=both` mode for validation
- Monitor metrics for performance and correctness
- Keep RBAC as safety net

### Medium Term (Weeks 5-8)
- Switch to `AUTH_SYSTEM=kessel` in stage
- Gradual rollout in production (5% → 100%)
- Monitor error rates and latency
- Document any edge cases

### Long Term (Week 9+)
- Remove RBAC permission check code
- Keep RBAC client for workspace lookup
- Update documentation
- Share learnings with other teams

## Conclusion

Both RBAC and Kessel can effectively authorize playbook-dispatcher requests. Kessel offers:

**Advantages**:
- ✅ More explicit permission model
- ✅ Better workspace integration
- ✅ Modern gRPC protocol
- ✅ Future-proof architecture

**Considerations**:
- ⚠️ Slightly higher initial latency (mitigated by caching)
- ⚠️ Additional complexity during migration
- ⚠️ New monitoring and debugging needed

The migration is **low risk** due to:
- Feature flag-based toggle
- Parallel operation mode
- Easy rollback mechanism
- No API contract changes
- Comprehensive testing strategy
