# PR #699 Plan: Implementation Plan

**Date**: 2025-11-13
**Approach**: Centralized Kessel schema in playbook-dispatcher namespace, RBAC role changes required
**Timeline**: 9-13 weeks
**Breaking Changes**: Yes (RBAC role format)

---

## Overview

PR #699 implements Kessel authorization by:
1. Creating centralized Kessel schema in `playbook-dispatcher.ksl`
2. Updating RBAC roles to new format (removes attribute filters)
3. Refactoring RBAC code to parse new permission format
4. Using feature flags to toggle between RBAC and Kessel
5. Requires RBAC code changes before role updates (critical timing)

---

## Phase 1: RBAC Code Refactoring (Week 1-3)

### Objective
Update playbook-dispatcher RBAC code to support both old and new role formats.

### Critical Requirement
**RBAC code MUST be deployed to production BEFORE any role updates**, otherwise existing code will break and users will see no data.

### Tasks

#### 1.1: Implement New RBAC Parsing Logic
**Repository**: playbook-dispatcher
**File**: `internal/api/rbac/transform.go`

**Current Code (breaks with new roles)**:
```go
func GetPredicateValues(permissions []Access, key string) (result []string) {
    for _, permission := range permissions {
        for _, resourceDefinition := range permission.ResourceDefinitions {
            // With new role format: ResourceDefinitions = []
            // This loop never executes
            // Returns empty array
        }
    }
    return
}
```

**New Implementation**:
```go
// GetAllowedServicesFromPermissions extracts allowed services from both old and new role formats
func GetAllowedServicesFromPermissions(permissions []Access, knownServices []string) []string {
    allowedServices := []string{}
    permissionMap := make(map[string]bool)

    // Build permission map
    for _, perm := range permissions {
        permissionMap[perm.Permission] = true
    }

    // Check each known service
    for _, service := range knownServices {
        // NEW FORMAT: playbook-dispatcher:remediations_run:read
        newFormatPerm := fmt.Sprintf("playbook-dispatcher:%s_run:read", service)
        if permissionMap[newFormatPerm] {
            allowedServices = append(allowedServices, service)
            continue
        }

        // OLD FORMAT: playbook-dispatcher:run:read with attribute filter
        for _, perm := range permissions {
            if perm.Permission == "playbook-dispatcher:run:read" {
                if hasServiceInAttributeFilter(perm.ResourceDefinitions, service) {
                    allowedServices = append(allowedServices, service)
                    break
                }
            }
        }
    }

    // Handle unrestricted access
    if len(allowedServices) == 0 {
        for _, perm := range permissions {
            if perm.Permission == "playbook-dispatcher:run:read" && len(perm.ResourceDefinitions) == 0 {
                return knownServices // Full access
            }
        }
    }

    return allowedServices
}

func hasServiceInAttributeFilter(resourceDefs []ResourceDefinition, service string) bool {
    for _, resDef := range resourceDefs {
        var opEqual ResourceDefinitionFilterOperationEqual
        if err := json.Unmarshal(resDef.AttributeFilter.union, &opEqual); err == nil {
            if opEqual.Key == "service" && opEqual.Value != nil && *opEqual.Value == service {
                return true
            }
        }

        var opIn ResourceDefinitionFilterOperationIn
        if err := json.Unmarshal(resDef.AttributeFilter.union, &opIn); err == nil {
            if opIn.Key == "service" {
                for _, val := range opIn.Value {
                    if val == service {
                        return true
                    }
                }
            }
        }
    }
    return false
}
```

**Steps**:
- [ ] Create `GetAllowedServicesFromPermissions()` function
- [ ] Implement `hasServiceInAttributeFilter()` helper
- [ ] Support both old format (with attributeFilter) and new format (without)
- [ ] Handle edge cases (no permissions, unrestricted access)

#### 1.2: Update Callers
**Repository**: playbook-dispatcher
**File**: `internal/api/controllers/public/kessel.go` (and other files using GetPredicateValues)

**Current Code**:
```go
permissions := middleware.GetPermissions(ctx)
allowedServices = rbac.GetPredicateValues(permissions, "service")
```

**Updated Code**:
```go
knownServices := []string{"remediations", "config_manager", "tasks"}
permissions := middleware.GetPermissions(ctx)
allowedServices = rbac.GetAllowedServicesFromPermissions(permissions, knownServices)
```

