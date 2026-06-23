package usecase

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/jbraconig/cortex-hdc/internal/domain"
)

type mockEncoder struct{}

func (m *mockEncoder) EncodeLine(kb *domain.KnowledgeBase, line string) domain.HVector {
	// Return a static vector for predictability
	return domain.GenerateRandomVector()
}

type mockLogReader struct {
	lines []string
}

func (m *mockLogReader) ReadLogs(ctx context.Context, filePaths []string) (<-chan string, error) {
	ch := make(chan string, len(m.lines))
	for _, l := range m.lines {
		ch <- l
	}
	close(ch)
	return ch, nil
}

type mockNotifier struct{}

func (m *mockNotifier) Notify(logLine string, similarity float64) error {
	return nil
}

type mockPersistence struct{}

func (m *mockPersistence) Save(kb *domain.KnowledgeBase, filepath string) error {
	return nil
}

func (m *mockPersistence) Load(filepath string) (*domain.KnowledgeBase, error) {
	return domain.NewKnowledgeBase(), nil
}

type mockClusterSync struct {
	mu           sync.Mutex
	broadcasts   []domain.HVector
	decayRates   []float64
	broadcastChan chan bool
}

func (m *mockClusterSync) BroadcastBaseline(vec domain.HVector, decayRate float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.broadcasts = append(m.broadcasts, vec)
	m.decayRates = append(m.decayRates, decayRate)
	if m.broadcastChan != nil {
		select {
		case m.broadcastChan <- true:
		default:
		}
	}
	return nil
}

func (m *mockClusterSync) NodeName() string {
	return "mock-node"
}

func (m *mockClusterSync) Shutdown() error {
	return nil
}

func TestInferenceDecayBroadcast(t *testing.T) {
	kb := domain.NewKnowledgeBase()
	// Set baseline to random vector
	kb.Baseline = domain.GenerateRandomVector()

	syncChan := make(chan bool, 1)
	mSync := &mockClusterSync{
		broadcastChan: syncChan,
	}

	reader := &mockLogReader{
		lines: []string{"healthy log line 1"},
	}

	// Threshold is very low (e.g. 0.0) so the similarity is always >= threshold (i.e. healthy)
	inf := NewInference(
		&mockEncoder{},
		reader,
		&mockNotifier{},
		&mockPersistence{},
		0.0,
		false,
		0.01, // decayRate > 0
		mSync,
		nil,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := inf.Run(ctx, kb, []string{"dummy.log"}, 1, "")
	if err != nil {
		t.Fatalf("unexpected error running inference: %v", err)
	}

	// Wait for async broadcast to finish
	select {
	case <-syncChan:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for async P2P broadcast")
	}

	mSync.mu.Lock()
	defer mSync.mu.Unlock()
	if len(mSync.broadcasts) != 1 {
		t.Errorf("expected 1 broadcast, got %d", len(mSync.broadcasts))
	}
	if mSync.decayRates[0] != 0.01 {
		t.Errorf("expected decay rate 0.01, got %f", mSync.decayRates[0])
	}
}
