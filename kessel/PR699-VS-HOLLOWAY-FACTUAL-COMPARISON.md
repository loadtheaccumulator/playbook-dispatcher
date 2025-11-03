# Factual Comparison: PR #699 vs Holloway Plan

**Date**: 2025-11-13
**Purpose**: Objective comparison of schema and role changes required for each approach
**Scope**: Facts only - what files change, who owns them, what coordination is required

---

## Executive Summary

| Aspect | PR #699 (Original) | Holloway Plan |
|--------|-------------------|---------------|
| **Kessel Schema Files Modified** | 1 | 3 |
| **Kessel Schema File Owners** | Playbook-dispatcher (or central) | 3 service teams |
| **RBAC Role Files Modified** | 3+ | 0 |
| **RBAC Role File Owners** | 3 service teams | N/A |
| **Cross-Team Coordination Required** | Yes (role changes) | Yes (schema changes) |
| **Playbook-Dispatcher RBAC Code Changes** | Yes | No |
| **Breaking Changes** | Yes (RBAC roles) | No |

---

## PR #699 Approach

**Source**: https://github.com/RedHatInsights/rbac-config/pull/699

### Kessel Schema Changes

#### Files Modified: 1

**File**: `configs/prod/schemas/src/playbook-dispatcher.ksl`
**Owner**: Playbook-dispatcher team or central schema management
**Action**: Create new file
**Cross-team coordination**: Not required for schema (self-managed)

**Content**:
```ksl
version 0.1
namespace playbook_dispatcher

import rbac

@rbac.add_v1_based_permission(app:'playbook_dispatcher', resource:'remediations_run', verb:'read', v2_perm:'playbook_dispatcher_remediations_run_view');
@rbac.add_v1_based_permission(app:'playbook_dispatcher', resource:'remediations_run', verb:'write', v2_perm:'playbook_dispatcher_remediations_run_delete');

@rbac.add_v1_based_permission(app:'playbook_dispatcher', resource:'tasks_run', verb:'read', v2_perm:'playbook_dispatcher_tasks_run_view');
@rbac.add_v1_based_permission(app:'playbook_dispatcher', resource:'tasks_run', verb:'write', v2_perm:'playbook_dispatcher_tasks_run_delete');

@rbac.add_v1_based_permission(app:'playbook_dispatcher', resource:'config_manager_run', verb:'read', v2_perm:'playbook_dispatcher_config_manager_run_view');
@rbac.add_v1_based_permission(app:'playbook_dispatcher', resource:'config_manager_run', verb:'write', v2_perm:'playbook_dispatcher_config_manager_run_delete');
```

### RBAC Role Changes

#### Files Modified: 3+

**File 1**: Role definition for remediations (exact path varies)
**Owner**: Remediations team
**Action**: Modify existing role
**Cross-team coordination**: Required

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

**File 2**: Role definition for config-manager
**Owner**: Config-manager team
**Action**: Modify existing role
**Cross-team coordination**: Required

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

**File 3**: Role definition for tasks
**Owner**: Tasks team
**Action**: Modify existing role
**Cross-team coordination**: Required

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

### Playbook-Dispatcher Code Changes

#### RBAC Code: Required

**File**: `internal/api/rbac/transform.go`
**Reason**: Current `GetPredicateValues()` function extracts service from `resourceDefinitions[].attributeFilter`. New role format removes `resourceDefinitions` array entirely.

**Current Code (breaks with new roles)**:
```go
func GetPredicateValues(permissions []Access, key string) (result []string) {
    for _, permission := range permissions {
        for _, resourceDefinition := range permission.ResourceDefinitions {
            // This loop is EMPTY with new role format
            // Returns [] regardless of permissions
        }
    }
    return
}
```

**Required New Code**:
```go
func GetAllowedServicesFromPermissions(permissions []Access, knownServices []string) []string {
    allowedServices := []string{}
    permissionMap := make(map[string]bool)

    for _, perm := range permissions {
        permissionMap[perm.Permission] = true
    }

    for _, service := range knownServices {
        // NEW FORMAT: playbook-dispatcher:remediations_run:read
        newFormatPerm := fmt.Sprintf("playbook-dispatcher:%s_run:read", service)
        if permissionMap[newFormatPerm] {
            allowedServices = append(allowedServices, service)
            continue
        }

        // OLD FORMAT: fallback for backward compatibility
        for _, perm := range permissions {
            if perm.Permission == "playbook-dispatcher:run:read" {
                if hasServiceInAttributeFilter(perm.ResourceDefinitions, service) {
                    allowedServices = append(allowedServices, service)
                    break
                }
            }
        }
    }

    return allowedServices
}
```