**Steps**:
- [ ] Find all usages of `GetPredicateValues()`
- [ ] Update to use `GetAllowedServicesFromPermissions()`
- [ ] Ensure knownServices list is passed correctly
- [ ] Test with old role format (should still work)

#### 1.3: Write Tests
**Repository**: playbook-dispatcher
**File**: `internal/api/rbac/transform_test.go`

**Test Cases**:
```go
func TestGetAllowedServices_OldFormat(t *testing.T) {
    // Test with current attribute filter format
    permissions := []Access{
        {
            Permission: "playbook-dispatcher:run:read",
            ResourceDefinitions: []ResourceDefinition{
                {AttributeFilter: /* service: remediations */},
            },
        },
    }

    services := GetAllowedServicesFromPermissions(permissions, knownServices)
    assert.Equal(t, []string{"remediations"}, services)
}

func TestGetAllowedServices_NewFormat(t *testing.T) {
    // Test with PR #699 format
    permissions := []Access{
        {Permission: "playbook-dispatcher:remediations_run:read"},
        {Permission: "playbook-dispatcher:config_manager_run:read"},
    }

    services := GetAllowedServicesFromPermissions(permissions, knownServices)
    assert.ElementsMatch(t, []string{"remediations", "config_manager"}, services)
}

func TestGetAllowedServices_MixedFormat(t *testing.T) {
    // Test mixed environment during migration
    permissions := []Access{
        {
            Permission: "playbook-dispatcher:run:read",
            ResourceDefinitions: []ResourceDefinition{
                {AttributeFilter: /* service: remediations */},
            },
        },
        {Permission: "playbook-dispatcher:config_manager_run:read"},
    }

    services := GetAllowedServicesFromPermissions(permissions, knownServices)
    assert.ElementsMatch(t, []string{"remediations", "config_manager"}, services)
}

func TestGetAllowedServices_NoPermissions(t *testing.T) {
    permissions := []Access{}
    services := GetAllowedServicesFromPermissions(permissions, knownServices)
    assert.Empty(t, services)
}

func TestGetAllowedServices_UnrestrictedAccess(t *testing.T) {
    // Test unrestricted access (empty resourceDefinitions)
    permissions := []Access{
        {
            Permission: "playbook-dispatcher:run:read",
            ResourceDefinitions: []ResourceDefinition{},
        },
    }

    services := GetAllowedServicesFromPermissions(permissions, knownServices)
    assert.ElementsMatch(t, knownServices, services)
}
```

**Steps**:
- [ ] Write unit tests for all scenarios
- [ ] Test old format (backward compatibility)
- [ ] Test new format (forward compatibility)
- [ ] Test mixed format (migration scenario)
- [ ] Test edge cases (no permissions, unrestricted)
- [ ] Achieve >90% test coverage

### Deliverables
- ✅ RBAC code supports both old and new role formats
- ✅ All unit tests passing
- ✅ Backward compatible with current roles
- ✅ Ready for deployment

### Dependencies
- None (self-contained code changes)

---

## Phase 2: Deploy RBAC Code Changes (Week 4-5)

### Objective
Deploy updated RBAC code to production while roles are still in old format.

### Critical Requirement
**Code must be tested and stable in production with OLD roles before any role updates begin.**

### Tasks

#### 2.1: Deploy to Ephemeral
**Steps**:
- [ ] Deploy updated code to ephemeral environment
- [ ] Configure with old role format
- [ ] Run smoke tests
- [ ] Verify service filtering works correctly
- [ ] Test with various permission scenarios

#### 2.2: Deploy to Stage
**Steps**:
- [ ] Deploy to stage environment
- [ ] Run full test suite
- [ ] Manual testing of authorization flows
- [ ] Load test to verify performance
- [ ] Soak test for 48 hours
- [ ] Verify no regressions

#### 2.3: Deploy to Production
**Steps**:
- [ ] Deploy to production
- [ ] Monitor rollout carefully
- [ ] Watch error rates and latency
- [ ] Verify authorization still works correctly
- [ ] Check logs for any parsing errors
- [ ] Soak for 72 hours minimum

#### 2.4: Verify Production Stability
**Metrics to Check**:
- Authorization success rate (should be unchanged)
- API latency (should be unchanged)
- Error rates (should be unchanged)
- User complaints (should be zero)

**Steps**:
- [ ] Review dashboards daily
- [ ] Check for any anomalies
- [ ] Confirm no support tickets related to authorization
- [ ] Get approval to proceed to role updates

