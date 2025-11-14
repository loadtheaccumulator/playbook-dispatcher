# Kessel Migration Documentation

## Overview

This directory contains comprehensive documentation for migrating playbook-dispatcher from RBAC to Kessel authorization.

## Document Index

### 1. [Migration Overview](01-MIGRATION-OVERVIEW.md)
**Start here for high-level understanding**

- Executive summary of the migration
- Background on RBAC and Kessel
- Migration goals and strategy
- Risk assessment
- Success criteria
- Timeline and phases

**Who should read**: All stakeholders, product managers, engineering leadership

---

### 2. [Implementation Guide](02-IMPLEMENTATION-GUIDE.md)
**Detailed technical implementation steps**

- Architecture diagrams
- Code structure and file organization
- Step-by-step implementation instructions
- Service permission mapping
- Testing strategy
- Deployment checklist

**Who should read**: Backend engineers, DevOps engineers

---

### 3. [Configuration Reference](03-CONFIGURATION-REFERENCE.md)
**Complete configuration documentation**

- Environment variable reference
- Feature flag states
- Environment-specific configurations
- Troubleshooting guide
- Rollback procedures

**Who should read**: DevOps engineers, SREs, backend engineers

---

### 4. [RBAC vs Kessel Comparison](04-RBAC-VS-KESSEL-COMPARISON.md)
**Deep technical comparison**

- Authorization model differences
- API comparison
- Performance analysis
- Code impact assessment
- Testing strategy comparison

**Who should read**: Backend engineers, architects, technical leads

---

## Quick Start

### For Engineers Implementing the Migration

1. Read [01-MIGRATION-OVERVIEW.md](01-MIGRATION-OVERVIEW.md) for context
2. Review [04-RBAC-VS-KESSEL-COMPARISON.md](04-RBAC-VS-KESSEL-COMPARISON.md) to understand differences
3. Follow [02-IMPLEMENTATION-GUIDE.md](02-IMPLEMENTATION-GUIDE.md) step-by-step
4. Use [03-CONFIGURATION-REFERENCE.md](03-CONFIGURATION-REFERENCE.md) for configuration

### For DevOps/SREs Deploying the Changes

