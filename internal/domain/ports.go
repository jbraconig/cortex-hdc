package domain

import "context"

// Encoder defines how to transform text into an HVector
type Encoder interface {
	EncodeLine(kb *KnowledgeBase, line string) HVector
}

// LogReader defines a producer of continuous logs
type LogReader interface {
	ReadLogs(ctx context.Context, filepath string) (<-chan string, error)
}

// Persistence defines how to save and retrieve knowledge (KnowledgeBase)
type Persistence interface {
	Save(kb *KnowledgeBase, filepath string) error
	Load(filepath string) (*KnowledgeBase, error)
}

// AlertNotifier defines how to dispatch alerts when an anomaly is found
type AlertNotifier interface {
	Notify(logLine string, similarity float64) error
}