### Deliverables
- ✅ RBAC code deployed to production
- ✅ Verified working with old role format
- ✅ No regressions or issues
- ✅ Production stable for 72+ hours

### Dependencies
- Phase 1 complete (code ready)

---

## Phase 3: Kessel Schema Creation (Week 4-5)

### Objective
Create centralized Kessel schema in playbook-dispatcher namespace.

**Note**: This can happen in parallel with Phase 2.

### Tasks

#### 3.1: Create playbook-dispatcher.ksl
**Repository**: rbac-config
**File**: `configs/prod/schemas/src/playbook-dispatcher.ksl` (new file)
**Owner**: Playbook-dispatcher team or central schema management

**Content**:
```ksl
version 0.1
namespace playbook_dispatcher

import rbac

# Remediations run permissions
@rbac.add_v1_based_permission(app:'playbook_dispatcher', resource:'remediations_run', verb:'read', v2_perm:'playbook_dispatcher_remediations_run_view');
@rbac.add_v1_based_permission(app:'playbook_dispatcher', resource:'remediations_run', verb:'write', v2_perm:'playbook_dispatcher_remediations_run_delete');

# Tasks run permissions
@rbac.add_v1_based_permission(app:'playbook_dispatcher', resource:'tasks_run', verb:'read', v2_perm:'playbook_dispatcher_tasks_run_view');
@rbac.add_v1_based_permission(app:'playbook_dispatcher', resource:'tasks_run', verb:'write', v2_perm:'playbook_dispatcher_tasks_run_delete');

# Config manager run permissions
@rbac.add_v1_based_permission(app:'playbook_dispatcher', resource:'config_manager_run', verb:'read', v2_perm:'playbook_dispatcher_config_manager_run_view');
@rbac.add_v1_based_permission(app:'playbook_dispatcher', resource:'config_manager_run', verb:'write', v2_perm:'playbook_dispatcher_config_manager_run_delete');
```

**Steps**:
- [ ] Create PR to rbac-config with new playbook-dispatcher.ksl
- [ ] Request review from platform/schema team
- [ ] Address feedback
- [ ] Merge PR
- [ ] Verify Kessel service ingests schema

### Deliverables
- ✅ playbook-dispatcher.ksl created and merged
- ✅ Schema deployed to Kessel service
- ✅ Permissions available in Kessel

### Dependencies
- Access to rbac-config repository

---

## Phase 4: RBAC Role Updates (Week 6)

### Objective
Update RBAC roles for all services to new format (remove attribute filters).

### Critical Requirement
**Phase 2 MUST be complete and stable before starting role updates.**

### Tasks

#### 4.1: Coordinate with Service Teams
**Teams**:
- Remediations team
- Config-Manager team
- Tasks team
- Any other services using playbook-dispatcher

**Communication**:
```
Subject: RBAC Role Update Required for Playbook-Dispatcher Kessel Migration

Teams,

We're migrating playbook-dispatcher to Kessel authorization. This requires updating
your RBAC role definitions to a new format.

IMPORTANT: The updated playbook-dispatcher code has been deployed and is working
with the current roles. This role update will not break anything.

OLD ROLE FORMAT:
{
  "permission": "playbook-dispatcher:run:read",
  "resourceDefinitions": [{
    "attributeFilter": {
      "key": "service",
      "value": "remediations"
    }
  }]
}

NEW ROLE FORMAT:
{
  "permission": "playbook-dispatcher:remediations_run:read"
}

ACTION REQUIRED:
- Update your role definitions in rbac-config
- Timeline: Week 6 (after RBAC code is stable)

Questions? Contact [playbook-dispatcher team]
```

**Steps**:
- [ ] Send communication to all service teams
- [ ] Schedule office hours for questions
- [ ] Prepare example PRs for each service
- [ ] Set target completion date

#### 4.2: Update Remediations Role
**Repository**: rbac-config
**File**: Role definition for remediations (path varies)
**Owner**: Remediations team

**OLD**:
```json
{
  "permission": "playbook-dispatcher:run:read",
  "resourceDefinitions": [{
    "attributeFilter": {
      "key": "service",
      "operation": "equal",
      "value": "remediations"
    }
  }]
}
```

**NEW**:
```json
{
  "permission": "playbook-dispatcher:remediations_run:read"
}
```

**Steps**:
- [ ] Remediations team creates PR with role update
- [ ] Review PR
- [ ] Merge PR
- [ ] Verify in stage environment
- [ ] Monitor production after role update deploys

