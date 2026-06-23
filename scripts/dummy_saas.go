package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"

	pb "github.com/jbraconig/cortex-hdc/api/proto/v1"
	"google.golang.org/grpc"
)

type server struct {
	pb.UnimplementedTelemetryServiceServer
}

type ReceivedReport struct {
	Token        string  `json:"token"`
	NodeID       string  `json:"node_id"`
	AnomalyScore float64 `json:"anomaly_score"`
	Timestamp    int64   `json:"timestamp"`
	VectorLen    int     `json:"vector_len"`
}

func (s *server) ReportAnomaly(ctx context.Context, req *pb.AnomalyReportRequest) (*pb.AnomalyReportResponse, error) {
	log.Printf("[DUMMY-SaaS] Received anomaly report from node %s (score: %.4f)", req.NodeId, req.AnomalyScore)

	report := ReceivedReport{
		Token:        req.Token,
		NodeID:       req.NodeId,
		AnomalyScore: req.AnomalyScore,
		Timestamp:    req.Timestamp,
		VectorLen:    len(req.HdcVector),
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return nil, err
	}

	_ = os.MkdirAll("test-data", 0755)
	err = os.WriteFile("test-data/telemetry_received.json", data, 0644)
	if err != nil {
		log.Printf("[DUMMY-SaaS] Error writing file: %v", err)
		return &pb.AnomalyReportResponse{Success: false, Message: err.Error()}, nil
	}

	// Print indicator for test assertion script
	fmt.Println("TELEMETRY_REPORT_SAVED")

	return &pb.AnomalyReportResponse{Success: true, Message: "Report received"}, nil
}

func main() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	pb.RegisterTelemetryServiceServer(s, &server{})

	log.Println("[DUMMY-SaaS] Listening on :50051...")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
