# Kessel Authorization Package

This package provides Kessel-based authorization for playbook-dispatcher as a replacement for the legacy RBAC system.

## Overview

Kessel is Red Hat's authorization service based on Google Zanzibar. It provides fine-grained, relationship-based access control using gRPC for high performance.

## Package Structure

```
kessel/
├── types.go      - Core types (Subject, Resource, ResourceCheck, etc.)
├── client.go     - Kessel gRPC client implementation
├── mock.go       - Mock client for testing and development
├── config.go     - Configuration helpers
├── utils.go      - Helper functions for common operations
└── README.md     - This file
```

## Core Concepts

### Subject
Represents who is requesting access (user, service account, etc.)

```go
subject := kessel.Subject{
    Type:   kessel.SubjectTypeUser,
    ID:     "username@redhat.com",
    Tenant: "org-id-123",
}
```

### Resource
Represents what is being accessed

```go
resource := kessel.Resource{
    Type:   kessel.ResourceTypeRun,
    ID:     "run-uuid-123",
    Tenant: "org-id-123",
}
```

### Relation
The permission being checked (e.g., "read", "write", "execute")

```go
relation := kessel.RelationRead
```

### ResourceCheck
Combines subject, relation, and resource for authorization check

```go
check := kessel.ResourceCheck{
    Subject:  subject,
    Relation: kessel.RelationRead,
    Resource: resource,
}

allowed, err := client.Check(ctx, check)
```

## Client Usage

### Initialize Client

```go
import "playbook-dispatcher/internal/api/kessel"

// From configuration
client, err := kessel.NewKesselClient(cfg)
if err != nil {
    log.Fatal(err)
}
defer client.Close()

// Or use helper
client, err := kessel.NewKesselClientFromConfig(cfg)
```

### Single Check

```go
allowed, err := client.Check(ctx, check)
if err != nil {
    // Handle error
}

if !allowed {
    // Deny access
}
```

### Batch Checks

```go
checks := []kessel.ResourceCheck{
    kessel.DispatcherRunCheck(subject, kessel.RelationRead, "run-1"),
    kessel.DispatcherRunCheck(subject, kessel.RelationRead, "run-2"),
    kessel.DispatcherRunCheck(subject, kessel.RelationRead, "run-3"),
}

results, err := client.CheckBatch(ctx, checks)
// results[i] corresponds to checks[i]
```

## Helper Functions

### Create Subject from Context

```go
subject, err := kessel.SubjectFromContext(ctx)
```

### Check Run Access

```go
allowed, err := kessel.CheckRunAccess(ctx, client, subject, "run-123", kessel.RelationRead)
```

### Filter Authorized Resources

```go
runIDs := []string{"run-1", "run-2", "run-3", "run-4"}

authorizedIDs, err := kessel.FilterAuthorizedResources(
    ctx,
    client,
    subject,
    kessel.RelationRead,
    kessel.ResourceTypeRun,
    runIDs,
)
// Returns only IDs the subject can read
```

## Resource Types

Defined resource types for playbook-dispatcher:

- `kessel.ResourceTypeRun` - `"playbook-dispatcher/run"`
- `kessel.ResourceTypeRunHost` - `"playbook-dispatcher/run_host"`
- `kessel.ResourceTypeOrg` - `"playbook-dispatcher/org"`

## Relations (Permissions)

Standard relations:

- `kessel.RelationRead` - View/read access
- `kessel.RelationWrite` - Create/update access
- `kessel.RelationExecute` - Execute/run access
- `kessel.RelationDelete` - Delete access
- `kessel.RelationCancel` - Cancel operation access

## Subject Types

- `kessel.SubjectTypeUser` - Human user
- `kessel.SubjectTypeServiceAccount` - Service/system account

## Mock Client

For testing and development:

```go
// Allow all checks
client := kessel.NewMockKesselClient(true)

// Deny all checks
client := kessel.NewMockKesselClient(false)
```

