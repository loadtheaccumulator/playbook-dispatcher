# Holloway Plan: Implementation Plan

**Date**: 2025-11-13
**Approach**: Distributed Kessel schemas across service namespaces, no RBAC role changes
**Timeline**: 6-10 weeks
**Breaking Changes**: None

---

## Overview

The Holloway Plan implements Kessel authorization by:
1. Adding Kessel permissions to each service's existing `.ksl` schema file
2. Keeping RBAC roles unchanged (attribute filters remain)
3. Using feature flags to toggle between RBAC and Kessel
4. No RBAC code refactoring required

---

## Phase 1: Kessel Schema Updates (Week 1-2)

### Objective
Add `playbook_dispatcher_run` permissions to each service's Kessel schema.

### Tasks

#### 1.1: Update remediations.ksl
**Repository**: rbac-config
**File**: `configs/prod/schemas/src/remediations.ksl`
**Owner**: Remediations team
**Action**: Add permission line

**Change**:
```ksl
# Add to existing remediations.ksl file
@rbac.add_v1_based_permission(app:'remediations', resource:'playbook_dispatcher_run', verb:'read', v2_perm:'remediations_playbook_dispatcher_run_view');
@rbac.add_v1_based_permission(app:'remediations', resource:'playbook_dispatcher_run', verb:'write', v2_perm:'remediations_playbook_dispatcher_run_delete');
```

**Steps**:
- [ ] Create PR to rbac-config with changes to remediations.ksl
- [ ] Request review from Remediations team
- [ ] Address feedback
- [ ] Merge PR

#### 1.2: Update config-manager.ksl
**Repository**: rbac-config
**File**: `configs/prod/schemas/src/config-manager.ksl`
**Owner**: Config-Manager team
**Action**: Add permission line

**Change**:
```ksl
# Add to existing config-manager.ksl file
@rbac.add_v1_based_permission(app:'config_manager', resource:'playbook_dispatcher_run', verb:'read', v2_perm:'config_manager_playbook_dispatcher_run_view');
@rbac.add_v1_based_permission(app:'config_manager', resource:'playbook_dispatcher_run', verb:'write', v2_perm:'config_manager_playbook_dispatcher_run_delete');
```

**Steps**:
- [ ] Create PR to rbac-config with changes to config-manager.ksl
- [ ] Request review from Config-Manager team
- [ ] Address feedback
- [ ] Merge PR

