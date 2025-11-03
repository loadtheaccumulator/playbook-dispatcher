package kessel

import (
	"context"
	"fmt"
)

// AttributeFilter represents filtering based on resource attributes
type AttributeFilter struct {
	// Key is the attribute name (e.g., "service")
	Key string

	// Operation is the filter operation ("equal", "in")
	Operation string

	// Values are the allowed values for this attribute
	Values []string
}

// GetAttributeFilters retrieves attribute filters from Kessel for a subject
// This is used to implement service-scoped permissions similar to RBAC
func GetAttributeFilters(ctx context.Context, client KesselClient, subject Subject, relation string, resourceType string, attributeKey string) ([]string, error) {
	// Note: This is a placeholder for Kessel's attribute-based filtering
	// The actual implementation depends on how Kessel handles conditional relationships

	// Option 1: Use Kessel's ListResources with attribute filters
	// Option 2: Store attribute filters as resource relationship metadata
	// Option 3: Use Kessel's policy engine for attribute-based conditions

	// For now, return empty list (no filtering)
	// This needs to be implemented based on your Kessel deployment's capabilities
	return []string{}, fmt.Errorf("attribute filtering not yet implemented in Kessel")
}

// CheckWithAttributes performs an authorization check with attribute filtering
// This supports cases like "user can read runs WHERE service IN ['remediations']"
func CheckWithAttributes(ctx context.Context, client KesselClient, check ResourceCheck, attributes map[string]string) (bool, error) {
	// Basic check first
	allowed, err := client.Check(ctx, check)
	if err != nil {
		return false, err
	}

	if !allowed {
		return false, nil
	}

	// Additional attribute filtering would go here
	// This depends on Kessel's support for conditional relationships

	return true, nil
}

// FilterByServicePermission filters resources based on service attribute permissions
// This replicates the RBAC service filtering functionality
func FilterByServicePermission(ctx context.Context, client KesselClient, subject Subject, relation string, resourceType string) (allowedServices []string, filteringRequired bool, err error) {
	// This would query Kessel for service-scoped permissions
	// For example: "user:jdoe can read playbook-dispatcher/run WHERE service = 'remediations'"

	// Kessel implementation options:
	// 1. Store service permissions as separate relationship tuples
	//    e.g., (user:jdoe, read-service-remediations, org:123)
	// 2. Use Kessel's attribute filtering in the check API
	// 3. Store allowed services as resource metadata

	// For now, return no filtering (allows all services)
	return nil, false, nil
}

// ApplyServiceFilter is a helper function for controllers to apply service filtering
// This maintains compatibility with the existing RBAC service filtering
func ApplyServiceFilter(ctx context.Context, client KesselClient, subject Subject) (allowedServices []string, shouldFilter bool, err error) {
	// Get allowed services from Kessel
	services, filtering, err := FilterByServicePermission(
		ctx,
		client,
		subject,
		RelationRead,
		ResourceTypeRun,
	)

	if err != nil {
		return nil, false, err
	}

	// If no specific services are returned, user has access to all services
	if !filtering || len(services) == 0 {
		return nil, false, nil
	}

	return services, true, nil
}