#### Kessel Code: Required

**File**: `internal/api/controllers/public/kessel.go` (or new file)
**Action**: Implement Kessel permission checking

```go
func getKesselAllowedServices(ctx echo.Context, knownServices []string) []string {
    allowedServices := []string{}

    for _, service := range knownServices {
        // PR #699 format: playbook_dispatcher_{service}_run_view
        permission := fmt.Sprintf("playbook_dispatcher_%s_run_view", service)

        allowed, err := kesselClient.Check(ctx, workspace, permission, user)
        if err == nil && allowed {
            allowedServices = append(allowedServices, service)
        }
    }

    return allowedServices
}
```

### Deployment Coordination

#### Critical Requirement: Code MUST deploy before roles update

**Correct Sequence**:
1. Deploy new RBAC code with `GetAllowedServicesFromPermissions()` (supports both old and new formats)
2. Verify in production with old roles
3. Service teams update their roles to new format
4. New code handles new format

**If roles update before code deploys**:
- `GetPredicateValues()` returns empty array
- Users see no runs or incorrect data
- Production incident

### Summary: PR #699 Changes

| Component | Files Changed | Owners | Cross-Team Coordination |
|-----------|--------------|--------|------------------------|
| Kessel Schema | 1 | Playbook-dispatcher | No |
| RBAC Roles | 3+ | 3+ service teams | Yes |
| RBAC Code | 1+ | Playbook-dispatcher | No |
| Kessel Code | 1+ | Playbook-dispatcher | No |
| **Total Cross-Team** | **3+ files** | **3+ teams** | **Yes (roles)** |

---

## Holloway Plan Approach

**Source**: `holloway_kessel_schema_implementation_idea.md`

### Kessel Schema Changes

#### Files Modified: 3

**File 1**: `configs/prod/schemas/src/remediations.ksl`
**Owner**: Remediations team
**Action**: Add line to existing file
**Cross-team coordination**: Required

**Content Added**:
```ksl
@rbac.add_v1_based_permission(app:'remediations', resource:'playbook_dispatcher_run', verb:'read', v2_perm:'remediations_playbook_dispatcher_run_view');
```

**File 2**: `configs/prod/schemas/src/config-manager.ksl`
**Owner**: Config-manager team
**Action**: Add line to existing file
**Cross-team coordination**: Required

**Content Added**:
```ksl
@rbac.add_v1_based_permission(app:'config_manager', resource:'playbook_dispatcher_run', verb:'read', v2_perm:'config_manager_playbook_dispatcher_run_view');
```