1. Skim [01-MIGRATION-OVERVIEW.md](01-MIGRATION-OVERVIEW.md) for background
2. Focus on [03-CONFIGURATION-REFERENCE.md](03-CONFIGURATION-REFERENCE.md)
3. Review deployment checklist in [02-IMPLEMENTATION-GUIDE.md](02-IMPLEMENTATION-GUIDE.md#deployment-checklist)
4. Keep rollback procedure handy

### For Product/Engineering Leadership

1. Read [01-MIGRATION-OVERVIEW.md](01-MIGRATION-OVERVIEW.md) completely
2. Review risk assessment and success criteria
3. Approve timeline and migration phases

---

## Key Concepts

### RBAC (Current State)

- **Protocol**: HTTP REST API
- **Permission**: `playbook-dispatcher:run:read`
- **Filtering**: Attribute-based (service field)
- **Services**: remediations, tasks, config_manager

### Kessel (Target State)

- **Protocol**: gRPC
- **Permissions**: Service-specific
  - `playbook_dispatcher_remediations_run_view`
  - `playbook_dispatcher_tasks_run_view`
  - `playbook_dispatcher_config_manager_run_view`
- **Authorization**: Workspace-based
- **Model**: Relationship-based (ReBAC)

### Feature Flags

The migration uses configuration-based feature flags:

- `AUTH_SYSTEM=rbac` - Current RBAC only
- `AUTH_SYSTEM=both` - Run both systems, RBAC enforces (Kessel logs only)
- `AUTH_SYSTEM=kessel` - Kessel only (target)

---

## Migration Phases

### Phase 1: Implementation (Weeks 1-2)
- Add Kessel dependencies
- Implement Kessel client and middleware
- Add feature flag infrastructure
- Write comprehensive tests

### Phase 2: Testing (Weeks 3-4)
- Unit testing
- Integration testing
- Performance testing
- Security testing

### Phase 3: Deployment (Weeks 5-8)
- Stage deployment with validation
- Canary rollout in production
- Full production rollout
- Monitoring and stabilization

---

## Critical Files

### New Files to Create

```
internal/api/kessel/
├── client.go           # Kessel gRPC client
├── permissions.go      # Service-to-permission mapping
├── rbac.go            # Workspace lookup client
├── mock.go            # Mock client for testing
└── types.go           # Type definitions

internal/api/middleware/
├── kessel.go          # Kessel authorization middleware
└── authselector.go    # Feature flag-based auth selector
```

### Files to Modify

```
internal/common/config/config.go        # Add Kessel config
internal/api/main.go                     # Use auth selector
internal/api/controllers/public/*.go     # Add Kessel filtering
internal/api/instrumentation/probes.go   # Add Kessel metrics
deploy/clowdapp.yaml                     # Add env vars
go.mod                                   # Add dependencies
```

---

## Configuration Quick Reference

### Local Development
```bash
AUTH_SYSTEM=rbac
KESSEL_ENABLED=false
RBAC_IMPL=mock
```

### Stage - Validation
```bash
AUTH_SYSTEM=both
KESSEL_ENABLED=true
KESSEL_URL=kessel-inventory-api.stage.svc.cluster.local:9091
KESSEL_AUTH_ENABLED=true
RBAC_IMPL=impl
```

### Production - Target
```bash
AUTH_SYSTEM=kessel
KESSEL_ENABLED=true
KESSEL_URL=kessel-inventory-api.prod.svc.cluster.local:9091
KESSEL_AUTH_ENABLED=true
KESSEL_INSECURE=false
RBAC_IMPL=impl  # Still needed for workspace lookup
```

---

## Troubleshooting Quick Reference

### All requests return 403

1. Check Kessel service is running
2. Verify `KESSEL_URL` is correct
3. Check authentication credentials
4. Verify permissions configured in Kessel

**Quick rollback**:
```bash
kubectl set env deployment/playbook-dispatcher AUTH_SYSTEM=rbac
```

### High latency

1. Check `KESSEL_TIMEOUT` (increase if needed)
2. Verify gRPC connection pooling
3. Implement workspace caching
4. Check Kessel service health

### Different results between RBAC and Kessel

1. Enable debug logging: `LOG_LEVEL=debug`
2. Compare permission mappings
3. Check service name normalization
4. Verify Kessel schema deployed

---

## Dependencies

### Required Services

- **Kessel Inventory API**: gRPC authorization service
- **RBAC Service**: For workspace lookup and fallback

### Required Libraries

```
github.com/project-kessel/inventory-api
github.com/project-kessel/inventory-client-go
google.golang.org/grpc
```

---

## Metrics to Monitor

### Kessel Metrics
```
api_kessel_error_total
api_kessel_rejected_total
api_kessel_check_duration_seconds
api_kessel_workspace_lookup_duration_seconds
```

### Existing RBAC Metrics
```
api_rbac_error_total
api_rbac_rejected_total
```

### Application Metrics
```
http_request_duration_seconds
http_requests_total
```

---

## Testing Checklist

- [ ] Unit tests for Kessel client
- [ ] Unit tests for permission mapping
- [ ] Unit tests for middleware
- [ ] Integration tests for authorization flow
- [ ] Integration tests for service filtering
- [ ] Performance tests comparing RBAC vs Kessel
- [ ] Security tests for auth bypass attempts
- [ ] Manual testing with real users/roles
- [ ] Comparison testing with `AUTH_SYSTEM=both`

---

## Rollback Plan

### Immediate Rollback (< 5 minutes)

```bash
# Option 1: Environment variable
kubectl set env deployment/playbook-dispatcher AUTH_SYSTEM=rbac

# Option 2: Deployment rollback
kubectl rollout undo deployment/playbook-dispatcher
```

### Verification

```bash
# Check logs
kubectl logs -f deployment/playbook-dispatcher | grep "auth.*system"

# Check metrics
curl http://playbook-dispatcher:9000/metrics | grep -E "(rbac|kessel)"

# Test API
curl -H "x-rh-identity: $(echo '...' | base64)" \
     http://playbook-dispatcher:8000/api/playbook-dispatcher/v1/runs
```

---

## Success Criteria

### Functional
- ✅ All authorization scenarios work with Kessel
- ✅ Service-level filtering identical to RBAC
- ✅ No unauthorized access granted
- ✅ All existing tests pass

### Performance
- ✅ P95 latency within 10% of RBAC
- ✅ No increase in error rates
- ✅ Kessel client connection pooling effective

### Operational
- ✅ Successful canary rollout
- ✅ Error rate < 0.1%
- ✅ Monitoring and alerting in place
- ✅ Runbook for rollback documented

---

## Key Contacts

| Role | Responsibility |
|------|---------------|
| **Backend Engineers** | Implementation, testing |
| **DevOps/SRE** | Deployment, monitoring |
| **Security Team** | Security review, OIDC credentials |
| **Platform Team** | Kessel service support |
| **Product Owner** | Migration approval, timeline |

---

## References

### Related PRs
- [rbac-config PR #699](https://github.com/RedHatInsights/rbac-config/pull/699/files) - Kessel schema for playbook-dispatcher

### Related Codebases
- [config-manager](https://github.com/RedHatInsights/config-manager) - Reference Kessel implementation
- [rbac-config](https://github.com/RedHatInsights/rbac-config) - Permission and role definitions

### External Documentation
- [Project Kessel](https://github.com/project-kessel)
- [Kessel Inventory API](https://github.com/project-kessel/inventory-api)
- [SpiceDB Documentation](https://authzed.com/docs)

---

## Document Maintenance

### Last Updated
2025-11-13

### Document Owner
Playbook Dispatcher Team

### Review Schedule
- Review after each migration phase
- Update with lessons learned
- Archive after migration complete

---

## FAQ

### Q: Will this affect API clients?
**A**: No, authorization is server-side only. API endpoints, request/response formats, and status codes remain unchanged.

### Q: Can we rollback if something goes wrong?
**A**: Yes, easily. Change `AUTH_SYSTEM=rbac` and restart pods (~2 minutes).

### Q: Do we need to migrate the database?
**A**: No, database schema remains unchanged.

### Q: What happens if Kessel is unavailable?
**A**: Requests will fail with 500 error. During migration, you can configure fallback to RBAC.

### Q: How long will the migration take?
**A**: Estimated 8 weeks total (2 weeks implementation, 2 weeks testing, 4 weeks phased deployment).

### Q: Will this impact performance?
**A**: Minimal impact. Kessel may add 5-15ms initially, but caching and connection pooling will optimize this.

### Q: Do we maintain two code paths permanently?
**A**: No, RBAC code will be removed after successful migration (after ~2-4 week stability period).

---

## Next Steps

1. **Engineering Team**: Review all documents and raise questions
2. **Security Team**: Review authentication and authorization approach
3. **DevOps**: Prepare environments (Kessel service endpoints, credentials)
4. **Product Owner**: Approve migration timeline
5. **All**: Kick off implementation following [02-IMPLEMENTATION-GUIDE.md](02-IMPLEMENTATION-GUIDE.md)
