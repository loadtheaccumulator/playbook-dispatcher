# Kessel Service Filtering Configuration

## Overview

Playbook-dispatcher uses **service-scoped permissions** to restrict which playbook runs users can see based on the service that created them (e.g., `remediations`, `config-manager`, `vulnerability`).

## Current RBAC Implementation

### RBAC Permission Format
```json
{
  "permission": "playbook-dispatcher:run:read",
  "resourceDefinitions": [
    {
      "attributeFilter": {
        "key": "service",
        "operation": "in",
        "value": ["remediations", "config-manager"]
      }
    }
  ]
}
```

### How It Works
1. User requests list of runs
2. RBAC returns permissions with `resourceDefinitions`
3. Code extracts `service` attribute values using `GetPredicateValues()`
4. SQL query filters: `WHERE service IN ('remediations', 'config-manager')`

See: `internal/api/controllers/public/runsList.go:60-63`

## Kessel Implementation Options

Kessel doesn't have direct equivalents to RBAC's `resourceDefinitions`, but supports attribute-based filtering through several mechanisms:

### Option 1: Service-Specific Resource Types (Recommended)

Define separate resource types per service:

```yaml
resourceTypes:
  - name: playbook-dispatcher/run/remediations
    relations:
      - name: read
      - name: write

  - name: playbook-dispatcher/run/config-manager
    relations:
      - name: read
      - name: write
```

**Relationship tuples:**
```
user:jdoe@redhat.com#member@playbook-dispatcher/org:org-123
playbook-dispatcher/run/remediations:*#viewer@playbook-dispatcher/org:org-123
playbook-dispatcher/run/config-manager:*#viewer@playbook-dispatcher/org:org-123
```

**Pros:**
- Clean, explicit permissions
- Standard Kessel patterns

**Cons:**
- Requires multiple Kessel checks per service
- Need to update schema when new services are added

### Option 2: Relationship Metadata (Future)

Use Kessel's relationship metadata/attributes feature (when available):

```
Tuple: (user:jdoe, read, playbook-dispatcher/run:*)
Metadata: { "services": ["remediations", "config-manager"] }
```

**Pros:**
- Flexible, dynamic
- Single check per user

**Cons:**
- Not yet widely supported in Kessel
- Requires custom filtering logic

### Option 3: Hierarchical Resources

Create a hierarchy with service as parent:

```yaml
resourceTypes:
  - name: playbook-dispatcher/service
    relations:
      - name: admin
      - name: viewer

  - name: playbook-dispatcher/run
    relations:
      - name: read
        # Inherits from parent service
      - name: service
        # Parent relationship
```

**Relationship tuples:**
```
user:jdoe@redhat.com#viewer@playbook-dispatcher/service:remediations
playbook-dispatcher/run:123#service@playbook-dispatcher/service:remediations
```

**Pros:**
- Natural hierarchy
- Kessel can handle transitive relationships

**Cons:**
- Complex to query
- Need to maintain run-to-service mappings in Kessel

### Option 4: Policy-Based Filtering (CEL)

Use Kessel's Common Expression Language (CEL) support for conditions:

```yaml
policies:
  - name: service-filter
    condition: |
      resource.service in subject.allowedServices
```

**Pros:**
- Most flexible
- Supports complex conditions

**Cons:**
- Requires Kessel policy engine
- May have performance implications

## Recommended Approach

**Use Option 1 (Service-Specific Resource Types) with a hybrid approach:**

### 1. Define Resource Schema

```yaml
resourceTypes:
  - name: playbook-dispatcher/org
    relations:
      - name: admin
      - name: member

  - name: playbook-dispatcher/service-permission
    relations:
      - name: reader
        # Can read runs from this service

  - name: playbook-dispatcher/run
    relations:
      - name: read
      - name: write
```

### 2. Create Service Permission Tuples

When granting access to a service:
```
user:jdoe@redhat.com#reader@playbook-dispatcher/service-permission:remediations
user:jdoe@redhat.com#reader@playbook-dispatcher/service-permission:config-manager
```

### 3. Query Pattern

