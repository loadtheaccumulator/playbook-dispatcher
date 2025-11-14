# Kessel Configuration Reference

## Table of Contents

1. [Environment Variables](#environment-variables)
2. [Configuration File](#configuration-file)
3. [Feature Flag States](#feature-flag-states)
4. [Environment-Specific Configurations](#environment-specific-configurations)
5. [Troubleshooting](#troubleshooting)

## Environment Variables

### Core Kessel Configuration

#### `KESSEL_ENABLED`
- **Type**: Boolean
- **Default**: `false`
- **Values**: `true`, `false`
- **Description**: Master switch to enable/disable Kessel authorization checks
- **Usage**: Set to `true` only when `AUTH_SYSTEM=kessel` or `AUTH_SYSTEM=both`

```bash
KESSEL_ENABLED=true
```

#### `KESSEL_URL`
- **Type**: String
- **Default**: `localhost:9091`
- **Format**: `hostname:port`
- **Description**: Kessel inventory service gRPC endpoint
- **Required**: When `KESSEL_ENABLED=true`

```bash
# Local development
KESSEL_URL=localhost:9091

# Stage environment
KESSEL_URL=kessel-inventory-api.stage.svc.cluster.local:9091

# Production environment
KESSEL_URL=kessel-inventory-api.prod.svc.cluster.local:9091
```

#### `KESSEL_TIMEOUT`
- **Type**: Integer
- **Default**: `10`
- **Unit**: Seconds
- **Description**: Timeout for Kessel gRPC calls and RBAC workspace lookup
- **Recommended**: 5-15 seconds depending on network latency

```bash
KESSEL_TIMEOUT=10
```

#### `KESSEL_INSECURE`
- **Type**: Boolean
- **Default**: `true`
- **Values**: `true`, `false`
- **Description**: Disable TLS verification for Kessel gRPC connection
- **Security**: Set to `false` in production environments

```bash
# Development/local
KESSEL_INSECURE=true

# Production
KESSEL_INSECURE=false
```

### Kessel Authentication Configuration

#### `KESSEL_AUTH_ENABLED`
- **Type**: Boolean
- **Default**: `false`
- **Values**: `true`, `false`
- **Description**: Enable OIDC authentication for Kessel client
- **Required**: `true` for production environments

```bash
KESSEL_AUTH_ENABLED=true
```

#### `KESSEL_AUTH_CLIENT_ID`
- **Type**: String
- **Default**: `""`
- **Description**: OIDC client ID for Kessel authentication
- **Required**: When `KESSEL_AUTH_ENABLED=true`
- **Source**: Provided by platform team

```bash
KESSEL_AUTH_CLIENT_ID=playbook-dispatcher-prod
```

#### `KESSEL_AUTH_CLIENT_SECRET`
- **Type**: String (Secret)
- **Default**: `""`
- **Description**: OIDC client secret for Kessel authentication
- **Required**: When `KESSEL_AUTH_ENABLED=true`
- **Security**: Store in Kubernetes secrets, never in code
- **Source**: Provided by platform team

```bash
# In Kubernetes secret
KESSEL_AUTH_CLIENT_SECRET=<secret-value>
```

#### `KESSEL_AUTH_OIDC_ISSUER`
- **Type**: String (URL)
- **Default**: `https://sso.redhat.com/auth/realms/redhat-external/protocol/openid-connect/token`
- **Description**: OIDC token issuer endpoint
- **Format**: Full OIDC token endpoint URL

```bash
KESSEL_AUTH_OIDC_ISSUER=https://sso.redhat.com/auth/realms/redhat-external/protocol/openid-connect/token
```

### Authorization System Selection

#### `AUTH_SYSTEM`
- **Type**: String (Enum)
- **Default**: `rbac`
- **Values**: `rbac`, `kessel`, `both`
- **Description**: Selects which authorization system to use
- **Migration Path**: `rbac` → `both` → `kessel`

```bash
# Current state: RBAC only
AUTH_SYSTEM=rbac

# Comparison mode: both systems run, RBAC enforces (Kessel logs only)
AUTH_SYSTEM=both

# Target state: Kessel only
AUTH_SYSTEM=kessel
```

**Behavior by Value**:

| Value | RBAC Check | Kessel Check | Enforcement | Use Case |
|-------|-----------|--------------|-------------|----------|
| `rbac` | ✅ Yes | ❌ No | RBAC | Current production state |
| `both` | ✅ Yes | ✅ Yes (log only) | RBAC | Migration validation |
| `kessel` | ❌ No | ✅ Yes | Kessel | Target production state |

### Existing RBAC Configuration

These settings continue to be used for workspace lookup and during migration.

#### `RBAC_IMPL`
- **Type**: String (Enum)
- **Default**: `mock`
- **Values**: `impl`, `mock`
- **Description**: Toggle between real and mock RBAC client
- **Usage**: Set to `impl` in deployed environments

```bash
# Production
RBAC_IMPL=impl

# Local development
RBAC_IMPL=mock
```

#### `RBAC_HOST`
- **Type**: String
- **Default**: `rbac`
- **Description**: RBAC service hostname
- **Required**: When `RBAC_IMPL=impl` or for Kessel workspace lookup

```bash
# Stage
RBAC_HOST=rbac-service.stage.svc.cluster.local

# Production
RBAC_HOST=rbac-service.prod.svc.cluster.local
```

#### `RBAC_PORT`
- **Type**: Integer
- **Default**: `8080`
- **Description**: RBAC service port

```bash
RBAC_PORT=8080
```

#### `RBAC_SCHEME`
- **Type**: String (Enum)
- **Default**: `http`
- **Values**: `http`, `https`
- **Description**: HTTP scheme for RBAC service

```bash
RBAC_SCHEME=http
```

#### `RBAC_TIMEOUT`
- **Type**: Integer
- **Default**: `10`
- **Unit**: Seconds
- **Description**: Timeout for RBAC HTTP requests

```bash
RBAC_TIMEOUT=10
```

## Configuration File

### Viper Configuration Structure

If using a configuration file (YAML/JSON), the structure matches environment variables:

**config.yaml** (example):

```yaml
kessel:
  enabled: false
  url: "localhost:9091"
  timeout: 10
  insecure: true
  auth:
    enabled: false
    client_id: ""
    client_secret: ""
    oidc_issuer: "https://sso.redhat.com/auth/realms/redhat-external/protocol/openid-connect/token"

auth:
  system: "rbac"  # rbac, kessel, or both

rbac:
  impl: "mock"
  host: "rbac"
  port: 8080
  scheme: "http"
  timeout: 10
```

### Configuration Precedence

Configuration values are resolved in this order (highest to lowest priority):

1. Environment variables (e.g., `KESSEL_ENABLED`)
2. Configuration file values
3. Default values in code

## Feature Flag States

### State 1: RBAC Only (Current)

**Use Case**: Current production state, no changes

```bash
AUTH_SYSTEM=rbac
KESSEL_ENABLED=false
RBAC_IMPL=impl
```

**Behavior**:
- RBAC permission checks enforced
- No Kessel calls made
- Service filtering via RBAC resource definitions

---

### State 2: Both Systems (Migration Validation)

**Use Case**: Validate Kessel behaves identically to RBAC

```bash
AUTH_SYSTEM=both
KESSEL_ENABLED=true
KESSEL_URL=kessel-inventory-api.stage.svc.cluster.local:9091
KESSEL_AUTH_ENABLED=true
KESSEL_AUTH_CLIENT_ID=playbook-dispatcher-stage
# ... other Kessel config ...
RBAC_IMPL=impl
```

**Behavior**:
- Both RBAC and Kessel checks run
- RBAC enforces (blocks unauthorized requests) - production system remains authoritative
- Kessel runs but only logs results
- Logs show comparison of both systems
- Use for validation before full switch

**Log Output Example**:
```
INFO RBAC granted remediations permission to user-123
INFO Kessel would have allowed
```

---

### State 3: Kessel Only (Target)

**Use Case**: Final production state after migration

```bash
AUTH_SYSTEM=kessel
KESSEL_ENABLED=true
KESSEL_URL=kessel-inventory-api.prod.svc.cluster.local:9091
KESSEL_AUTH_ENABLED=true
KESSEL_AUTH_CLIENT_ID=playbook-dispatcher-prod
KESSEL_INSECURE=false
# ... other Kessel config ...
RBAC_IMPL=impl  # Still needed for workspace lookup
```

**Behavior**:
- Only Kessel checks run
- RBAC client only used for workspace lookup
- Service filtering via Kessel permissions
- RBAC permission check code not executed

---

### State 4: Mock/Development

**Use Case**: Local development without external dependencies

```bash
AUTH_SYSTEM=rbac
KESSEL_ENABLED=false
RBAC_IMPL=mock
```

**Behavior**:
- Mock RBAC client returns all permissions
- No external service calls
- All requests allowed for testing

## Environment-Specific Configurations

### Local Development

```bash
# Minimal config for local development
AUTH_SYSTEM=rbac
KESSEL_ENABLED=false
RBAC_IMPL=mock
```

### CI/CD Testing

```bash
# Use mocks for fast tests
AUTH_SYSTEM=kessel
KESSEL_ENABLED=true
KESSEL_IMPL=mock  # If mock implemented
RBAC_IMPL=mock
```

### Stage Environment (Pre-Migration)

```bash
# Stage with RBAC only
AUTH_SYSTEM=rbac
KESSEL_ENABLED=false
RBAC_IMPL=impl
RBAC_HOST=rbac-service.stage.svc.cluster.local
RBAC_PORT=8080
RBAC_SCHEME=http
```

### Stage Environment (Validation Phase)

```bash
# Stage with both systems for validation
AUTH_SYSTEM=both
KESSEL_ENABLED=true
KESSEL_URL=kessel-inventory-api.stage.svc.cluster.local:9091
KESSEL_AUTH_ENABLED=true
KESSEL_AUTH_CLIENT_ID=playbook-dispatcher-stage
KESSEL_AUTH_CLIENT_SECRET=<from-secret>
KESSEL_INSECURE=false
KESSEL_TIMEOUT=10
RBAC_IMPL=impl
RBAC_HOST=rbac-service.stage.svc.cluster.local
```

### Stage Environment (Post-Migration)

```bash
# Stage with Kessel only
AUTH_SYSTEM=kessel
KESSEL_ENABLED=true
KESSEL_URL=kessel-inventory-api.stage.svc.cluster.local:9091
KESSEL_AUTH_ENABLED=true
KESSEL_AUTH_CLIENT_ID=playbook-dispatcher-stage
KESSEL_AUTH_CLIENT_SECRET=<from-secret>
KESSEL_INSECURE=false
KESSEL_TIMEOUT=10
RBAC_IMPL=impl  # Still needed for workspace lookup
RBAC_HOST=rbac-service.stage.svc.cluster.local
```

### Production Environment (Pre-Migration)

```bash
# Production with RBAC only
AUTH_SYSTEM=rbac
KESSEL_ENABLED=false
RBAC_IMPL=impl
RBAC_HOST=rbac-service.prod.svc.cluster.local
RBAC_PORT=8080
RBAC_SCHEME=http
RBAC_TIMEOUT=10
```

### Production Environment (Canary Phase)

```bash
# Canary deployment with Kessel
# Deploy to subset of pods
AUTH_SYSTEM=kessel
KESSEL_ENABLED=true
KESSEL_URL=kessel-inventory-api.prod.svc.cluster.local:9091
KESSEL_AUTH_ENABLED=true
KESSEL_AUTH_CLIENT_ID=playbook-dispatcher-prod
KESSEL_AUTH_CLIENT_SECRET=<from-secret>
KESSEL_INSECURE=false
KESSEL_TIMEOUT=10
RBAC_IMPL=impl
RBAC_HOST=rbac-service.prod.svc.cluster.local
```

### Production Environment (Post-Migration)

```bash
# Full production with Kessel
AUTH_SYSTEM=kessel
KESSEL_ENABLED=true
KESSEL_URL=kessel-inventory-api.prod.svc.cluster.local:9091
KESSEL_AUTH_ENABLED=true
KESSEL_AUTH_CLIENT_ID=playbook-dispatcher-prod
KESSEL_AUTH_CLIENT_SECRET=<from-secret>
KESSEL_INSECURE=false
KESSEL_TIMEOUT=10
RBAC_IMPL=impl  # Still needed for workspace lookup
RBAC_HOST=rbac-service.prod.svc.cluster.local
```

## Troubleshooting

### Issue: All requests return 403 Forbidden

**Possible Causes**:

1. **Kessel service unavailable**
   - Check: `KESSEL_URL` is correct
   - Check: Network connectivity to Kessel service
   - Check: Kessel service is running
   - Logs: "Error performing authorization check"

2. **Authentication failure**
   - Check: `KESSEL_AUTH_ENABLED=true` with valid credentials
   - Check: `KESSEL_AUTH_CLIENT_ID` and `KESSEL_AUTH_CLIENT_SECRET`
   - Check: Token endpoint is accessible
   - Logs: "failed to get auth token"

3. **Workspace lookup failure**
   - Check: `RBAC_HOST` is correct
   - Check: RBAC service is accessible
   - Logs: "Workspace lookup failed"

4. **Permissions not configured**
   - Check: Kessel schema deployed to environment
   - Check: User/ServiceAccount has appropriate roles
   - Logs: "Kessel denied * permission"

### Issue: Requests timing out

**Possible Causes**:

1. **Timeout too low**
   - Increase: `KESSEL_TIMEOUT=15`
   - Increase: `RBAC_TIMEOUT=15`

2. **Network latency**
   - Check: Kessel service latency metrics
   - Check: RBAC service latency metrics
   - Consider: Connection pooling, caching

### Issue: "Unsupported identity type" errors

**Possible Causes**:

1. **Wrong identity type in request**
   - Supported: `User`, `ServiceAccount`
   - Not supported: `System`, `Associate`
   - Check: `x-rh-identity` header content

### Issue: Metrics show high error rates

**Metrics to Check**:

```
api_kessel_error_total             # Kessel client errors
api_kessel_rejected_total          # Authorization denials
api_rbac_error_total               # RBAC client errors (if still used)
api_kessel_check_duration_seconds  # Kessel check latency
```

**Actions**:

1. Check Kessel service health
2. Check RBAC service health
3. Review logs for specific error messages
4. Consider rollback to `AUTH_SYSTEM=rbac`

### Issue: Different results between RBAC and Kessel in "both" mode

**Debugging Steps**:

1. Enable debug logging:
   ```bash
   LOG_LEVEL=debug
   ```

2. Compare logs:
   ```
   grep "Kessel granted" /var/log/playbook-dispatcher.log
   grep "RBAC would have allowed" /var/log/playbook-dispatcher.log
   ```

3. Check permission mapping:
   - Verify service name format (underscore vs hyphen)
   - Verify permission mapping is correct
   - Check Kessel schema deployment

4. Test with specific user/org:
   ```bash
   # Query Kessel directly
   grpcurl -d '{"object":{"resource_type":"workspace",...}}' \
     kessel-inventory-api:9091 \
     kessel.inventory.v1beta2.KesselInventoryService/Check
   ```

### Configuration Validation Script

```bash
#!/bin/bash
# validate-kessel-config.sh

echo "Validating Kessel configuration..."

# Check required variables
if [ "$AUTH_SYSTEM" = "kessel" ] || [ "$AUTH_SYSTEM" = "both" ]; then
    if [ "$KESSEL_ENABLED" != "true" ]; then
        echo "ERROR: AUTH_SYSTEM=$AUTH_SYSTEM requires KESSEL_ENABLED=true"
        exit 1
    fi

    if [ -z "$KESSEL_URL" ]; then
        echo "ERROR: KESSEL_URL is required when Kessel is enabled"
        exit 1
    fi

    if [ "$KESSEL_AUTH_ENABLED" = "true" ]; then
        if [ -z "$KESSEL_AUTH_CLIENT_ID" ]; then
            echo "ERROR: KESSEL_AUTH_CLIENT_ID required when auth enabled"
            exit 1
        fi
        if [ -z "$KESSEL_AUTH_CLIENT_SECRET" ]; then
            echo "ERROR: KESSEL_AUTH_CLIENT_SECRET required when auth enabled"
            exit 1
        fi
    fi
fi

# Warn about insecure settings in production
if [ "$ENVIRONMENT" = "production" ] && [ "$KESSEL_INSECURE" = "true" ]; then
    echo "WARNING: KESSEL_INSECURE=true in production is not recommended"
fi

echo "Configuration validation passed"
```

### Rollback Procedure

If issues are encountered after enabling Kessel:

1. **Immediate Rollback** (< 5 minutes):
   ```bash
   # Update environment variable
   kubectl set env deployment/playbook-dispatcher AUTH_SYSTEM=rbac

   # Or update ConfigMap and restart
   kubectl edit configmap playbook-dispatcher-config
   kubectl rollout restart deployment/playbook-dispatcher
   ```

2. **Verify Rollback**:
   ```bash
   # Check logs
   kubectl logs -f deployment/playbook-dispatcher | grep "auth.*system"

   # Check metrics
   curl http://playbook-dispatcher:9000/metrics | grep rbac
   ```

3. **Post-Rollback**:
   - Document issue in incident report
   - Analyze logs for root cause
   - Fix issue in lower environment
   - Re-test before attempting rollout again