#### 4.3: Update Config-Manager Role
**Repository**: rbac-config
**File**: Role definition for config-manager
**Owner**: Config-Manager team

**OLD**:
```json
{
  "permission": "playbook-dispatcher:run:read",
  "resourceDefinitions": [{
    "attributeFilter": {
      "key": "service",
      "value": "config_manager"
    }
  }]
}
```

**NEW**:
```json
{
  "permission": "playbook-dispatcher:config_manager_run:read"
}
```

**Steps**:
- [ ] Config-Manager team creates PR with role update
- [ ] Review PR
- [ ] Merge PR
- [ ] Verify in stage environment
- [ ] Monitor production after role update deploys

#### 4.4: Update Tasks Role
**Repository**: rbac-config
**File**: Role definition for tasks
**Owner**: Tasks team

**OLD**:
```json
{
  "permission": "playbook-dispatcher:run:read",
  "resourceDefinitions": [{
    "attributeFilter": {
      "key": "service",
      "value": "tasks"
    }
  }]
}
```

**NEW**:
```json
{
  "permission": "playbook-dispatcher:tasks_run:read"
}
```

**Steps**:
- [ ] Tasks team creates PR with role update
- [ ] Review PR
- [ ] Merge PR
- [ ] Verify in stage environment
- [ ] Monitor production after role update deploys

#### 4.5: Verify Role Updates
**Steps**:
- [ ] Confirm all role updates are merged
- [ ] Verify updates deployed to RBAC service
- [ ] Test authorization with new role format
- [ ] Monitor for any authorization failures
- [ ] Check support tickets for user issues

### Deliverables
- ✅ All RBAC roles updated to new format
- ✅ Roles deployed to production
- ✅ Authorization working correctly with new format
- ✅ No user impact or support tickets

### Dependencies
- Phase 2 complete (RBAC code stable in production)
- Phase 3 complete (Kessel schema exists)

---

## Phase 5: Kessel Client Implementation (Week 7-9)

### Objective
Implement Kessel permission checking in playbook-dispatcher.

### Tasks

#### 5.1: Add Kessel Dependencies
**Repository**: playbook-dispatcher
**Files**: `go.mod`, `go.sum`

**Steps**:
- [ ] Add Kessel client library dependency
```bash
go get github.com/project-kessel/relations-api/api/kessel/relations/v1beta1
```
- [ ] Run `go mod tidy`
- [ ] Commit dependency updates

#### 5.2: Add Configuration
**Repository**: playbook-dispatcher
**File**: `internal/common/config/config.go`

**Add Configuration Options**:
```go
// Kessel configuration
options.SetDefault("kessel.enabled", false)
options.SetDefault("kessel.dual_mode", false)
options.SetDefault("kessel.primary", "rbac")
options.SetDefault("kessel.compare_results", false)
options.SetDefault("kessel.fallback_rbac", true)
options.SetDefault("kessel.api.url", "")
options.SetDefault("kessel.api.timeout", "10s")
```

#### 5.3: Implement Kessel Client
**Repository**: playbook-dispatcher
**File**: `internal/api/kessel/client.go` (new file)

**Implementation**: (same as Holloway Plan)
```go
package kessel

import (
    "context"
    "fmt"

    kesselv2 "github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
    "google.golang.org/grpc"
)

type Client struct {
    conn   *grpc.ClientConn
    client kesselv2.KesselCheckServiceClient
}

func NewClient(url string) (*Client, error) {
    conn, err := grpc.Dial(url, grpc.WithInsecure())
    if err != nil {
        return nil, fmt.Errorf("failed to connect to Kessel: %w", err)
    }

    return &Client{
        conn:   conn,
        client: kesselv2.NewKesselCheckServiceClient(conn),
    }, nil
}

func (c *Client) Check(ctx context.Context, req *kesselv2.CheckRequest) (bool, error) {
    resp, err := c.client.Check(ctx, req)
    if err != nil {
        return false, err
    }
    return resp.Allowed == kesselv2.CheckResponse_ALLOWED_TRUE, nil
}

func (c *Client) Close() error {
    return c.conn.Close()
}
```

#### 5.4: Implement Service Filtering Logic
**Repository**: playbook-dispatcher
**File**: `internal/api/controllers/public/kessel.go`

