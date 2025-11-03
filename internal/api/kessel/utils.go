package kessel

import (
	"context"
	"fmt"

	identityMiddleware "github.com/redhatinsights/platform-go-middlewares/identity"
)

// SubjectFromContext extracts a Subject from the request context using identity middleware
func SubjectFromContext(ctx context.Context) (Subject, error) {
	identity := identityMiddleware.GetIdentity(ctx)
	if identity.Identity.OrgID == "" {
		return Subject{}, fmt.Errorf("missing org_id in identity")
	}

	if identity.Identity.User.Username == "" {
		return Subject{}, fmt.Errorf("missing username in identity")
	}

	return Subject{
		Type:   SubjectTypeUser,
		ID:     identity.Identity.User.Username,
		Tenant: identity.Identity.OrgID,
	}, nil
}

// ServiceAccountSubject creates a Subject for service accounts
func ServiceAccountSubject(serviceAccountID string, orgID string) Subject {
	return Subject{
		Type:   SubjectTypeServiceAccount,
		ID:     serviceAccountID,
		Tenant: orgID,
	}
}

// CheckRunAccess is a helper to check if a subject can access a run
func CheckRunAccess(ctx context.Context, client KesselClient, subject Subject, runID string, relation string) (bool, error) {
	check := DispatcherRunCheck(subject, relation, runID)
	return client.Check(ctx, check)
}

// CheckRunHostAccess is a helper to check if a subject can access a run host
func CheckRunHostAccess(ctx context.Context, client KesselClient, subject Subject, runHostID string, relation string) (bool, error) {
	check := DispatcherRunHostCheck(subject, relation, runHostID)
	return client.Check(ctx, check)
}

// FilterAuthorizedResources filters a list of resource IDs based on authorization
// This is useful for list endpoints where you need to filter results by permission
func FilterAuthorizedResources(ctx context.Context, client KesselClient, subject Subject, relation string, resourceType string, resourceIDs []string) ([]string, error) {
	if len(resourceIDs) == 0 {
		return []string{}, nil
	}

	// Build checks for all resources
	checks := make([]ResourceCheck, len(resourceIDs))
	for i, id := range resourceIDs {
		checks[i] = DispatcherResourceCheck(subject, relation, resourceType, id)
	}

	// Perform batch check
	results, err := client.CheckBatch(ctx, checks)
	if err != nil {
		return nil, err
	}

	// Filter to only allowed resources
	authorized := make([]string, 0, len(resourceIDs))
	for i, allowed := range results {
		if allowed {
			authorized = append(authorized, resourceIDs[i])
		}
	}

	return authorized, nil
}
