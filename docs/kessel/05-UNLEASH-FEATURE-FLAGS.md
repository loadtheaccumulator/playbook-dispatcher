# Implementing Unleash Feature Flags for Kessel Migration

## Overview

This document describes how to implement Unleash feature flags for the Kessel authorization migration in playbook-dispatcher, following the patterns established in the [edge-api](https://github.com/RedHatInsights/edge-api) repository.

## Table of Contents

1. [Why Unleash?](#why-unleash)
2. [Architecture](#architecture)
3. [Implementation Steps](#implementation-steps)
4. [Feature Flag Definitions](#feature-flag-definitions)
5. [Usage in Code](#usage-in-code)
6. [Gradual Rollout Strategy](#gradual-rollout-strategy)
7. [Testing](#testing)
8. [Monitoring](#monitoring)

## Why Unleash?

Unleash provides several advantages over simple environment variable toggles:

| Feature | Environment Variables | Unleash |
|---------|----------------------|---------|
| **Runtime Toggle** | ❌ Requires restart | ✅ Instant updates |
| **Gradual Rollout** | ❌ All or nothing | ✅ Percentage-based |
| **User Targeting** | ❌ Not supported | ✅ By org, user, etc. |
| **A/B Testing** | ❌ Not supported | ✅ Built-in |
| **Audit Log** | ❌ Manual tracking | ✅ Automatic |
| **Dashboard** | ❌ None | ✅ Web UI |
| **Rollback** | ⚠️ Requires deployment | ✅ Instant toggle |

## Architecture

### Current State (Environment Variables)

```
┌─────────────┐
│ Environment │
│  Variables  │
│             │
│ AUTH_SYSTEM │
│   = rbac    │
└──────┬──────┘
       │
       ↓
┌─────────────┐
│   Config    │
│   Loader    │
└──────┬──────┘
       │
       ↓
┌─────────────┐
│    Auth     │
│  Selector   │
└─────────────┘
```

### Target State (Unleash)

```
┌──────────────┐      ┌──────────────┐
│   Unleash    │─────→│   Unleash    │
│   Server     │      │    Client    │
│              │      │   (in-app)   │
└──────────────┘      └──────┬───────┘
                             │
                    Poll every 15s
                             │
                             ↓
                      ┌──────────────┐
                      │ Feature Flag │
                      │    Cache     │
                      └──────┬───────┘
                             │
                             ↓
                      ┌──────────────┐
                      │     Auth     │
                      │   Selector   │
                      └──────────────┘
```

## Implementation Steps

### Step 1: Add Dependencies

**File**: `go.mod`

```bash
go get github.com/Unleash/unleash-client-go/v4
```

### Step 2: Create Unleash Package Structure

Create the following directory structure following edge-api pattern:

```
internal/api/unleash/
├── client.go           # Unleash client initialization
├── listener.go         # Event listener for logging
├── features/
│   └── feature.go      # Feature flag definitions
└── unleash_mock.go     # Mock client for testing
```

### Step 3: Implement Unleash Listener

**File**: `internal/api/unleash/listener.go`

Based on [edge-api's EdgeListener](https://github.com/RedHatInsights/edge-api/blob/main/unleash/edge_listener.go):

```go
package unleash

import (
    unleashclient "github.com/Unleash/unleash-client-go/v4"
    "github.com/sirupsen/logrus"
)

// DispatcherListener implements the Unleash event listener interface
type DispatcherListener struct {
    log *logrus.Logger
}

func NewDispatcherListener(log *logrus.Logger) *DispatcherListener {
    return &DispatcherListener{log: log}
}

// OnError logs Unleash errors
func (l *DispatcherListener) OnError(err error) {
    l.log.WithError(err).Warn("Unleash client error")
}

// OnWarning logs Unleash warnings
func (l *DispatcherListener) OnWarning(warning error) {
    l.log.WithError(warning).Warn("Unleash client warning")
}

// OnReady logs when Unleash is ready
func (l *DispatcherListener) OnReady() {
    l.log.Info("Unleash client ready")
}

// OnCount tracks feature flag evaluations (optional)
func (l *DispatcherListener) OnCount(name string, enabled bool) {
    // Can be used for metrics
}

// OnSent logs when metrics are sent (optional)
func (l *DispatcherListener) OnSent(payload unleashclient.MetricsData) {
    l.log.Debug("Unleash metrics sent")
}

// OnRegistered logs client registration
func (l *DispatcherListener) OnRegistered(payload unleashclient.ClientData) {
    l.log.WithFields(logrus.Fields{
        "app_name": payload.AppName,
        "instance_id": payload.InstanceId,
    }).Debug("Unleash client registered")
}
```

### Step 4: Define Feature Flags

**File**: `internal/api/unleash/features/feature.go`

```go
package features

// Feature flag definitions for playbook-dispatcher
const (
    // KesselAuthEnabled controls whether Kessel authorization is enabled
    // When false: RBAC only
    // When true: Check KesselAuthMode for behavior
    KesselAuthEnabled = "playbook-dispatcher.kessel-auth-enabled"

    // KesselAuthMode controls which auth system to use when Kessel is enabled
    // Values: "rbac" | "validation" | "kessel"
    KesselAuthMode = "playbook-dispatcher.kessel-auth-mode"
)

// Flag represents a feature flag with environment variable fallback
type Flag struct {
    Name   string
    EnvVar string
}

// Kessel feature flags
var (
    // KesselEnabled toggles Kessel authorization
    KesselEnabled = Flag{
        Name:   KesselAuthEnabled,
        EnvVar: "KESSEL_ENABLED",
    }

    // KesselMode determines auth mode: rbac, validation, or kessel
    KesselMode = Flag{
        Name:   KesselAuthMode,
        EnvVar: "KESSEL_AUTH_MODE",
    }
)
```

### Step 5: Implement Unleash Client

**File**: `internal/api/unleash/client.go`

```go
package unleash

import (
    "context"
    "fmt"

    unleashclient "github.com/Unleash/unleash-client-go/v4"
    unleashcontext "github.com/Unleash/unleash-client-go/v4/context"
    "github.com/sirupsen/logrus"

    "playbook-dispatcher/internal/api/unleash/features"
    "playbook-dispatcher/internal/common/config"
)

// Client wraps the Unleash client
type Client interface {
    IsEnabled(feature string, ctx *unleashcontext.Context) bool
    IsEnabledWithFallback(feature string, ctx *unleashcontext.Context, fallback bool) bool
    GetVariant(feature string, ctx *unleashcontext.Context) string
    Close() error
}

type unleashClient struct {
    client *unleashclient.Client
    log    *logrus.Logger
}

// Initialize creates and initializes the Unleash client
func Initialize(cfg config.Config, log *logrus.Logger) (Client, error) {
    if !cfg.FeatureFlagsConfigured() {
        log.Warn("Unleash not configured, using environment variable fallback")
        return NewMockClient(cfg, log), nil
    }

    unleashOptions := []unleashclient.ConfigOption{
        unleashclient.WithUrl(cfg.UnleashURL),
        unleashclient.WithAppName("playbook-dispatcher"),
        unleashclient.WithInstanceId(cfg.Hostname), // Or pod name
        unleashclient.WithListener(NewDispatcherListener(log)),
        unleashclient.WithRefreshInterval(15), // Poll every 15 seconds
    }

    // Add authentication if token is configured
    if cfg.FeatureFlagsAPIToken != "" {
        unleashOptions = append(unleashOptions,
            unleashclient.WithCustomHeaders(map[string]string{
                "Authorization": cfg.FeatureFlagsAPIToken,
            }),
        )
    }

    client, err := unleashclient.NewClient(unleashOptions...)
    if err != nil {
        return nil, fmt.Errorf("failed to create Unleash client: %w", err)
    }

    // Wait for client to be ready
    <-client.Ready()

    return &unleashClient{
        client: client,
        log:    log,
    }, nil
}

// IsEnabled checks if a feature is enabled
func (u *unleashClient) IsEnabled(feature string, ctx *unleashcontext.Context) bool {
    return u.client.IsEnabled(feature, unleashclient.WithContext(*ctx))
}

// IsEnabledWithFallback checks if a feature is enabled with a fallback value
func (u *unleashClient) IsEnabledWithFallback(feature string, ctx *unleashcontext.Context, fallback bool) bool {
    if ctx == nil {
        ctx = &unleashcontext.Context{}
    }
    return u.client.IsEnabled(feature,
        unleashclient.WithContext(*ctx),
        unleashclient.WithFallback(fallback),
    )
}

// GetVariant returns the variant for a feature flag
func (u *unleashClient) GetVariant(feature string, ctx *unleashcontext.Context) string {
    if ctx == nil {
        ctx = &unleashcontext.Context{}
    }
    variant := u.client.GetVariant(feature, unleashclient.WithContext(*ctx))
    return variant.Name
}

// Close shuts down the Unleash client
func (u *unleashClient) Close() error {
    u.client.Close()
    return nil
}

// BuildContext creates an Unleash context from request data
func BuildContext(orgID, userID, accountNumber string) *unleashcontext.Context {
    ctx := unleashcontext.Context{
        Properties: map[string]string{
            "orgId":         orgID,
            "userId":        userID,
            "accountNumber": accountNumber,
        },
    }
    return &ctx
}
```

### Step 6: Create Mock Client

**File**: `internal/api/unleash/unleash_mock.go`

```go
package unleash

import (
    "os"
    "strings"

    unleashcontext "github.com/Unleash/unleash-client-go/v4/context"
    "github.com/sirupsen/logrus"

    "playbook-dispatcher/internal/common/config"
    "playbook-dispatcher/internal/api/unleash/features"
)

// mockClient provides environment variable fallback when Unleash is not configured
type mockClient struct {
    cfg config.Config
    log *logrus.Logger
}

// NewMockClient creates a mock client that reads from environment variables
func NewMockClient(cfg config.Config, log *logrus.Logger) Client {
    log.Info("Using mock Unleash client with environment variable fallback")
    return &mockClient{
        cfg: cfg,
        log: log,
    }
}

func (m *mockClient) IsEnabled(feature string, ctx *unleashcontext.Context) bool {
    return m.IsEnabledWithFallback(feature, ctx, false)
}

func (m *mockClient) IsEnabledWithFallback(feature string, ctx *unleashcontext.Context, fallback bool) bool {
    // Map feature flags to environment variables
    var envVar string
    switch feature {
    case features.KesselAuthEnabled:
        envVar = features.KesselEnabled.EnvVar
    case features.KesselAuthMode:
        envVar = features.KesselMode.EnvVar
    default:
        m.log.Warnf("Unknown feature flag: %s, using fallback: %v", feature, fallback)
        return fallback
    }

    value := os.Getenv(envVar)
    if value == "" {
        return fallback
    }

    // Parse boolean or string values
    lowerValue := strings.ToLower(value)
    return lowerValue == "true" || lowerValue == "1" || lowerValue == "yes"
}

func (m *mockClient) GetVariant(feature string, ctx *unleashcontext.Context) string {
    var envVar string
    switch feature {
    case features.KesselAuthMode:
        envVar = features.KesselMode.EnvVar
    default:
        return "disabled"
    }

    value := os.Getenv(envVar)
    if value == "" {
        return "disabled"
    }
    return value
}

func (m *mockClient) Close() error {
    return nil
}
```

### Step 7: Update Configuration

**File**: `internal/common/config/config.go`

Add Unleash configuration fields:

```go
type Config struct {
    // ... existing fields ...

    // Unleash / Feature Flags
    UnleashURL           string `mapstructure:"unleash_url"`
    FeatureFlagsURL      string `mapstructure:"feature_flags_url"`
    FeatureFlagsAPIToken string `mapstructure:"feature_flags_api_token"`
    Hostname             string `mapstructure:"hostname"`

    // Fallback environment variables (when Unleash unavailable)
    KesselEnabled   bool   `mapstructure:"kessel_enabled"`
    KesselAuthMode  string `mapstructure:"kessel_auth_mode"`
}

// FeatureFlagsConfigured returns true if feature flags are configured
func (c *Config) FeatureFlagsConfigured() bool {
    return c.UnleashURL != "" || c.FeatureFlagsURL != ""
}

// In init() or GetConfig():
func init() {
    // ... existing code ...

    // Unleash configuration
    if clowder.IsClowderEnabled() {
        if featureFlags := clowder.LoadedConfig.FeatureFlags; featureFlags != nil {
            DefaultConfig.UnleashURL = fmt.Sprintf(
                "%s://%s:%d/api",
                featureFlags.Scheme,
                featureFlags.Hostname,
                featureFlags.Port,
            )
            DefaultConfig.FeatureFlagsAPIToken = featureFlags.ClientAccessToken
        }
    }

    // Fallback to environment variables
    if DefaultConfig.UnleashURL == "" {
        DefaultConfig.UnleashURL = os.Getenv("UNLEASH_URL")
    }
    if DefaultConfig.FeatureFlagsAPIToken == "" {
        DefaultConfig.FeatureFlagsAPIToken = os.Getenv("UNLEASH_TOKEN")
    }

    // Get hostname for Unleash instance ID
    hostname, _ := os.Hostname()
    DefaultConfig.Hostname = hostname

    // Fallback feature flag values
    DefaultConfig.KesselEnabled = os.Getenv("KESSEL_ENABLED") == "true"
    DefaultConfig.KesselAuthMode = os.Getenv("KESSEL_AUTH_MODE")
    if DefaultConfig.KesselAuthMode == "" {
        DefaultConfig.KesselAuthMode = "rbac" // Safe default
    }
}
```

### Step 8: Update Authorization Selector

**File**: `internal/api/middleware/authselector.go`

Replace environment variable checks with Unleash:

```go
package middleware

import (
    "github.com/labstack/echo/v4"
    "github.com/redhatinsights/platform-go-middlewares/identity"

    "playbook-dispatcher/internal/api/unleash"
    "playbook-dispatcher/internal/api/unleash/features"
    "playbook-dispatcher/internal/common/config"
)

type AuthSelector struct {
    unleashClient unleash.Client
    cfg           config.Config
}

func NewAuthSelector(unleashClient unleash.Client, cfg config.Config) *AuthSelector {
    return &AuthSelector{
        unleashClient: unleashClient,
        cfg:           cfg,
    }
}

// SelectAuthorizationMiddleware returns the appropriate auth middleware based on feature flags
func (a *AuthSelector) SelectAuthorizationMiddleware() echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            // Build Unleash context from identity
            id := identity.GetIdentity(c.Request().Context())
            unleashCtx := unleash.BuildContext(
                id.Identity.OrgID,
                getUserID(id),
                id.Identity.AccountNumber,
            )

            // Check if Kessel is enabled for this request
            kesselEnabled := a.unleashClient.IsEnabledWithFallback(
                features.KesselAuthEnabled,
                unleashCtx,
                a.cfg.KesselEnabled, // Fallback to env var
            )

            if !kesselEnabled {
                // Use RBAC only
                return EnforcePermissions(a.cfg, DispatcherPermission("run", "read"))(next)(c)
            }

            // Kessel is enabled, check the mode
            mode := a.unleashClient.GetVariant(features.KesselAuthMode, unleashCtx)
            if mode == "" {
                mode = a.cfg.KesselAuthMode // Fallback to env var
            }

            switch mode {
            case "validation":
                // Run both, RBAC enforces
                return chainMiddleware(
                    EnforcePermissions(a.cfg, DispatcherPermission("run", "read")),
                    logOnlyKesselMiddleware(a.cfg),
                )(next)(c)
            case "kessel":
                // Kessel only
                return EnforceKesselPermissions(a.cfg)(next)(c)
            default: // "rbac" or unknown
                // RBAC only (safe default)
                return EnforcePermissions(a.cfg, DispatcherPermission("run", "read"))(next)(c)
            }
        }
    }
}

func getUserID(id identity.XRHID) string {
    switch id.Identity.Type {
    case "User":
        return id.Identity.User.UserID
    case "ServiceAccount":
        return id.Identity.ServiceAccount.UserId
    default:
        return ""
    }
}
```

### Step 9: Update Main Application

**File**: `internal/api/main.go`

Initialize Unleash client:

```go
package main

import (
    // ... existing imports ...
    "playbook-dispatcher/internal/api/unleash"
)

func main() {
    cfg := config.Get()
    log := logger.Get()

    // Initialize Unleash client
    unleashClient, err := unleash.Initialize(cfg, log)
    if err != nil {
        log.WithError(err).Fatal("Failed to initialize Unleash client")
    }
    defer unleashClient.Close()

    // ... existing setup ...

    // Create auth selector with Unleash
    authSelector := middleware.NewAuthSelector(unleashClient, cfg)

    // Use in middleware
    public.Use(authSelector.SelectAuthorizationMiddleware())

    // ... rest of application ...
}
```

### Step 10: Update Deployment Configuration

**File**: `deploy/clowdapp.yaml`

Add Unleash dependency:

```yaml
apiVersion: v1
kind: ClowdApp
metadata:
  name: playbook-dispatcher
spec:
  # Add feature flags dependency
  featureFlags: true

  deployments:
  - name: api
    podSpec:
      env:
        # Fallback environment variables (if Unleash unavailable)
        - name: KESSEL_ENABLED
          value: ${KESSEL_ENABLED}
        - name: KESSEL_AUTH_MODE
          value: ${KESSEL_AUTH_MODE}
        # ... other Kessel config ...

# In parameters:
parameters:
  - name: KESSEL_ENABLED
    value: "false"
  - name: KESSEL_AUTH_MODE
    value: "rbac"
```

## Feature Flag Definitions

### Flag: `playbook-dispatcher.kessel-auth-enabled`

**Type**: Boolean toggle

**Purpose**: Master switch to enable/disable Kessel authorization

**Values**:
- `false` (default): RBAC only, Kessel code not executed
- `true`: Kessel available, check mode flag for behavior

**Strategies**:
- **Default**: Off (RBAC only)
- **UserIDs**: Enable for specific test users
- **Gradual Rollout**: Enable for X% of users
- **OrgID**: Enable for specific organizations

**Environment Fallback**: `KESSEL_ENABLED=true|false`

---

### Flag: `playbook-dispatcher.kessel-auth-mode`

**Type**: String variant

**Purpose**: Controls which auth system enforces when Kessel is enabled

**Variants**:
- `rbac` (default): RBAC only (safe fallback)
- `validation`: Both run, RBAC enforces, Kessel logs
- `kessel`: Kessel only

**Strategies**:
- **Default**: `rbac`
- **By OrgID**: Different orgs can be in different phases
- **By User**: Internal users can test `kessel` mode
- **Gradual Rollout**: Move % of traffic through phases

**Environment Fallback**: `KESSEL_AUTH_MODE=rbac|validation|kessel`

---

## Usage in Code

### Basic Feature Flag Check

```go
import (
    "playbook-dispatcher/internal/api/unleash"
    "playbook-dispatcher/internal/api/unleash/features"
)

// In handler or service:
unleashCtx := unleash.BuildContext(orgID, userID, accountNumber)

if unleashClient.IsEnabled(features.KesselAuthEnabled, unleashCtx) {
    // Kessel is enabled for this org/user
    useKesselAuth()
} else {
    // Use RBAC
    useRBACAuth()
}
```

### Get Variant

```go
mode := unleashClient.GetVariant(features.KesselAuthMode, unleashCtx)

switch mode {
case "validation":
    runBothSystems()
case "kessel":
    runKesselOnly()
default: // "rbac"
    runRBACOnly()
}
```

### With Fallback

```go
// If Unleash is down, fallback to config value
kesselEnabled := unleashClient.IsEnabledWithFallback(
    features.KesselAuthEnabled,
    unleashCtx,
    config.Get().KesselEnabled, // Fallback value
)
```

## Gradual Rollout Strategy

### Phase 1: Internal Testing (Week 1)

**Unleash Configuration**:

```
Flag: playbook-dispatcher.kessel-auth-enabled
Strategy: UserIDs
Users: internal-user-1, internal-user-2, internal-user-3
Enabled: true

Flag: playbook-dispatcher.kessel-auth-mode
Strategy: Default
Value: validation
```

**Result**: Internal users run both systems, RBAC enforces

---

### Phase 2: Canary Rollout (Weeks 2-3)

**Unleash Configuration**:

```
Flag: playbook-dispatcher.kessel-auth-enabled
Strategy: Gradual Rollout
Percentage: 5%
Enabled: true

Flag: playbook-dispatcher.kessel-auth-mode
Strategy: Default
Value: validation
```

**Result**: 5% of all requests run both systems for comparison

**Monitoring**: Compare RBAC vs Kessel results in logs

**Success Criteria**: >99.9% agreement between systems

---

### Phase 3: Increase Rollout (Week 4)

```
Flag: playbook-dispatcher.kessel-auth-enabled
Strategy: Gradual Rollout
Percentage: 25% → 50% → 75%
```

**Increase every 2-3 days** if metrics are good

---

### Phase 4: Kessel Enforcement - Canary (Week 5)

**Unleash Configuration**:

```
Flag: playbook-dispatcher.kessel-auth-enabled
Strategy: OrgIDs (specific test orgs)
Orgs: org-123, org-456
Enabled: true

Flag: playbook-dispatcher.kessel-auth-mode
Strategy: OrgIDs
Orgs: org-123, org-456
Value: kessel
```

**Result**: Specific orgs use Kessel for enforcement

---

### Phase 5: Kessel Enforcement - Gradual (Weeks 6-7)

```
Flag: playbook-dispatcher.kessel-auth-enabled
Strategy: Gradual Rollout
Percentage: 100%
Enabled: true

Flag: playbook-dispatcher.kessel-auth-mode
Strategy: Gradual Rollout
Rollout:
  - 5% → validation
  - 5% → kessel
  - 25% → kessel
  - 50% → kessel
  - 100% → kessel
```

---

### Phase 6: Complete Migration (Week 8+)

```
Flag: playbook-dispatcher.kessel-auth-enabled
Strategy: Default
Enabled: true

Flag: playbook-dispatcher.kessel-auth-mode
Strategy: Default
Value: kessel
```

**Result**: All traffic uses Kessel

**Next Steps**: Remove RBAC code after 2-4 week stability period

---

## Testing

### Unit Tests with Mock Client

```go
func TestAuthSelector_KesselDisabled(t *testing.T) {
    mockUnleash := unleash.NewMockClient(config.Config{
        KesselEnabled: false,
    }, logger)

    selector := middleware.NewAuthSelector(mockUnleash, cfg)
    // Test RBAC is selected
}

func TestAuthSelector_ValidationMode(t *testing.T) {
    mockUnleash := unleash.NewMockClient(config.Config{
        KesselEnabled: true,
        KesselAuthMode: "validation",
    }, logger)

    selector := middleware.NewAuthSelector(mockUnleash, cfg)
    // Test both systems run
}
```

### Integration Tests

```go
func TestUnleashIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }

    // Requires Unleash server running
    cfg := config.Config{
        UnleashURL: "http://localhost:4242/api",
    }

    client, err := unleash.Initialize(cfg, logger)
    require.NoError(t, err)
    defer client.Close()

    ctx := unleash.BuildContext("test-org", "test-user", "test-account")
    enabled := client.IsEnabled(features.KesselAuthEnabled, ctx)
    // Assertions based on Unleash configuration
}
```

## Monitoring

### Metrics to Track

Add Prometheus metrics for feature flag usage:

```go
var (
    unleashFlagEvaluations = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "unleash_flag_evaluations_total",
            Help: "Number of feature flag evaluations",
        },
        []string{"flag", "enabled"},
    )

    unleashFlagErrors = promauto.NewCounter(
        prometheus.CounterOpts{
            Name: "unleash_flag_errors_total",
            Help: "Number of Unleash errors",
        },
    )
)

// In IsEnabled():
func (u *unleashClient) IsEnabled(feature string, ctx *unleashcontext.Context) bool {
    enabled := u.client.IsEnabled(feature, unleashclient.WithContext(*ctx))
    unleashFlagEvaluations.WithLabelValues(feature, fmt.Sprintf("%v", enabled)).Inc()
    return enabled
}
```

### Dashboards

**Grafana Dashboard Queries**:

```promql
# Feature flag evaluation rate
rate(unleash_flag_evaluations_total[5m])

# Percentage of requests with Kessel enabled
sum(rate(unleash_flag_evaluations_total{flag="playbook-dispatcher.kessel-auth-enabled",enabled="true"}[5m]))
/
sum(rate(unleash_flag_evaluations_total{flag="playbook-dispatcher.kessel-auth-enabled"}[5m]))
* 100

# Unleash error rate
rate(unleash_flag_errors_total[5m])
```

### Alerts

```yaml
- alert: UnleashHighErrorRate
  expr: rate(unleash_flag_errors_total[5m]) > 0.01
  annotations:
    summary: "Unleash feature flag errors detected"
    description: "Unleash client experiencing errors, falling back to env vars"

- alert: UnleashClientDown
  expr: up{job="unleash"} == 0
  annotations:
    summary: "Unleash server is down"
    description: "Feature flags unavailable, using fallback values"
```

## Rollback Scenarios

### Scenario 1: Kessel Issues in Production

**Problem**: Kessel authorization causing errors

**Solution**: Instant rollback via Unleash dashboard

```
1. Open Unleash dashboard
2. Navigate to: playbook-dispatcher.kessel-auth-mode
3. Change default variant: kessel → rbac
4. Save (applies immediately, no restart needed)
```

**Time to Rollback**: ~30 seconds

---

### Scenario 2: Unleash Server Down

**Problem**: Unleash server unavailable

**Behavior**:
- Mock client automatically activates
- Falls back to environment variables
- Application continues running

**No action needed** - automatic fallback

---

### Scenario 3: Gradual Rollback

**Problem**: Need to reduce Kessel traffic

**Solution**: Decrease gradual rollout percentage

```
playbook-dispatcher.kessel-auth-enabled
Gradual Rollout: 75% → 50% → 25% → 5% → 0%
```

---

## Best Practices

### 1. Always Provide Fallbacks

```go
// ✅ Good: Has fallback
kesselEnabled := unleashClient.IsEnabledWithFallback(
    features.KesselAuthEnabled,
    ctx,
    config.Get().KesselEnabled,
)

// ❌ Bad: No fallback if Unleash is down
kesselEnabled := unleashClient.IsEnabled(features.KesselAuthEnabled, ctx)
```

### 2. Use Context for Targeting

```go
// ✅ Good: Provides org/user context for targeting
ctx := unleash.BuildContext(orgID, userID, accountNumber)
enabled := client.IsEnabled(feature, ctx)

// ❌ Bad: No context, can't target specific orgs/users
enabled := client.IsEnabled(feature, nil)
```

### 3. Log Feature Flag Decisions

```go
mode := unleashClient.GetVariant(features.KesselAuthMode, ctx)
log.WithFields(logrus.Fields{
    "org_id": orgID,
    "mode": mode,
    "feature": features.KesselAuthMode,
}).Debug("Feature flag evaluated")
```

### 4. Test Both Paths

```go
// Test with flag enabled
func TestWithKesselEnabled(t *testing.T) { ... }

// Test with flag disabled
func TestWithKesselDisabled(t *testing.T) { ... }

// Test with Unleash unavailable (fallback)
func TestWithUnleashDown(t *testing.T) { ... }
```

## Comparison: Environment Variables vs Unleash

### Environment Variable Approach

**Pros**:
- ✅ Simple to implement
- ✅ No external dependencies
- ✅ Works in all environments

**Cons**:
- ❌ Requires pod restart to change
- ❌ All-or-nothing (no gradual rollout)
- ❌ No targeting by org/user
- ❌ No audit trail
- ❌ Manual rollback process

### Unleash Approach

**Pros**:
- ✅ Instant toggle (no restart)
- ✅ Gradual rollout (5% → 100%)
- ✅ Target by org, user, account
- ✅ A/B testing capability
- ✅ Audit trail in dashboard
- ✅ Instant rollback
- ✅ Environment variable fallback

**Cons**:
- ⚠️ Additional dependency (Unleash server)
- ⚠️ Slightly more complex code
- ⚠️ Requires Unleash dashboard access

## Recommended Approach

**Use Unleash with Environment Variable Fallback**:

1. **Primary**: Unleash for gradual rollout and instant control
2. **Fallback**: Environment variables when Unleash unavailable
3. **Testing**: Mock client that reads environment variables

This provides the best of both worlds:
- Dynamic control via Unleash in production
- Reliability via environment variable fallback
- Simplicity in development/testing

## References

- [Unleash Documentation](https://docs.getunleash.io/)
- [Unleash Go Client](https://github.com/Unleash/unleash-client-go)
- [Edge-API Unleash Implementation](https://github.com/RedHatInsights/edge-api/tree/main/unleash)
- [Gradual Rollout Strategies](https://docs.getunleash.io/reference/activation-strategies#gradual-rollout)