**Implementation**:
```go
func getAllowedServices(ctx echo.Context) []string {
    knownServices := []string{"remediations", "config_manager", "tasks"}

    // Kessel path
    if cfg.GetBool("kessel.enabled") {
        return getKesselAllowedServices(ctx, knownServices)
    }

    // RBAC path (now with new format support)
    permissions := middleware.GetPermissions(ctx)
    return rbac.GetAllowedServicesFromPermissions(permissions, knownServices)
}

func getKesselAllowedServices(ctx echo.Context, knownServices []string) []string {
    identity := identityMiddleware.Get(ctx.Request().Context())
    userId, err := extractUserID(identity)
    if err != nil {
        utils.GetLogFromEcho(ctx).Errorw("Failed to extract user ID", "error", err)
        return nil
    }

    workspaceId, err := rbacClient.GetDefaultWorkspaceID(ctx.Request().Context(), identity.Identity.OrgID)
    if err != nil {
        utils.GetLogFromEcho(ctx).Errorw("Failed to get workspace ID", "error", err)
        return nil
    }

    allowedServices := []string{}

    for _, service := range knownServices {
        // PR #699 format: playbook_dispatcher_{service}_run_view
        permission := fmt.Sprintf("playbook_dispatcher_%s_run_view", service)

        allowed, err := kesselClient.Check(ctx.Request().Context(), &kesselv2.CheckRequest{
            Object: &kesselv2.ResourceReference{
                ResourceType: "workspace",
                ResourceId:   workspaceId,
                Reporter:     &kesselv2.ReporterReference{Type: "rbac"},
            },
            Relation: permission,
            Subject: &kesselv2.SubjectReference{
                Resource: &kesselv2.ResourceReference{
                    ResourceType: "principal",
                    ResourceId:   fmt.Sprintf("redhat/%s", userId),
                    Reporter:     &kesselv2.ReporterReference{Type: "rbac"},
                },
            },
        })

        if err != nil {
            utils.GetLogFromEcho(ctx).Errorw("Kessel check failed", "service", service, "error", err)
            continue
        }

        if allowed {
            allowedServices = append(allowedServices, service)
        }
    }

    return allowedServices
}
```

**Steps**:
- [ ] Implement `getAllowedServices()` with feature flag logic
- [ ] Implement `getKesselAllowedServices()` with PR #699 permission format
- [ ] Add workspace ID lookup
- [ ] Add error handling and logging
- [ ] Write unit tests
- [ ] Write integration tests

#### 5.5: Add Instrumentation
**Repository**: playbook-dispatcher
**File**: `internal/api/instrumentation/probes.go`

(Same metrics as Holloway Plan)

### Deliverables
- ✅ Kessel client integrated
- ✅ Service filtering logic implemented
- ✅ Feature flags configured
- ✅ Metrics and logging added
- ✅ Tests passing

### Dependencies
- Phase 4 complete (roles updated)

---

## Phase 6: Testing and Deployment (Week 10)

### Objective
Deploy Kessel client with feature flag disabled.

### Tasks

(Same as Holloway Plan Phase 3)

#### 6.1: Local Testing
#### 6.2: Ephemeral Environment
#### 6.3: Stage Environment
#### 6.4: Production Deployment

### Deliverables
- ✅ Code deployed to all environments
- ✅ KESSEL_ENABLED=false
- ✅ No regressions

### Dependencies
- Phase 5 complete

---

## Phase 7: Shadow Mode Validation (Week 11-12)

### Objective
Run RBAC and Kessel in parallel, validate correctness.

(Same process as Holloway Plan Phase 4)

### Tasks
#### 7.1: Enable Shadow Mode in Stage
#### 7.2: Analyze Comparison Results
#### 7.3: Enable Shadow Mode in Production (Gradual)

### Deliverables
- ✅ Shadow mode on 100% of pods
- ✅ 99%+ match rate
- ✅ All discrepancies resolved

---

## Phase 8: Switch to Kessel Primary (Week 13)

### Objective
Make Kessel primary authorization source.

(Same process as Holloway Plan Phase 5)

### Tasks
#### 8.1: Switch in Stage
#### 8.2: Switch in Production (Gradual)

### Deliverables
- ✅ Kessel primary on 100% of pods
- ✅ Performance within SLO
- ✅ No authorization failures

---

## Phase 9: Monitoring and Optimization (Week 14+)

### Objective
Monitor Kessel in production, optimize performance.

(Same as Holloway Plan Phase 6)

### Tasks
#### 9.1: Production Monitoring
#### 9.2: Performance Optimization
#### 9.3: Documentation
#### 9.4: Decide on RBAC Fallback

---

## Rollback Procedures