#### 1.3: Update tasks.ksl
**Repository**: rbac-config
**File**: `configs/prod/schemas/src/tasks.ksl`
**Owner**: Tasks team
**Action**: Add permission line (or create file if doesn't exist)

**Change**:
```ksl
# If file exists, add these lines
# If file doesn't exist, create with:
version 0.1
namespace tasks

import rbac

@rbac.add_v1_based_permission(app:'tasks', resource:'playbook_dispatcher_run', verb:'read', v2_perm:'tasks_playbook_dispatcher_run_view');
@rbac.add_v1_based_permission(app:'tasks', resource:'playbook_dispatcher_run', verb:'write', v2_perm:'tasks_playbook_dispatcher_run_delete');
```

**Steps**:
- [ ] Check if tasks.ksl exists
- [ ] Create PR to rbac-config (add to existing or create new file)
- [ ] Request review from Tasks team
- [ ] Address feedback
- [ ] Merge PR

#### 1.4: Verify Schema Updates
**Steps**:
- [ ] Confirm all 3 schema PRs are merged
- [ ] Verify Kessel service has ingested updated schemas
- [ ] Test permission checks in Kessel (manual verification)

### Deliverables
- ✅ remediations.ksl updated with playbook_dispatcher_run permissions
- ✅ config-manager.ksl updated with playbook_dispatcher_run permissions
- ✅ tasks.ksl updated with playbook_dispatcher_run permissions
- ✅ All changes deployed to Kessel service

### Dependencies
- Access to rbac-config repository
- Cooperation from Remediations, Config-Manager, and Tasks teams

---

## Phase 2: Kessel Client Implementation (Week 3-5)

### Objective
Implement Kessel permission checking in playbook-dispatcher without affecting existing RBAC functionality.

### Tasks

#### 2.1: Add Kessel Dependencies
**Repository**: playbook-dispatcher
**Files**: `go.mod`, `go.sum`

**Steps**:
- [ ] Add Kessel client library dependency
```bash
go get github.com/project-kessel/relations-api/api/kessel/relations/v1beta1
```
- [ ] Run `go mod tidy`
- [ ] Commit dependency updates

#### 2.2: Add Configuration
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

**Steps**:
- [ ] Add Kessel configuration options
- [ ] Update configuration documentation
- [ ] Add environment variable mappings

#### 2.3: Implement Kessel Client
**Repository**: playbook-dispatcher
**File**: `internal/api/kessel/client.go` (new file)

**Implementation**:
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

**Steps**:
- [ ] Create kessel package
- [ ] Implement Client struct
- [ ] Add connection management
- [ ] Add error handling
- [ ] Write unit tests

#### 2.4: Implement Service Filtering Logic
**Repository**: playbook-dispatcher
**File**: `internal/api/controllers/public/kessel.go` (new or update existing)

**Implementation**:
```go
func getAllowedServices(ctx echo.Context) []string {
    knownServices := []string{"remediations", "config_manager", "tasks"}

    // Kessel path
    if cfg.GetBool("kessel.enabled") {
        return getKesselAllowedServices(ctx, knownServices)
    }

    // RBAC path (unchanged)
    permissions := middleware.GetPermissions(ctx)
    return rbac.GetPredicateValues(permissions, "service")
}

func getKesselAllowedServices(ctx echo.Context, knownServices []string) []string {
    // Get identity from context
    identity := identityMiddleware.Get(ctx.Request().Context())
    userId, err := extractUserID(identity)
    if err != nil {
        utils.GetLogFromEcho(ctx).Errorw("Failed to extract user ID", "error", err)
        return nil
    }

    // Get workspace ID
    workspaceId, err := rbacClient.GetDefaultWorkspaceID(ctx.Request().Context(), identity.Identity.OrgID)
    if err != nil {
        utils.GetLogFromEcho(ctx).Errorw("Failed to get workspace ID", "error", err)
        return nil
    }

    allowedServices := []string{}

    for _, service := range knownServices {
        // Holloway format: {service}_playbook_dispatcher_run_view
        permission := fmt.Sprintf("%s_playbook_dispatcher_run_view", service)

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

func extractUserID(identity *identityMiddleware.Identity) (string, error) {
    if identity == nil || identity.Identity.User == nil {
        return "", fmt.Errorf("no user in identity")
    }
    return identity.Identity.User.Username, nil
}
```

**Steps**:
- [ ] Implement `getAllowedServices()` with feature flag logic
- [ ] Implement `getKesselAllowedServices()` with Holloway permission format
- [ ] Add workspace ID lookup
- [ ] Add error handling and logging
- [ ] Write unit tests
- [ ] Write integration tests

#### 2.5: Add Instrumentation
**Repository**: playbook-dispatcher
**File**: `internal/api/instrumentation/probes.go`

**Add Metrics**:
```go
var (
    kesselChecksTotal = promauto.NewCounterVec(prometheus.CounterOpts{
        Name: "playbook_dispatcher_kessel_checks_total",
        Help: "Total number of Kessel permission checks",
    }, []string{"service", "result"})

    kesselCheckDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
        Name: "playbook_dispatcher_kessel_check_duration_seconds",
        Help: "Duration of Kessel permission checks",
        Buckets: prometheus.ExponentialBuckets(0.001, 2, 10),
    }, []string{"service"})

    kesselComparisonTotal = promauto.NewCounterVec(prometheus.CounterOpts{
        Name: "playbook_dispatcher_kessel_comparison_total",
        Help: "Results of RBAC vs Kessel comparison",
    }, []string{"result"}) // "match", "mismatch", "rbac_only", "kessel_only"
)
```

**Steps**:
- [ ] Add Kessel-specific metrics
- [ ] Add logging for Kessel operations
- [ ] Add tracing spans
- [ ] Test metric collection

### Deliverables
- ✅ Kessel client integrated into playbook-dispatcher
- ✅ Service filtering logic implemented
- ✅ Feature flags configured
- ✅ Metrics and logging added
- ✅ Unit and integration tests passing

### Dependencies
- Phase 1 complete (schemas deployed)
- Kessel service available and accessible

---

## Phase 3: Testing and Deployment (Week 6-7)

### Objective
Deploy Kessel client to all environments with feature flag disabled (RBAC only mode).

### Tasks

#### 3.1: Local Testing
**Steps**:
- [ ] Test with KESSEL_ENABLED=false (verify RBAC still works)
- [ ] Test with KESSEL_ENABLED=true (verify Kessel checks work)
- [ ] Test with invalid Kessel configuration (verify fallback)
- [ ] Test permission edge cases (no permissions, all permissions, partial permissions)

#### 3.2: Ephemeral Environment
**Steps**:
- [ ] Deploy to ephemeral environment
- [ ] Configure Kessel connection (KESSEL_ENABLED=false initially)
- [ ] Run smoke tests
- [ ] Verify RBAC authorization still works
- [ ] Enable Kessel (KESSEL_ENABLED=true)
- [ ] Test Kessel authorization
- [ ] Verify metrics are collected

#### 3.3: Stage Environment
**Steps**:
- [ ] Deploy to stage environment with KESSEL_ENABLED=false
- [ ] Run full test suite
- [ ] Verify no regression in RBAC functionality
- [ ] Soak test for 24 hours
- [ ] Monitor error rates and performance

#### 3.4: Production Deployment
**Steps**:
- [ ] Deploy to production with KESSEL_ENABLED=false
- [ ] Monitor deployment rollout
- [ ] Verify RBAC continues working
- [ ] Check error rates and latency
- [ ] Soak for 48 hours before enabling Kessel

### Deliverables
- ✅ Code deployed to all environments
- ✅ KESSEL_ENABLED=false (RBAC only mode)
- ✅ No regressions in existing functionality
- ✅ Monitoring and alerting configured

---

## Phase 4: Shadow Mode Validation (Week 8-9)

### Objective
Run RBAC and Kessel in parallel, compare results, validate correctness.

### Tasks

#### 4.1: Enable Shadow Mode in Stage
**Configuration**:
```bash
KESSEL_ENABLED=true
KESSEL_DUAL_MODE=true
KESSEL_PRIMARY=rbac
KESSEL_COMPARE_RESULTS=true
KESSEL_FALLBACK_RBAC=true
```

**Steps**:
- [ ] Update stage configuration
- [ ] Deploy configuration change
- [ ] Monitor comparison metrics
- [ ] Analyze mismatches
- [ ] Fix any discrepancies

#### 4.2: Analyze Comparison Results
**Queries**:
```promql
# Match rate
rate(playbook_dispatcher_kessel_comparison_total{result="match"}[5m])
  / rate(playbook_dispatcher_kessel_comparison_total[5m])

# Mismatch rate
rate(playbook_dispatcher_kessel_comparison_total{result="mismatch"}[5m])
  / rate(playbook_dispatcher_kessel_comparison_total[5m])
```

**Steps**:
- [ ] Review mismatch logs
- [ ] Identify patterns in mismatches
- [ ] Determine root causes (schema issues, permission issues, code issues)
- [ ] Fix identified issues
- [ ] Redeploy and retest
- [ ] Achieve 99%+ match rate

#### 4.3: Enable Shadow Mode in Production (Gradual)
**Week 8 - 10% of pods**:
- [ ] Enable shadow mode on 10% of production pods
- [ ] Monitor for 48 hours
- [ ] Verify match rate remains high

**Week 9 - Scale up**:
- [ ] Increase to 25% of pods
- [ ] Monitor for 24 hours
- [ ] Increase to 50% of pods
- [ ] Monitor for 24 hours
- [ ] Increase to 100% of pods
- [ ] Monitor for 48 hours

**Steps at each stage**:
- [ ] Check comparison metrics
- [ ] Review mismatch logs
- [ ] Verify no performance degradation
- [ ] Check error rates

### Deliverables
- ✅ Shadow mode running on 100% of production pods
- ✅ 99%+ match rate between RBAC and Kessel
- ✅ All mismatches investigated and resolved
- ✅ Performance within acceptable limits

---

## Phase 5: Switch to Kessel Primary (Week 10-11)

### Objective
Make Kessel the primary authorization source while keeping RBAC as fallback.

### Tasks

#### 5.1: Switch in Stage
**Configuration**:
```bash
KESSEL_ENABLED=true
KESSEL_DUAL_MODE=false
KESSEL_PRIMARY=kessel
KESSEL_FALLBACK_RBAC=true
```

**Steps**:
- [ ] Update stage configuration
- [ ] Deploy configuration change
- [ ] Run full test suite
- [ ] Verify authorization works correctly
- [ ] Test fallback scenario (simulate Kessel outage)
- [ ] Monitor for 48 hours

#### 5.2: Switch in Production (Gradual)
**Week 10 - 10% of traffic**:
```bash
# Configure 10% of pods
KESSEL_ENABLED=true
KESSEL_DUAL_MODE=false
KESSEL_PRIMARY=kessel
KESSEL_FALLBACK_RBAC=true
```

**Steps**:
- [ ] Update configuration for 10% of pods
- [ ] Deploy configuration change
- [ ] Monitor error rates and 403 responses
- [ ] Check latency metrics
- [ ] Verify no user complaints
- [ ] Monitor for 48 hours

**Week 10-11 - Scale up**:
- [ ] Increase to 25% of pods (monitor 24 hours)
- [ ] Increase to 50% of pods (monitor 24 hours)
- [ ] Increase to 75% of pods (monitor 24 hours)
- [ ] Increase to 100% of pods (monitor 48 hours)

**Steps at each stage**:
- [ ] Monitor authorization success rate
- [ ] Check 403 error rates
- [ ] Verify performance metrics
- [ ] Review user feedback/tickets
- [ ] Check fallback invocations (should be minimal)

### Deliverables
- ✅ Kessel is primary authorization source on 100% of pods
- ✅ RBAC fallback enabled
- ✅ No increase in authorization failures
- ✅ Performance within SLO

---

## Phase 6: Monitoring and Optimization (Week 12+)

### Objective
Monitor Kessel in production, optimize performance, plan for RBAC fallback removal (optional).

### Tasks

#### 6.1: Production Monitoring
**Metrics to Track**:
- Kessel check success rate
- Kessel check latency (p50, p95, p99)
- RBAC fallback invocation rate
- Authorization denial rate
- User complaints/tickets

**Steps**:
- [ ] Set up dashboards for Kessel metrics
- [ ] Configure alerts for anomalies
- [ ] Review weekly performance reports
- [ ] Optimize slow permission checks

#### 6.2: Performance Optimization
**Areas to Investigate**:
- [ ] Batch permission checks if possible
- [ ] Cache workspace ID lookups
- [ ] Optimize Kessel client connection pooling
- [ ] Review and optimize permission check patterns

#### 6.3: Documentation
**Steps**:
- [ ] Document Kessel implementation
- [ ] Update runbooks for Kessel issues
- [ ] Create troubleshooting guide
- [ ] Document rollback procedures
- [ ] Update architecture diagrams

#### 6.4: Decide on RBAC Fallback
**Options**:

**Option A: Keep RBAC fallback indefinitely**
- Provides safety net
- Allows instant rollback via feature flag
- Maintains dual system complexity

**Option B: Remove RBAC fallback after confidence period**
- Simplifies codebase
- Reduces dual-system overhead
- Requires high confidence in Kessel

**Steps**:
- [ ] Review Kessel reliability metrics (after 3+ months)
- [ ] Assess risk of removing RBAC fallback
- [ ] Make decision with stakeholders
- [ ] If removing: Plan removal implementation
- [ ] If keeping: Document long-term dual-system strategy

### Deliverables
- ✅ Production monitoring established
- ✅ Performance optimized
- ✅ Documentation complete
- ✅ Decision made on RBAC fallback retention

---

## Rollback Procedures

### Scenario 1: Issues During Schema Updates (Phase 1)
**Action**: No impact - schemas are additive and not yet used
**Steps**: Fix schema issues and redeploy

### Scenario 2: Issues During Development (Phase 2-3)
**Action**: No impact - KESSEL_ENABLED=false
**Steps**: Fix code issues before enabling Kessel

### Scenario 3: Issues During Shadow Mode (Phase 4)
**Action**: Disable shadow mode
```bash
KESSEL_DUAL_MODE=false
```
**Impact**: None - still using RBAC primary
**Steps**: Investigate issues, fix, re-enable

### Scenario 4: Issues After Kessel Primary Switch (Phase 5)
**Action**: Immediate rollback to RBAC
```bash
KESSEL_PRIMARY=rbac
# OR
KESSEL_ENABLED=false
```
**Impact**: Instant rollback, users restored to RBAC
**Steps**:
- [ ] Identify root cause
- [ ] Fix issue
- [ ] Test in stage
- [ ] Retry switch to Kessel

---

## Success Criteria

### Phase 1
- [ ] All 3 Kessel schema files updated
- [ ] Schemas deployed to Kessel service

### Phase 2
- [ ] Kessel client integrated
- [ ] Feature flags working
- [ ] Unit tests passing (>80% coverage)
- [ ] Integration tests passing

### Phase 3
- [ ] Deployed to all environments
- [ ] RBAC functionality unchanged
- [ ] No production incidents

### Phase 4
- [ ] Shadow mode running on 100% of pods
- [ ] 99%+ match rate RBAC vs Kessel
- [ ] Performance within baseline

### Phase 5
- [ ] Kessel primary on 100% of pods
- [ ] No increase in 403 errors
- [ ] Latency within SLO (p95 < 100ms for authorization)

### Phase 6
- [ ] Production stable for 30+ days
- [ ] Documentation complete
- [ ] Team trained on Kessel operations

---

## Dependencies and Risks

### External Dependencies
- **Remediations team**: Must review and merge remediations.ksl changes
- **Config-Manager team**: Must review and merge config-manager.ksl changes
- **Tasks team**: Must review and merge tasks.ksl changes
- **Kessel service**: Must be available and operational
- **RBAC service**: Still needed for workspace ID lookup

### Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Service team delays schema merge | Medium | Medium | Start early, follow up regularly |
| Kessel service outages | Low | High | RBAC fallback enabled |
| Permission mismatches | Medium | Medium | Shadow mode catches issues |
| Performance degradation | Low | Medium | Load testing, gradual rollout |

---

## Timeline Summary

| Phase | Duration | Key Milestone |
|-------|----------|---------------|
| Phase 1: Schemas | Week 1-2 | All schemas merged |
| Phase 2: Implementation | Week 3-5 | Code complete |
| Phase 3: Deployment | Week 6-7 | Deployed to prod (RBAC mode) |
| Phase 4: Shadow Mode | Week 8-9 | 99% match rate achieved |
| Phase 5: Kessel Primary | Week 10-11 | 100% Kessel traffic |
| Phase 6: Monitoring | Week 12+ | Stable in production |

**Total**: 10-12 weeks to full Kessel production

---

**Document**: HOLLOWAY-PLAN-IMPLEMENTATION.md
**Date**: 2025-11-13
**Approach**: Holloway Plan (Distributed Schemas, No RBAC Changes)
