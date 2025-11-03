package public

// This example shows how to implement the runsList controller with Kessel service filtering
// This maintains compatibility with the existing RBAC service filtering behavior

import (
	"net/http"
	"playbook-dispatcher/internal/api/kessel"
	dbModel "playbook-dispatcher/internal/common/model/db"

	"github.com/labstack/echo/v4"
	identityMiddleware "github.com/redhatinsights/platform-go-middlewares/identity"
	"gorm.io/gorm"
)

type controllersWithKessel struct {
	database     *gorm.DB
	kesselClient kessel.KesselClient
}

// ApiRunsListWithServiceFiltering demonstrates how to implement service filtering with Kessel
func (this *controllersWithKessel) ApiRunsListWithServiceFiltering(ctx echo.Context, params ApiRunsListParams) error {
	identity := identityMiddleware.Get(ctx.Request().Context())
	db := this.database.WithContext(ctx.Request().Context())

	// Create subject from identity
	subject := kessel.Subject{
		Type:   kessel.SubjectTypeUser,
		ID:     identity.Identity.User.Username,
		Tenant: identity.Identity.OrgID,
	}

	// Base query with tenant isolation
	queryBuilder := db.Table("runs").Where("org_id = ?", identity.Identity.OrgID)

	// Apply Kessel service filtering (replaces RBAC GetPredicateValues)
	allowedServices, filterRequired, err := kessel.GetAllowedServices(
		ctx.Request().Context(),
		this.kesselClient,
		subject,
	)

	if err != nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "error checking service permissions")
	}

	if filterRequired {
		if len(allowedServices) == 0 {
			// User has no service access, return empty list
			return ctx.JSON(http.StatusOK, map[string]interface{}{
				"data":  []interface{}{},
				"meta":  map[string]int{"count": 0},
				"links": map[string]interface{}{},
			})
		}

		// Filter by allowed services
		queryBuilder = queryBuilder.Where("service IN ?", allowedServices)
	}
	// If filterRequired is false, user has org-level access (no service filtering)

	// Continue with rest of query as normal
	// ... (sorting, pagination, field selection, etc.)

	var dbRuns []dbModel.Run
	dbResult := queryBuilder.Find(&dbRuns)

	if dbResult.Error != nil {
		return ctx.NoContent(http.StatusInternalServerError)
	}

	return ctx.JSON(http.StatusOK, formatRunsResponse(dbRuns))
}

// Example: Using service filtering in runHostsList
func (this *controllersWithKessel) ApiRunHostsListWithServiceFiltering(ctx echo.Context, params ApiRunHostsListParams) error {
	identity := identityMiddleware.Get(ctx.Request().Context())

	subject := kessel.Subject{
		Type:   kessel.SubjectTypeUser,
		ID:     identity.Identity.User.Username,
		Tenant: identity.Identity.OrgID,
	}

	queryBuilder := this.database.
		WithContext(ctx.Request().Context()).
		Table("run_hosts").
		Joins("INNER JOIN runs on runs.id = run_hosts.run_id").
		Where("runs.org_id = ?", identity.Identity.OrgID)

	// Apply service filtering (same pattern)
	allowedServices, filterRequired, err := kessel.GetAllowedServices(
		ctx.Request().Context(),
		this.kesselClient,
		subject,
	)

	if err != nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "error checking service permissions")
	}

	if filterRequired {
		if len(allowedServices) == 0 {
			return ctx.JSON(http.StatusOK, map[string]interface{}{
				"data":  []interface{}{},
				"meta":  map[string]int{"count": 0},
				"links": map[string]interface{}{},
			})
		}
		queryBuilder = queryBuilder.Where("runs.service IN ?", allowedServices)
	}

	// Continue with rest of query...
	var dbRunHosts []dbModel.RunHost
	dbResult := queryBuilder.Find(&dbRunHosts)

	if dbResult.Error != nil {
		return ctx.NoContent(http.StatusInternalServerError)
	}

	return ctx.JSON(http.StatusOK, formatRunHostsResponse(dbRunHosts))
}

// Example: Checking specific service access before creating a run
func (this *controllersWithKessel) ApiRunsCreateWithServiceCheck(ctx echo.Context, service string) error {
	identity := identityMiddleware.Get(ctx.Request().Context())

	subject := kessel.Subject{
		Type:   kessel.SubjectTypeUser,
		ID:     identity.Identity.User.Username,
		Tenant: identity.Identity.OrgID,
	}

	// Check if user has access to this service
	hasAccess, err := kessel.HasServiceAccess(
		ctx.Request().Context(),
		this.kesselClient,
		subject,
		service,
	)

	if err != nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "error checking service permissions")
	}

	if !hasAccess {
		return echo.NewHTTPError(http.StatusForbidden, "no access to service: "+service)
	}

	// Proceed with creating the run...
	return ctx.JSON(http.StatusCreated, map[string]string{"status": "created"})
}

// Helper functions (placeholders)
func formatRunHostsResponse(hosts []dbModel.RunHost) interface{} {
	return hosts
}