Configure via environment:
```bash
export KESSEL_IMPL=mock
```

## Configuration

Required configuration keys:

```bash
KESSEL_ENABLED=true|false       # Enable Kessel authorization
KESSEL_IMPL=impl|mock           # Use real or mock client
KESSEL_HOSTNAME=hostname        # Kessel service hostname
KESSEL_PORT=9000                # Kessel service port
KESSEL_INSECURE=true|false      # Use insecure connection (dev only)
KESSEL_TIMEOUT=10               # Timeout in seconds
```

Set defaults in code:

```go
import "playbook-dispatcher/internal/api/kessel"

kessel.ConfigureDefaults(cfg)
```

## Examples

### Example 1: Simple Authorization Check

```go
func handleRequest(ctx context.Context, runID string) error {
    subject, _ := kessel.SubjectFromContext(ctx)

    allowed, err := kessel.CheckRunAccess(
        ctx,
        kesselClient,
        subject,
        runID,
        kessel.RelationRead,
    )

    if err != nil || !allowed {
        return errors.New("access denied")
    }

    // Proceed with request
    return nil
}
```

### Example 2: Filtering List Results

```go
func listRuns(ctx context.Context) ([]Run, error) {
    subject, _ := kessel.SubjectFromContext(ctx)

    // Get all runs from DB
    runs := fetchRunsFromDB()

    // Extract IDs
    runIDs := make([]string, len(runs))
    for i, run := range runs {
        runIDs[i] = run.ID
    }

    // Filter by authorization
    authorizedIDs, err := kessel.FilterAuthorizedResources(
        ctx,
        kesselClient,
        subject,
        kessel.RelationRead,
        kessel.ResourceTypeRun,
        runIDs,
    )

    if err != nil {
        return nil, err
    }

    // Return only authorized runs
    return filterRuns(runs, authorizedIDs), nil
}
```

### Example 3: Multiple Permission Types

```go
// Check if user can both read AND cancel a run
checks := []kessel.ResourceCheck{
    kessel.DispatcherRunCheck(subject, kessel.RelationRead, runID),
    kessel.DispatcherRunCheck(subject, kessel.RelationCancel, runID),
}

results, err := kesselClient.CheckBatch(ctx, checks)
if err != nil {
    return err
}

canRead := results[0]
canCancel := results[1]

if !canRead {
    return errors.New("cannot read run")
}

if !canCancel {
    return errors.New("cannot cancel run")
}
```

## Integration with Middleware

This package is designed to work with the Kessel middleware in `internal/api/middleware/kessel.go`.

See:
- `/docs/kessel-migration-guide.md` for migration strategy
- `/docs/kessel-quick-start.md` for quick reference
- `/internal/api/middleware/kessel.go` for middleware implementation
- `/internal/api/main_kessel_example.go` for route setup examples

## Testing

### Unit Tests

```go
func TestKesselCheck(t *testing.T) {
    client := kessel.NewMockKesselClient(true)

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

For integration tests, use the real Kessel client with a test Kessel instance or use the mock client with `KESSEL_IMPL=mock`.

## Error Handling

All client methods return errors. Common error scenarios:

- **Connection errors**: Kessel service unavailable
- **Context timeout**: Request took too long
- **Invalid request**: Malformed check request

Always handle errors and fail closed (deny access on error):

```go
allowed, err := client.Check(ctx, check)
if err != nil {
    log.Error("Kessel check failed", err)
    return http.StatusServiceUnavailable
}

if !allowed {
    return http.StatusForbidden
}
```

## Performance Considerations

1. **Use batch checks** when checking multiple resources
2. **Reuse client instances** - they maintain connection pools
3. **Set appropriate timeouts** in configuration
4. **Consider caching** for frequently accessed resources (future enhancement)

## Security Notes

1. Always validate identity before creating Subject
2. Never bypass authorization checks
3. Fail closed on errors (deny access)
4. Log authorization failures for auditing
5. Ensure resource tenant matches subject tenant
