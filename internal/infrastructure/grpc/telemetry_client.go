package grpc

import (
	"context"
	"fmt"
	"log"
	"time"

	pb "github.com/jbraconig/cortex-hdc/api/proto/v1"
	"github.com/jbraconig/cortex-hdc/internal/domain"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var _ domain.TelemetryClient = (*RealTelemetryClient)(nil)
var _ domain.TelemetryClient = (*NoOpTelemetryClient)(nil)

// RealTelemetryClient implements domain.TelemetryClient and communicates with the SaaS Control Plane.
type RealTelemetryClient struct {
	client pb.TelemetryServiceClient
	conn   *grpc.ClientConn
	token  string
}

// NewRealTelemetryClient creates a new gRPC telemetry client.
func NewRealTelemetryClient(endpoint string, token string) (domain.TelemetryClient, error) {
	// Use insecure credentials for this implementation (can be updated to TLS later)
	conn, err := grpc.NewClient(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to dial gRPC endpoint %s: %w", endpoint, err)
	}

	client := pb.NewTelemetryServiceClient(conn)
	return &RealTelemetryClient{
		client: client,
		conn:   conn,
		token:  token,
	}, nil
}

// ReportAnomaly reports an anomaly asynchronously to not block the main logic.
func (c *RealTelemetryClient) ReportAnomaly(nodeID string, score float64, timestamp int64, hdcVector []byte, rawLog string, threshold float64) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		req := &pb.AnomalyReportRequest{
			Token:        c.token,
			NodeId:       nodeID,
			AnomalyScore: score,
			Timestamp:    timestamp,
			HdcVector:    hdcVector,
			RawLog:       rawLog,
			Threshold:    threshold,
		}

		_, err := c.client.ReportAnomaly(ctx, req)
		if err != nil {
			log.Printf("[WARN] Telemetry report failed: %v", err)
		} else {
			log.Printf("[INFO] Telemetry report sent successfully for node %s", nodeID)
		}
	}()
}

// Close closes the gRPC connection.
func (c *RealTelemetryClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// NoOpTelemetryClient implements a no-op TelemetryClient when SaaS is disabled.
type NoOpTelemetryClient struct{}

// NewNoOpTelemetryClient creates a no-op client.
func NewNoOpTelemetryClient() domain.TelemetryClient {
	return &NoOpTelemetryClient{}
}

// ReportAnomaly does nothing.
func (c *NoOpTelemetryClient) ReportAnomaly(nodeID string, score float64, timestamp int64, hdcVector []byte, rawLog string, threshold float64) {}

// Close does nothing.
func (c *NoOpTelemetryClient) Close() error { return nil }
