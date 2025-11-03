package middleware

import (
	"net/http"
	"playbook-dispatcher/internal/api/instrumentation"
	"playbook-dispatcher/internal/api/kessel"
	"playbook-dispatcher/internal/common/utils"

	"github.com/labstack/echo/v4"
	identityMiddleware "github.com/redhatinsights/platform-go-middlewares/identity"
	"github.com/spf13/viper"
)

type kesselPermissionsKeyType int

const kesselPermissionsKey kesselPermissionsKeyType = iota

// KesselResourceExtractor is a function that extracts the resource being accessed from the request
// This allows different endpoints to specify which resource should be checked
type KesselResourceExtractor func(c echo.Context) (kessel.Resource, error)

// EnforceKesselPermissions creates middleware that enforces Kessel-based authorization
// The resourceExtractor function determines which resource is being accessed
// The relation specifies what permission is required (e.g., "read", "write")
func EnforceKesselPermissions(cfg *viper.Viper, relation string, resourceExtractor KesselResourceExtractor) echo.MiddlewareFunc {
	var client kessel.KesselClient

	// Initialize client based on configuration
	if cfg.GetString("kessel.impl") == "impl" {
		var err error
		client, err = kessel.NewKesselClient(cfg)
		if err != nil {
			panic(err) // Fail fast if client can't be created
		}
	} else {
		client = kessel.NewMockKesselClient(true)
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()
			ctx := req.Context()

			// Extract identity from context
			identity := identityMiddleware.GetIdentity(ctx)
			if identity.Identity.OrgID == "" || identity.Identity.User.Username == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, "missing identity information")
			}

			// Create subject from identity
			subject := kessel.Subject{
				Type:   kessel.SubjectTypeUser,
				ID:     identity.Identity.User.Username,
				Tenant: identity.Identity.OrgID,
			}

			// Extract resource from request
			resource, err := resourceExtractor(c)
			if err != nil {
				return echo.NewHTTPError(http.StatusBadRequest, "failed to extract resource from request")
			}

			// Perform authorization check
			check := kessel.ResourceCheck{
				Subject:  subject,
				Relation: relation,
				Resource: resource,
			}

			allowed, err := client.Check(ctx, check)
			if err != nil {
				instrumentation.RbacError(c, err)
				return echo.NewHTTPError(http.StatusServiceUnavailable, "error checking permissions with Kessel")
			}

			if !allowed {
				instrumentation.RbacRejected(c)
				return echo.NewHTTPError(http.StatusForbidden)
			}

			// Store the subject in context for later use
			utils.SetRequestContextValue(c, kesselPermissionsKey, subject)

			return next(c)
		}
	}
}

// EnforceKesselOrgPermissions enforces organization-level permissions
// This is used when the resource type is checked at org level rather than specific resource
func EnforceKesselOrgPermissions(cfg *viper.Viper, relation string, resourceType string) echo.MiddlewareFunc {
	// Resource extractor that creates an org-level resource
	extractor := func(c echo.Context) (kessel.Resource, error) {
		ctx := c.Request().Context()
		identity := identityMiddleware.GetIdentity(ctx)

		return kessel.Resource{
			Type:   resourceType,
			ID:     "*", // Wildcard for org-level permissions
			Tenant: identity.Identity.OrgID,
		}, nil
	}

	return EnforceKesselPermissions(cfg, relation, extractor)
}

// GetKesselSubject retrieves the subject from the request context
func GetKesselSubject(c echo.Context) kessel.Subject {
	return c.Request().Context().Value(kesselPermissionsKey).(kessel.Subject)
}