**Controller logic:**
```go
subject, _ := kessel.SubjectFromContext(ctx)

// Get allowed services
allowedServices, err := getServicesForSubject(ctx, kesselClient, subject)

// Apply to SQL query
if len(allowedServices) > 0 {
    queryBuilder.Where("service IN ?", allowedServices)
} else {
    // Check if user has org-level access (no service restriction)
    hasOrgAccess, _ := kesselClient.Check(ctx, kessel.ResourceCheck{
        Subject: subject,
        Relation: kessel.RelationRead,
        Resource: kessel.Resource{
            Type: kessel.ResourceTypeOrg,
            ID: subject.Tenant,
        },
    })

    if !hasOrgAccess {
        // No access at all
        return forbidden
    }
    // User has org-level access, no service filtering needed
}
```

**Helper function:**
```go
func getServicesForSubject(ctx context.Context, client kessel.KesselClient, subject kessel.Subject) ([]string, error) {
    // Known services in playbook-dispatcher
    knownServices := []string{"remediations", "config-manager", "vulnerability", "advisor", "compliance"}

    allowedServices := []string{}

    // Check each service
    for _, service := range knownServices {
        check := kessel.ResourceCheck{
            Subject: subject,
            Relation: "reader",
            Resource: kessel.Resource{
                Type: "playbook-dispatcher/service-permission",
                ID: service,
                Tenant: subject.Tenant,
            },
        }

        allowed, err := client.Check(ctx, check)
        if err != nil {
            return nil, err
        }

        if allowed {
            allowedServices = append(allowedServices, service)
        }
    }

    return allowedServices, nil
}
```

### 4. Optimization: Batch Checks

Instead of individual checks, use batch:

```go
func getServicesForSubjectBatch(ctx context.Context, client kessel.KesselClient, subject kessel.Subject) ([]string, error) {
    knownServices := []string{"remediations", "config-manager", "vulnerability", "advisor", "compliance"}

    checks := make([]kessel.ResourceCheck, len(knownServices))
    for i, service := range knownServices {
        checks[i] = kessel.ResourceCheck{
            Subject: subject,
            Relation: "reader",
            Resource: kessel.Resource{
                Type: "playbook-dispatcher/service-permission",
                ID: service,
                Tenant: subject.Tenant,
            },
        }
    }

    results, err := client.CheckBatch(ctx, checks)
    if err != nil {
        return nil, err
    }

    allowedServices := []string{}
    for i, allowed := range results {
        if allowed {
            allowedServices = append(allowedServices, knownServices[i])
        }
    }

    return allowedServices, nil
}
```

## Migration Steps

### 1. Define Kessel Schema

Create the service-permission resource type in Kessel.

### 2. Migrate Existing Permissions

For each user with RBAC permissions:
```bash
# If user has service filter in RBAC:
# permission: playbook-dispatcher:run:read
# resourceDefinitions.attributeFilter: {"key": "service", "value": ["remediations"]}

# Create Kessel tuple:
kessel create-relationship \
  --subject "user:jdoe@redhat.com" \
  --relation "reader" \
  --resource "playbook-dispatcher/service-permission:remediations"
```

### 3. Update Controller Code

Add service filtering logic to `runsList.go` and `runHostsList.go`.

### 4. Test

Verify that service filtering works correctly with Kessel.

## Alternative: No Service Filtering in Kessel

If service filtering is not critical for authorization (only for convenience), you could:

1. **Grant org-level permissions in Kessel** (no service filtering)
2. **Filter services in application logic** based on user preferences or other metadata
3. **Store service preferences** in a separate service (not Kessel)

This simplifies Kessel configuration but loses the security benefit of service-scoped authorization.

## Performance Considerations

- **Batch checks** for multiple services (single gRPC call)
- **Cache** service permissions per request
- **Monitor** Kessel latency for batch checks
- Consider **fallback** to no filtering if Kessel is slow/unavailable

## Security Notes

Service filtering is **authorization**, not just UI filtering. Users should not be able to bypass service restrictions by manipulating API calls.

Ensure:
1. Service filtering is enforced in authorization layer (middleware or controller)
2. Cannot be bypassed via direct database queries
3. Audit logs capture service-filtered access attempts