**File 3**: `configs/prod/schemas/src/tasks.ksl`
**Owner**: Tasks team
**Action**: Add line to existing file (or create file if doesn't exist)
**Cross-team coordination**: Required

**Content Added**:
```ksl
@rbac.add_v1_based_permission(app:'tasks', resource:'playbook_dispatcher_run', verb:'read', v2_perm:'tasks_playbook_dispatcher_run_view');
```

### RBAC Role Changes

#### Files Modified: 0

**No changes to RBAC roles** - they remain with attribute filters:

```json
{
  "permission": "playbook-dispatcher:run:read",
  "resourceDefinitions": [{
    "attributeFilter": {
      "key": "service",
      "value": "remediations"
    }
  }]
}
```

### Playbook-Dispatcher Code Changes

#### RBAC Code: Not Required

Existing `GetPredicateValues()` continues to work unchanged because RBAC role format is unchanged.

```go
// Existing code - continues to work
func GetPredicateValues(permissions []Access, key string) (result []string) {
    for _, permission := range permissions {
        for _, resourceDefinition := range permission.ResourceDefinitions {
            // Still populated with attributeFilter
            // Works as before
        }
    }
    return
}
```

#### Kessel Code: Required

**File**: `internal/api/controllers/public/kessel.go` (or new file)
**Action**: Implement Kessel permission checking

```go
func getAllowedServices(ctx echo.Context) []string {
    knownServices := []string{"remediations", "config_manager", "tasks"}

    // Kessel path
    if cfg.GetBool("kessel.enabled") {
        return getKesselAllowedServices(ctx, knownServices)
    }

    // RBAC path - UNCHANGED
    permissions := middleware.GetPermissions(ctx)
    return rbac.GetPredicateValues(permissions, "service")
}

func getKesselAllowedServices(ctx echo.Context, knownServices []string) []string {
    allowedServices := []string{}

    for _, service := range knownServices {
        // Holloway format: {service}_playbook_dispatcher_run_view
        permission := fmt.Sprintf("%s_playbook_dispatcher_run_view", service)

        allowed, err := kesselClient.Check(ctx, workspace, permission, user)
        if err == nil && allowed {
            allowedServices = append(allowedServices, service)
        }
    }

    return allowedServices
}
```

### Deployment Coordination

#### No Critical Timing Requirements

**Flexible Sequence**:
1. Service teams add Kessel schema lines (any order, any timing)
2. Playbook-dispatcher deploys Kessel client code with `KESSEL_ENABLED=false`
3. Enable Kessel when ready via feature flag

**RBAC continues working throughout** - no breaking changes.

### Summary: Holloway Plan Changes

| Component | Files Changed | Owners | Cross-Team Coordination |
|-----------|--------------|--------|------------------------|
| Kessel Schema | 3 | 3 service teams | Yes |
| RBAC Roles | 0 | N/A | No |
| RBAC Code | 0 | N/A | No |
| Kessel Code | 1+ | Playbook-dispatcher | No |
| **Total Cross-Team** | **3 files** | **3 teams** | **Yes (schemas)** |

---

## Side-by-Side Comparison

### Files Modified

| File Type | PR #699 | Holloway Plan |
|-----------|---------|---------------|
| **Kessel Schema** | 1 (playbook-dispatcher.ksl) | 3 (remediations.ksl, config-manager.ksl, tasks.ksl) |
| **RBAC Roles** | 3+ (all service roles) | 0 |
| **RBAC Code** | 1+ (transform.go, etc.) | 0 |
| **Kessel Code** | 1+ (new implementation) | 1+ (new implementation) |
| **Total Files** | 5+ | 4+ |

### File Ownership

| Component | PR #699 Owner | Holloway Plan Owner |
|-----------|--------------|-------------------|
| **Kessel Schema** | Playbook-dispatcher team | Remediations, Config-manager, Tasks teams |
| **RBAC Roles** | Remediations, Config-manager, Tasks teams | N/A |
| **RBAC Code** | Playbook-dispatcher team | N/A |
| **Kessel Code** | Playbook-dispatcher team | Playbook-dispatcher team |

### Cross-Team Coordination Required

| Aspect | PR #699 | Holloway Plan |
|--------|---------|---------------|
| **For Kessel Schemas** | No | Yes (3 teams) |
| **For RBAC Roles** | Yes (3 teams) | No |
| **Total Teams Involved** | 3+ teams | 3+ teams |
| **Type of Changes** | Role updates (modify existing) | Schema additions (add lines) |
| **Breaking Changes** | Yes (role format changes) | No |

### Permission Format

| Aspect | PR #699 | Holloway Plan |
|--------|---------|---------------|
| **Kessel V2 Permission** | `playbook_dispatcher_remediations_run_view` | `remediations_playbook_dispatcher_run_view` |
| **RBAC V1 Permission (new)** | `playbook-dispatcher:remediations_run:read` | (unchanged) `playbook-dispatcher:run:read` |
| **Namespace** | `playbook_dispatcher` | `remediations`, `config_manager`, `tasks` |
| **Semantic Meaning** | Dispatcher owns runs for each service | Each service owns dispatcher access |

### Code Complexity

| Component | PR #699 | Holloway Plan |
|-----------|---------|---------------|
| **RBAC Code Changes** | Required (new parsing logic) | Not required |
| **RBAC Backward Compatibility** | Must implement for old + new formats | Native (no format change) |
| **Kessel Code Complexity** | Similar | Similar |
| **Feature Flags** | Required | Required |
| **Dual-Mode Support** | Required (RBAC + Kessel) | Required (RBAC + Kessel) |

### Deployment Requirements

| Aspect | PR #699 | Holloway Plan |
|--------|---------|---------------|
| **Critical Deployment Order** | Yes (code before roles) | No |
| **Risk if Wrong Order** | Production incident (users see no data) | None (no breaking change) |
| **Rollback Complexity** | Medium (must revert code and/or roles) | Low (toggle feature flag) |
| **Can Deploy Incrementally** | No (requires coordination) | Yes (independent components) |

---

## Change Request Process

### PR #699 Process

#### Step 1: Kessel Schema
**Action**: Create playbook-dispatcher.ksl
**Who**: Playbook-dispatcher team
**Coordination**: None required (self-managed)

#### Step 2: RBAC Code
**Action**: Implement new RBAC parsing logic
**Who**: Playbook-dispatcher team
**Coordination**: None required (self-managed)

#### Step 3: Deploy RBAC Code
**Action**: Deploy to production
**Who**: Playbook-dispatcher team
**Coordination**: None required (self-managed)

#### Step 4: Update RBAC Roles
**Action**: Modify role definitions for remediations, config-manager, tasks
**Who**: Remediations team, Config-manager team, Tasks team
**Coordination**: Required (3+ teams must make changes)

#### Step 5: Deploy Kessel Client
**Action**: Implement and deploy Kessel code
**Who**: Playbook-dispatcher team
**Coordination**: None required (self-managed)

**Total Steps Requiring Cross-Team Coordination**: 1 (Step 4)
**Total Teams Involved**: 3+ teams
**Critical Timing**: Yes (Step 3 must complete before Step 4)

### Holloway Plan Process

#### Step 1: Update Kessel Schemas
**Action**: Add playbook_dispatcher_run permission to remediations.ksl, config-manager.ksl, tasks.ksl
**Who**: Remediations team, Config-manager team, Tasks team
**Coordination**: Required (3+ teams must make changes)

#### Step 2: Deploy Kessel Client
**Action**: Implement and deploy Kessel code with KESSEL_ENABLED=false
**Who**: Playbook-dispatcher team
**Coordination**: None required (self-managed)

#### Step 3: Enable Kessel
**Action**: Toggle feature flag when ready
**Who**: Playbook-dispatcher team
**Coordination**: None required (self-managed)

**Total Steps Requiring Cross-Team Coordination**: 1 (Step 1)
**Total Teams Involved**: 3+ teams
**Critical Timing**: No (RBAC continues working regardless)

---

## Breaking Change Analysis

### PR #699 Breaking Changes

**Component**: RBAC Role Format
**What Breaks**: `resourceDefinitions` array removed from roles
**Impact**: Existing playbook-dispatcher RBAC code (`GetPredicateValues()`) returns empty array
**User Impact**: Users see no runs or incorrect authorization
**Mitigation**: Deploy new RBAC code before roles update
**Severity**: High (production incident if deployed in wrong order)

### Holloway Plan Breaking Changes

**None** - All changes are additive:
- Kessel schemas: New permissions added (doesn't affect existing)
- RBAC roles: Unchanged (attribute filters remain)
- Code: Adds Kessel path but RBAC path unchanged

---

## Feature Flag Usage

### Both Approaches Use Same Flags

```bash
# Phase 1: Development
KESSEL_ENABLED=false  # Use RBAC only

# Phase 2: Shadow mode
KESSEL_ENABLED=true
KESSEL_DUAL_MODE=true
KESSEL_PRIMARY=rbac
KESSEL_COMPARE_RESULTS=true

# Phase 3: Switch to Kessel
KESSEL_ENABLED=true
KESSEL_DUAL_MODE=false
KESSEL_PRIMARY=kessel
KESSEL_FALLBACK_RBAC=true
```

**Difference in Safety**:
- **PR #699**: Must coordinate with RBAC role updates (breaking change exists regardless of Kessel)
- **Holloway Plan**: Can toggle freely (RBAC unchanged, no breaking change)

---

## Summary Table

| Aspect | PR #699 (Original) | Holloway Plan |
|--------|-------------------|---------------|
| **Kessel Schema Files** | 1 | 3 |
| **Kessel Schema Owners** | Playbook-dispatcher | 3 service teams |
| **RBAC Role Files** | 3+ | 0 |
| **RBAC Role Owners** | 3 service teams | N/A |
| **RBAC Code Changes** | Yes (required) | No |
| **Breaking Changes** | Yes (RBAC roles) | No |
| **Cross-Team Coordination** | Yes (role updates) | Yes (schema additions) |
| **Teams Involved** | 3+ | 3+ |
| **Critical Deployment Order** | Yes | No |
| **Can Rollback via Flag** | Yes (but RBAC change remains) | Yes (fully) |
| **Backward Compatible** | Must implement | Native |

---

## Factual Conclusion

Both approaches require cross-team coordination:

- **PR #699**: Requires 3+ service teams to update their RBAC role files
- **Holloway Plan**: Requires 3+ service teams to update their Kessel schema files

Key differences:

1. **Breaking Changes**: PR #699 introduces breaking changes to RBAC role format; Holloway does not
2. **RBAC Code**: PR #699 requires RBAC code refactoring; Holloway does not
3. **Deployment Timing**: PR #699 requires critical deployment order (code before roles); Holloway does not
4. **File Ownership**: PR #699 modifies centralized schema (playbook-dispatcher owns) + distributed roles (services own); Holloway modifies distributed schemas (services own)

---

**Document**: PR699-VS-HOLLOWAY-FACTUAL-COMPARISON.md
**Date**: 2025-11-13
**Type**: Factual analysis (no opinions on coordination complexity)
