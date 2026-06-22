package domain

// PeerMessage defines the payload broadcasted over the Gossip protocol.
type PeerMessage struct {
	Vector HVector `json:"vector"`
	Decay  float64 `json:"decay"`
}

// ClusterSync defines the P2P synchronization operations.
type ClusterSync interface {
	// BroadcastBaseline sends the local vector and decay rate update to other nodes.
	BroadcastBaseline(vec HVector, decayRate float64) error
	// Shutdown gracefully stops cluster networking.
	Shutdown() error
}
