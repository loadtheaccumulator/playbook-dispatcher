package kessel

import "context"

// ServicePermissionResourceType is the Kessel resource type for service-scoped permissions
const ServicePermissionResourceType = "playbook-dispatcher/service-permission"

// ServicePermissionRelationReader is the relation for reading runs from a service
const ServicePermissionRelationReader = "reader"

// KnownServices lists all services that can create playbook runs
var KnownServices = []string{
	"remediations",
	"config-manager",
	"vulnerability",
	"advisor",
	"compliance",
	"drift",
	"policies",
	"resource-optimization",
}

// GetAllowedServices returns the list of services a subject can access
// This replicates RBAC's service attribute filtering
// Returns (services, filterRequired, error)
// - If filterRequired is false, user has access to all services (no filtering needed)
// - If filterRequired is true, only returned services are allowed
func GetAllowedServices(ctx context.Context, client KesselClient, subject Subject) ([]string, bool, error) {
	// First, check if user has org-level access (bypasses service filtering)
	orgCheck := ResourceCheck{
		Subject:  subject,
		Relation: RelationRead,
		Resource: Resource{
			Type:   ResourceTypeOrg,
			ID:     subject.Tenant,
			Tenant: subject.Tenant,
		},
	}

	hasOrgAccess, err := client.Check(ctx, orgCheck)
	if err != nil {
		return nil, false, err
	}

	if hasOrgAccess {
		// User has org-level access, no service filtering required
		return nil, false, nil
	}

	// User doesn't have org-level access, check service-specific permissions
	return getServicePermissions(ctx, client, subject)
}

// getServicePermissions checks which services the subject has access to
func getServicePermissions(ctx context.Context, client KesselClient, subject Subject) ([]string, bool, error) {
	// Build checks for all known services
	checks := make([]ResourceCheck, len(KnownServices))
	for i, service := range KnownServices {
		checks[i] = ResourceCheck{
			Subject:  subject,
			Relation: ServicePermissionRelationReader,
			Resource: Resource{
				Type:   ServicePermissionResourceType,
				ID:     service,
				Tenant: subject.Tenant,
			},
		}
	}

	// Perform batch check
	results, err := client.CheckBatch(ctx, checks)
	if err != nil {
		return nil, false, err
	}

	// Collect allowed services
	allowedServices := make([]string, 0)
	for i, allowed := range results {
		if allowed {
			allowedServices = append(allowedServices, KnownServices[i])
		}
	}

	// If no services are allowed, user has no access
	if len(allowedServices) == 0 {
		return nil, true, nil // Filter required, but empty list (no access)
	}

	return allowedServices, true, nil
}

// HasServiceAccess checks if a subject has access to a specific service
func HasServiceAccess(ctx context.Context, client KesselClient, subject Subject, service string) (bool, error) {
	// Check org-level access first
	orgCheck := ResourceCheck{
		Subject:  subject,
		Relation: RelationRead,
		Resource: Resource{
			Type:   ResourceTypeOrg,
			ID:     subject.Tenant,
			Tenant: subject.Tenant,
		},
	}

	hasOrgAccess, err := client.Check(ctx, orgCheck)
	if err != nil {
		return false, err
	}

	if hasOrgAccess {
		return true, nil
	}

	// Check service-specific access
	serviceCheck := ResourceCheck{
		Subject:  subject,
		Relation: ServicePermissionRelationReader,
		Resource: Resource{
			Type:   ServicePermissionResourceType,
			ID:     service,
			Tenant: subject.Tenant,
		},
	}

	return client.Check(ctx, serviceCheck)
}
