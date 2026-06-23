package domain

// TelemetryClient defines the interface for reporting anomaly metrics to a central Control Plane.
type TelemetryClient interface {
	ReportAnomaly(nodeID string, score float64, timestamp int64, hdcVector []byte)
	Close() error
}
