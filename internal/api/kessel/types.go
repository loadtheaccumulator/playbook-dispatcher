package kessel

import "context"

// KesselClient defines the interface for Kessel authorization checks
type KesselClient interface {
	// Check performs an authorization check for a single resource
	Check(ctx context.Context, check ResourceCheck) (bool, error)

	// CheckBatch performs authorization checks for multiple resources
	CheckBatch(ctx context.Context, checks []ResourceCheck) ([]bool, error)

	// ListResources returns a list of resource IDs the subject can access with the given relation
	ListResources(ctx context.Context, subject Subject, relation string, resourceType string) ([]string, error)
}

// Subject represents the entity requesting access (user, service account, etc)
type Subject struct {
	// Type is the subject type (e.g., "user", "service_account")
	Type string

	// ID is the unique identifier for the subject
	ID string

	// Tenant is the organization/account ID
	Tenant string
}

// Resource represents a resource being accessed
type Resource struct {
	// Type is the resource type (e.g., "playbook-dispatcher/run")
	Type string

	// ID is the unique identifier for the resource
	ID string

	// Tenant is the organization/account ID that owns the resource
	Tenant string
}

// ResourceCheck represents a single authorization check
type ResourceCheck struct {
	// Subject is who is requesting access
	Subject Subject

	// Relation is the permission being checked (e.g., "read", "write", "execute")
	Relation string

	// Resource is what is being accessed
	Resource Resource
}

// CheckResult represents the result of an authorization check
type CheckResult struct {
	Allowed bool
	Error   error
}

// Common resource types for playbook-dispatcher
const (
	ResourceTypeRun       = "playbook-dispatcher/run"
	ResourceTypeRunHost   = "playbook-dispatcher/run_host"
	ResourceTypeOrg       = "playbook-dispatcher/org"
)

// Common relations/permissions
const (
	RelationRead    = "read"
	RelationWrite   = "write"
	RelationExecute = "execute"
	RelationDelete  = "delete"
	RelationCancel  = "cancel"
)

// Subject types
const (
	SubjectTypeUser           = "user"
	SubjectTypeServiceAccount = "service_account"
)

// DispatcherResourceCheck is a helper to create ResourceCheck for playbook-dispatcher resources
func DispatcherResourceCheck(subject Subject, relation string, resourceType string, resourceID string) ResourceCheck {
	return ResourceCheck{
		Subject:  subject,
		Relation: relation,
		Resource: Resource{
			Type:   resourceType,
			ID:     resourceID,
			Tenant: subject.Tenant,
		},
	}
}

// DispatcherRunCheck creates a check for run resources
func DispatcherRunCheck(subject Subject, relation string, runID string) ResourceCheck {
	return DispatcherResourceCheck(subject, relation, ResourceTypeRun, runID)
}

// DispatcherRunHostCheck creates a check for run_host resources
func DispatcherRunHostCheck(subject Subject, relation string, runHostID string) ResourceCheck {
	return DispatcherResourceCheck(subject, relation, ResourceTypeRunHost, runHostID)
}
