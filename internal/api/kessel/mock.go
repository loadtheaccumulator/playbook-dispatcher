package kessel

import (
	"context"
)

// mockClient is a mock implementation of KesselClient for testing
type mockClient struct {
	allowAll bool
}

// NewMockKesselClient creates a mock Kessel client
// If allowAll is true, all checks return true (allowed)
func NewMockKesselClient(allowAll bool) KesselClient {
	return &mockClient{
		allowAll: allowAll,
	}
}

func (m *mockClient) Check(ctx context.Context, check ResourceCheck) (bool, error) {
	return m.allowAll, nil
}

func (m *mockClient) CheckBatch(ctx context.Context, checks []ResourceCheck) ([]bool, error) {
	results := make([]bool, len(checks))
	for i := range results {
		results[i] = m.allowAll
	}
	return results, nil
}

func (m *mockClient) ListResources(ctx context.Context, subject Subject, relation string, resourceType string) ([]string, error) {
	if m.allowAll {
		return []string{"*"}, nil
	}
	return []string{}, nil
}
