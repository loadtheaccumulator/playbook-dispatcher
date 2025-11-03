package public

// This file demonstrates how to refactor runsList.go to use Kessel instead of RBAC
// It shows the controller implementation with Kessel authorization

import (
	"net/http"
	"playbook-dispatcher/internal/api/kessel"
	dbModel "playbook-dispatcher/internal/common/model/db"

	"github.com/labstack/echo/v4"
	identityMiddleware "github.com/redhatinsights/platform-go-middlewares/identity"
	"gorm.io/gorm"
)

// Example controller method using Kessel for authorization
// This would replace the existing ApiRunsList method

type kesselControllers struct {
	database      *gorm.DB
	kesselClient  kessel.KesselClient
	// ... other fields
}

// ApiRunsListWithKessel demonstrates listing runs with Kessel authorization
// The middleware has already verified org-level read permission before this is called
func (this *kesselControllers) ApiRunsListWithKessel(ctx echo.Context, params ApiRunsListParams) error {
	// Get identity and create subject
	identity := identityMiddleware.GetIdentity(ctx.Request().Context())
	subject := kessel.Subject{
		Type:   kessel.SubjectTypeUser,
		ID:     identity.Identity.User.Username,
		Tenant: identity.Identity.OrgID,
	}

	// Build base query for runs
	query := this.database.
		Select(getFields(params.Fields)...).
		Where("account = ?", identity.Identity.OrgID)

	// Apply filters, sorting, pagination as before
	if params.Filter != nil {
		query = applyFilters(query, params.Filter)
	}

	if params.Limit != nil {
		query = query.Limit(int(*params.Limit))
	}

	if params.Offset != nil {
		query = query.Offset(int(*params.Offset))
	}

	query = query.Order(getOrderBy(params))

	// Execute query
	var runs []dbModel.Run
	result := query.Find(&runs)

	if result.Error != nil {
		return ctx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to query runs",
		})
	}

	// OPTION 1: If org-level permission is sufficient, return all runs
	// (This is the simple case - middleware already checked permission)
	return ctx.JSON(http.StatusOK, formatRunsResponse(runs))

	// OPTION 2: If you need resource-level filtering, filter by permission
	// Uncomment below to filter runs by individual permissions:
	/*
	runIDs := make([]string, len(runs))
	for i, run := range runs {
		runIDs[i] = run.ID.String()
	}

	authorizedIDs, err := kessel.FilterAuthorizedResources(
		ctx.Request().Context(),
		this.kesselClient,
		subject,
		kessel.RelationRead,
		kessel.ResourceTypeRun,
		runIDs,
	)

	if err != nil {
		return ctx.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "failed to check permissions",
		})
	}

	// Filter runs to only authorized ones
	authorizedRuns := make([]dbModel.Run, 0)
	authorizedSet := make(map[string]bool)
	for _, id := range authorizedIDs {
		authorizedSet[id] = true
	}

	for _, run := range runs {
		if authorizedSet[run.ID.String()] {
			authorizedRuns = append(authorizedRuns, run)
		}
	}

	return ctx.JSON(http.StatusOK, formatRunsResponse(authorizedRuns))
	*/
}

// ApiRunsGetWithKessel demonstrates getting a specific run with Kessel
// The middleware has already verified permission to access this specific run
func (this *kesselControllers) ApiRunsGetWithKessel(ctx echo.Context, runID string) error {
	identity := identityMiddleware.GetIdentity(ctx.Request().Context())

	var run dbModel.Run
	result := this.database.
		Where("id = ? AND account = ?", runID, identity.Identity.OrgID).
		First(&run)

	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return ctx.NoContent(http.StatusNotFound)
		}
		return ctx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to query run",
		})
	}

	// No additional permission check needed - middleware already verified
	// that the user has "read" permission on this specific run
	return ctx.JSON(http.StatusOK, formatRunResponse(run))
}

// ApiRunsCancelWithKessel demonstrates canceling a run with Kessel
// The middleware has already verified "cancel" permission on this run
func (this *kesselControllers) ApiRunsCancelWithKessel(ctx echo.Context, runID string) error {
	identity := identityMiddleware.GetIdentity(ctx.Request().Context())

	// Find the run
	var run dbModel.Run
	result := this.database.
		Where("id = ? AND account = ?", runID, identity.Identity.OrgID).
		First(&run)

	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return ctx.NoContent(http.StatusNotFound)
		}
		return ctx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to query run",
		})
	}

	// Check if run can be canceled (business logic, not authorization)
	if run.Status != "running" {
		return ctx.JSON(http.StatusBadRequest, map[string]string{
			"error": "can only cancel running jobs",
		})
	}

	// Update run status
	run.Status = "canceled"
	if err := this.database.Save(&run).Error; err != nil {
		return ctx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to cancel run",
		})
	}

	return ctx.JSON(http.StatusOK, formatRunResponse(run))
}

// Helper function for response formatting (placeholder)
func formatRunsResponse(runs []dbModel.Run) interface{} {
	// Implement response formatting
	return runs
}

func formatRunResponse(run dbModel.Run) interface{} {
	// Implement response formatting
	return run
}

func getFields(fields *[]ApiRunsListParamsFields) []string {
	// Implement field selection
	return []string{"*"}
}

func applyFilters(query *gorm.DB, filter *ApiRunsListParamsFilter) *gorm.DB {
	// Implement filtering logic
	return query
}
