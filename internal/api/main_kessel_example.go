package api

// This file demonstrates how to integrate Kessel into the main API setup
// It shows route registration with Kessel middleware instead of RBAC

import (
	"playbook-dispatcher/internal/api/controllers/public"
	"playbook-dispatcher/internal/api/kessel"
	"playbook-dispatcher/internal/api/middleware"

	"github.com/labstack/echo/v4"
	identityMiddleware "github.com/redhatinsights/platform-go-middlewares/identity"
	"github.com/spf13/viper"
)

// Example of setting up routes with Kessel middleware
// This would be part of your main.go or route setup

func SetupRoutesWithKessel(e *echo.Echo, cfg *viper.Viper, controllers *public.Controllers) error {
	// Initialize Kessel configuration
	kessel.ConfigureDefaults(cfg)

	// Create resource extractors for different endpoints
	runResourceExtractor := func(c echo.Context) (kessel.Resource, error) {
		runID := c.Param("run_id")
		identity := identityMiddleware.GetIdentity(c.Request().Context())

		return kessel.Resource{
			Type:   kessel.ResourceTypeRun,
			ID:     runID,
			Tenant: identity.Identity.OrgID,
		}, nil
	}

	runHostResourceExtractor := func(c echo.Context) (kessel.Resource, error) {
		runHostID := c.Param("run_host_id")
		identity := identityMiddleware.GetIdentity(c.Request().Context())

		return kessel.Resource{
			Type:   kessel.ResourceTypeRunHost,
			ID:     runHostID,
			Tenant: identity.Identity.OrgID,
		}, nil
	}

	// Public API routes
	api := e.Group("/api/playbook-dispatcher/v1")

	// Apply identity middleware to all routes
	api.Use(identityMiddleware.EnforceIdentity)

	// --- RUN ENDPOINTS ---

	// List runs - requires org-level read permission
	api.GET("/runs",
		controllers.ApiRunsList,
		middleware.EnforceKesselOrgPermissions(cfg, kessel.RelationRead, kessel.ResourceTypeRun))

	// Get specific run - requires read permission on specific run
	api.GET("/runs/:run_id",
		controllers.ApiRunsGet,
		middleware.EnforceKesselPermissions(cfg, kessel.RelationRead, runResourceExtractor))

	// Create run - requires write permission at org level
	api.POST("/runs",
		controllers.ApiRunsCreate,
		middleware.EnforceKesselOrgPermissions(cfg, kessel.RelationWrite, kessel.ResourceTypeRun))

	// Cancel run - requires cancel permission on specific run
	api.POST("/runs/:run_id/cancel",
		controllers.ApiRunsCancel,
		middleware.EnforceKesselPermissions(cfg, kessel.RelationCancel, runResourceExtractor))

	// --- RUN HOST ENDPOINTS ---

	// List run hosts - requires read permission on parent run
	// Uses the same run extractor since run_id is in the path
	api.GET("/runs/:run_id/hosts",
		controllers.ApiRunHostsList,
		middleware.EnforceKesselPermissions(cfg, kessel.RelationRead, runResourceExtractor))

	// Get specific run host - requires read permission on run host
	api.GET("/run_hosts/:run_host_id",
		controllers.ApiRunHostsGet,
		middleware.EnforceKesselPermissions(cfg, kessel.RelationRead, runHostResourceExtractor))

	return nil
}

// Example of setting up with both RBAC and Kessel for gradual migration
func SetupRoutesWithBothRBACAndKessel(e *echo.Echo, cfg *viper.Viper, controllers *public.Controllers) error {
	// Check if Kessel is enabled
	kesselEnabled := cfg.GetBool("kessel.enabled")

	api := e.Group("/api/playbook-dispatcher/v1")
	api.Use(identityMiddleware.EnforceIdentity)

	if kesselEnabled {
		// Use Kessel middleware
		runExtractor := func(c echo.Context) (kessel.Resource, error) {
			runID := c.Param("run_id")
			identity := identityMiddleware.GetIdentity(c.Request().Context())
			return kessel.Resource{
				Type:   kessel.ResourceTypeRun,
				ID:     runID,
				Tenant: identity.Identity.OrgID,
			}, nil
		}

		api.GET("/runs",
			controllers.ApiRunsList,
			middleware.EnforceKesselOrgPermissions(cfg, kessel.RelationRead, kessel.ResourceTypeRun))

		api.GET("/runs/:run_id",
			controllers.ApiRunsGet,
			middleware.EnforceKesselPermissions(cfg, kessel.RelationRead, runExtractor))
	} else {
		// Use legacy RBAC middleware
		// Note: This requires importing the old rbac package
		// import "playbook-dispatcher/internal/api/rbac"

		/*
		api.GET("/runs",
			controllers.ApiRunsList,
			middleware.EnforcePermissions(cfg, rbac.DispatcherPermission("run", "read")))

		api.GET("/runs/:run_id",
			controllers.ApiRunsGet,
			middleware.EnforcePermissions(cfg, rbac.DispatcherPermission("run", "read")))
		*/
	}

	return nil
}

// Example initialization function for the API server with Kessel
func InitializeAPIServerWithKessel(cfg *viper.Viper) (*echo.Echo, error) {
	e := echo.New()

	// Initialize Kessel
	kessel.ConfigureDefaults(cfg)

	// Create Kessel client (stored somewhere accessible to controllers)
	kesselClient, err := kessel.NewKesselClientFromConfig(cfg)
	if err != nil {
		return nil, err
	}

	// Note: In real implementation, you'd pass kesselClient to controllers
	// or store it in a way that controllers can access it

	// Initialize controllers
	// controllers := public.NewControllers(db, kesselClient)

	// Setup routes
	// SetupRoutesWithKessel(e, cfg, controllers)

	return e, nil
}
