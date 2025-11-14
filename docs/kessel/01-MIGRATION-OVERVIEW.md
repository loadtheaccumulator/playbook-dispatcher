# RBAC to Kessel Migration Overview

## Executive Summary

This document outlines the migration strategy for replacing the RBAC authorization system with Kessel in the playbook-dispatcher application. The migration will be implemented using feature flags to enable parallel operation of both authorization systems during the transition period.

## Background

### Current State: RBAC Implementation

Playbook-dispatcher currently uses Red Hat's RBAC (Role-Based Access Control) service for authorization:

- **Primary Permission**: `playbook-dispatcher:run:read`
- **Authorization Mechanism**: HTTP REST API calls to RBAC service
- **Attribute Filtering**: Service-level filtering based on resource definitions
- **Supported Services**: remediations, config_manager, tasks, test, edge

### Target State: Kessel Authorization

Kessel is a next-generation authorization service based on relationship-based access control (ReBAC):

- **Protocol**: gRPC-based API
- **Permission Model**: Workspace-based authorization with service-specific permissions
- **New Permissions** (from PR #699):
  - `playbook_dispatcher_remediations_run_view`
  - `playbook_dispatcher_tasks_run_view`
  - `playbook_dispatcher_config_manager_run_view`
- **Backward Compatibility**: V1-only permissions during transition
  - `playbook_dispatcher_run_read`
  - `playbook_dispatcher_run_write`

## Migration Goals

1. **Zero Downtime**: Enable seamless transition without service interruption
2. **Parallel Operation**: Run RBAC and Kessel side-by-side during migration
3. **Gradual Rollout**: Use feature flags to control authorization system selection
4. **Backward Compatibility**: Maintain existing API contracts and behavior
5. **Observability**: Comprehensive metrics and logging for both systems

## Key Changes from PR #699

### Permission Structure Evolution

#### V1 Permissions (RBAC - Current)
```
playbook-dispatcher:run:read
playbook-dispatcher:run:write
```

With attribute filtering:
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

#### V2 Permissions (Kessel - Target)
```
playbook_dispatcher_remediations_run_view
playbook_dispatcher_tasks_run_view
playbook_dispatcher_config_manager_run_view
```

### Schema Definition (playbook-dispatcher.ksl)

```ksl
version 0.1

namespace playbook_dispatcher

import rbac

@rbac.add_v1_based_permission(
  app:'playbook_dispatcher',
  resource:'remediations_run',
  verb:'read',
  v2_perm:'playbook_dispatcher_remediations_run_view'
);

@rbac.add_v1_based_permission(
  app:'playbook_dispatcher',
  resource:'tasks_run',
  verb:'read',
  v2_perm:'playbook_dispatcher_tasks_run_view'
);

@rbac.add_v1_based_permission(
  app:'playbook_dispatcher',
  resource:'config_manager_run',
  verb:'read',
  v2_perm:'playbook_dispatcher_config_manager_run_view'
);

// Placeholders for V1 permissions during migration
@rbac.add_v1only_permission(perm:'playbook_dispatcher_run_read');
@rbac.add_v1only_permission(perm:'playbook_dispatcher_run_write');
```

## Migration Strategy

### Phase 1: Implementation (Weeks 1-2)

1. **Add Kessel Dependencies**
   - `github.com/project-kessel/inventory-api`
   - `github.com/project-kessel/inventory-client-go`
   - `google.golang.org/grpc`

2. **Implement Kessel Client**
   - Create Kessel client abstraction
   - Implement workspace-based authorization checks
   - Add service-specific permission mapping

3. **Feature Flag Infrastructure**
   - Add configuration options for Kessel
   - Implement authorization system toggle
   - Support parallel operation mode

4. **Update Middleware**
   - Create new Kessel middleware
   - Implement authorization system selector
   - Maintain backward compatibility

### Phase 2: Testing (Weeks 3-4)

1. **Unit Testing**
   - Kessel client implementation
   - Permission mapping logic
   - Feature flag behavior

2. **Integration Testing**
   - End-to-end authorization flows
   - Service-level filtering
   - Error handling and fallback

3. **Performance Testing**
   - Compare RBAC vs Kessel latency
   - Load testing with both systems
   - Resource utilization analysis

### Phase 3: Deployment (Weeks 5-8)

1. **Stage Environment** (Week 5)
   - Deploy with Kessel disabled
   - Enable Kessel for internal testing
   - Monitor metrics and errors

2. **Canary Rollout** (Week 6)
   - Enable Kessel for 5% of traffic
   - Monitor error rates and latency
   - Gradually increase to 25%, 50%, 75%

3. **Production Rollout** (Week 7)
   - Enable Kessel for 100% of traffic
   - Monitor for 48 hours
   - Keep RBAC as fallback

4. **Cleanup** (Week 8+)
   - Remove RBAC client code (after stability period)
   - Remove feature flags
   - Update documentation

## Risk Assessment

### High Risk

1. **Permission Mapping Errors**
   - **Risk**: Incorrect service-to-permission mapping could grant wrong access
   - **Mitigation**: Comprehensive testing, parallel validation during rollout

2. **Kessel Service Availability**
   - **Risk**: Kessel unavailability could block all requests
   - **Mitigation**: Fallback to RBAC, circuit breaker pattern, aggressive timeouts

### Medium Risk

1. **Performance Degradation**
   - **Risk**: gRPC calls could be slower than HTTP REST
   - **Mitigation**: Performance testing, caching strategies, connection pooling

2. **Migration Complexity**
   - **Risk**: Managing two authorization systems increases complexity
   - **Mitigation**: Clear documentation, automated testing, gradual rollout

### Low Risk

1. **Configuration Drift**
   - **Risk**: Different configs across environments
   - **Mitigation**: Configuration management, automated deployment

## Success Criteria

1. **Functional**
   - All existing authorization scenarios work with Kessel
   - Service-level filtering maintains same behavior
   - No unauthorized access granted

2. **Performance**
   - P95 latency within 10% of current RBAC implementation
   - No increase in error rates
   - Kessel client connection pooling effective

3. **Operational**
   - Successful canary rollout with <0.1% error rate
   - Monitoring and alerting in place
   - Runbook for rollback documented

## References

- **PR #699**: https://github.com/RedHatInsights/rbac-config/pull/699/files
- **Config Manager Kessel Implementation**: `/home/jhollowa/dev/git/RedHatInsights/config-manager/internal/http/middleware/authorization/kessel.go`
- **Current RBAC Implementation**: `/home/jhollowa/dev/git/RedHatInsights/playbook-dispatcher/internal/api/middleware/rbac.go`
- **Kessel Documentation**: [Project Kessel](https://github.com/project-kessel)

## Next Steps

1. Review this migration plan with stakeholders
2. Confirm Kessel service availability in all environments
3. Review implementation guide (see `02-IMPLEMENTATION-GUIDE.md`)
4. Review configuration reference (see `03-CONFIGURATION-REFERENCE.md`)
5. Create implementation tickets and assign to team