### Scenario 1: Issues During RBAC Code Deployment (Phase 2)
**Action**: Rollback deployment
**Steps**:
- [ ] Revert to previous code version
- [ ] Deploy rollback
- [ ] Verify service filtering works
- [ ] Fix issues in code
- [ ] Redeploy

### Scenario 2: Issues After Role Updates (Phase 4)
**Critical**: This is why Phase 2 must complete first!

**If code was NOT deployed first**:
- Production incident (users see no data)
- Must emergency deploy RBAC code changes
- High priority fix

**If code WAS deployed first**:
- New code handles new role format
- Should work correctly
- If issues: Revert role updates in rbac-config

### Scenario 3: Issues During Kessel Implementation (Phase 5-6)
**Action**: No impact - KESSEL_ENABLED=false
**Steps**: Fix code issues before enabling

### Scenario 4: Issues During Shadow Mode (Phase 7)
**Action**: Disable shadow mode
```bash
KESSEL_DUAL_MODE=false
```
**Impact**: None - still using RBAC

### Scenario 5: Issues After Kessel Switch (Phase 8)
**Action**: Rollback to RBAC
```bash
KESSEL_PRIMARY=rbac
# OR
KESSEL_ENABLED=false
```
**Impact**: Instant rollback

---

## Success Criteria

### Phase 1
- [ ] RBAC code supports both old and new formats
- [ ] Unit tests passing (>90% coverage)

### Phase 2
- [ ] RBAC code deployed to production
- [ ] Working with old role format
- [ ] Stable for 72+ hours

### Phase 3
- [ ] playbook-dispatcher.ksl created and deployed

### Phase 4
- [ ] All service roles updated
- [ ] Authorization working with new format
- [ ] No user complaints

### Phase 5
- [ ] Kessel client integrated
- [ ] Tests passing

### Phase 6
- [ ] Deployed to production (RBAC mode)
- [ ] No regressions

### Phase 7
- [ ] Shadow mode on 100% of pods
- [ ] 99%+ match rate

### Phase 8
- [ ] Kessel primary on 100% of pods
- [ ] Latency within SLO

### Phase 9
- [ ] Production stable for 30+ days
- [ ] Documentation complete

---

## Dependencies and Risks

### External Dependencies
- **Service teams**: Must update RBAC roles (remediations, config-manager, tasks)
- **RBAC service**: Must deploy role updates
- **Kessel service**: Must be available

### Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| RBAC code deployed after roles update | Low | **Critical** | Deploy code first (Phase 2 before Phase 4) |
| Service team delays role updates | Medium | Medium | Clear communication, coordination |
| Role updates break authorization | Low | High | RBAC code supports both formats |
| Kessel service outages | Low | High | RBAC fallback enabled |
| Permission mismatches | Medium | Medium | Shadow mode catches issues |

---

## Critical Deployment Order

**This sequence is MANDATORY**:

```
1. Phase 1: RBAC code refactoring (Week 1-3)
2. Phase 2: Deploy RBAC code to production (Week 4-5)
   → VERIFY STABLE WITH OLD ROLES
3. Phase 3: Create Kessel schema (Week 4-5, can be parallel)
4. Phase 4: Update RBAC roles (Week 6)
   → ONLY AFTER Phase 2 is stable
5. Phase 5+: Continue with Kessel implementation
```

**If roles update before code**:
- ❌ Production incident
- ❌ Users see no authorization data
- ❌ Emergency code deployment required

---

## Timeline Summary

| Phase | Duration | Key Milestone |
|-------|----------|---------------|
| Phase 1: RBAC Code | Week 1-3 | Code refactored |
| Phase 2: Deploy RBAC Code | Week 4-5 | Deployed & stable |
| Phase 3: Kessel Schema | Week 4-5 | Schema created |
| Phase 4: Role Updates | Week 6 | All roles updated |
| Phase 5: Kessel Implementation | Week 7-9 | Code complete |
| Phase 6: Deployment | Week 10 | Deployed (RBAC mode) |
| Phase 7: Shadow Mode | Week 11-12 | 99% match rate |
| Phase 8: Kessel Primary | Week 13 | 100% Kessel traffic |
| Phase 9: Monitoring | Week 14+ | Stable in production |

**Total**: 13-15 weeks to full Kessel production

---

**Document**: PR699-PLAN-IMPLEMENTATION.md
**Date**: 2025-11-13
**Approach**: PR #699 (Centralized Schema, RBAC Role Changes Required)
