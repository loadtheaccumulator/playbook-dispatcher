package kessel

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	checkv1 "github.com/project-kessel/relations-api/api/kessel/relations/v1"
	"github.com/redhatinsights/platform-go-middlewares/request_id"
	"playbook-dispatcher/internal/common/constants"
)

// clientImpl implements the KesselClient interface using gRPC
type clientImpl struct {
	checkClient checkv1.KesselCheckServiceClient
	conn        *grpc.ClientConn
}

// NewKesselClient creates a new Kessel client from configuration
func NewKesselClient(cfg *viper.Viper) (KesselClient, error) {
	hostname := cfg.GetString("kessel.hostname")
	port := cfg.GetInt("kessel.port")
	insecureConn := cfg.GetBool("kessel.insecure")
	timeout := cfg.GetDuration("kessel.timeout") * time.Second

	address := fmt.Sprintf("%s:%d", hostname, port)

	var opts []grpc.DialOption
	if insecureConn {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	opts = append(opts, grpc.WithTimeout(timeout))

	conn, err := grpc.Dial(address, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Kessel at %s: %w", address, err)
	}

	checkClient := checkv1.NewKesselCheckServiceClient(conn)

	return &clientImpl{
		checkClient: checkClient,
		conn:        conn,
	}, nil
}

// Close closes the gRPC connection
func (c *clientImpl) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// Check performs a single authorization check
func (c *clientImpl) Check(ctx context.Context, check ResourceCheck) (bool, error) {
	// Add request metadata (request ID, identity)
	ctx = c.addRequestMetadata(ctx)

	req := &checkv1.CheckRequest{
		Subject: &checkv1.SubjectReference{
			Subject: &checkv1.ObjectReference{
				Type: &checkv1.ObjectType{
					Name: check.Subject.Type,
				},
				Id: check.Subject.ID,
			},
		},
		Relation: check.Relation,
		Resource: &checkv1.ObjectReference{
			Type: &checkv1.ObjectType{
				Name: check.Resource.Type,
			},
			Id: check.Resource.ID,
		},
	}

	resp, err := c.checkClient.Check(ctx, req)
	if err != nil {
		return false, fmt.Errorf("kessel check failed: %w", err)
	}

	return resp.Allowed == checkv1.CheckResponse_ALLOWED_TRUE, nil
}

// CheckBatch performs multiple authorization checks in a single request
func (c *clientImpl) CheckBatch(ctx context.Context, checks []ResourceCheck) ([]bool, error) {
	// Add request metadata
	ctx = c.addRequestMetadata(ctx)

	// Convert to batch request
	items := make([]*checkv1.CheckRequest, len(checks))
	for i, check := range checks {
		items[i] = &checkv1.CheckRequest{
			Subject: &checkv1.SubjectReference{
				Subject: &checkv1.ObjectReference{
					Type: &checkv1.ObjectType{
						Name: check.Subject.Type,
					},
					Id: check.Subject.ID,
				},
			},
			Relation: check.Relation,
			Resource: &checkv1.ObjectReference{
				Type: &checkv1.ObjectType{
					Name: check.Resource.Type,
				},
				Id: check.Resource.ID,
			},
		}
	}

	// Kessel doesn't have a native batch API, so we'll make individual calls
	// In production, consider using goroutines for parallel execution
	results := make([]bool, len(checks))
	for i, item := range items {
		resp, err := c.checkClient.Check(ctx, item)
		if err != nil {
			return nil, fmt.Errorf("kessel batch check failed at index %d: %w", i, err)
		}
		results[i] = resp.Allowed == checkv1.CheckResponse_ALLOWED_TRUE
	}

	return results, nil
}

// ListResources returns resource IDs the subject can access
// Note: This is a placeholder - Kessel's ListResources API may differ
func (c *clientImpl) ListResources(ctx context.Context, subject Subject, relation string, resourceType string) ([]string, error) {
	// Add request metadata
	ctx = c.addRequestMetadata(ctx)

	// This would use Kessel's list/lookup API when available
	// For now, this is a placeholder that would need to be implemented
	// based on the actual Kessel API
	return nil, fmt.Errorf("ListResources not yet implemented")
}

// addRequestMetadata adds request ID and identity headers to the gRPC context
func (c *clientImpl) addRequestMetadata(ctx context.Context) context.Context {
	md := metadata.MD{}

	// Add request ID if available
	if reqID := request_id.GetReqID(ctx); reqID != "" {
		md.Set("x-rh-insights-request-id", reqID)
	}

	// Add identity header if available
	if identity, ok := ctx.Value(constants.HeaderIdentity).(string); ok {
		md.Set("x-rh-identity", identity)
	}

	return metadata.NewOutgoingContext(ctx, md)
}
