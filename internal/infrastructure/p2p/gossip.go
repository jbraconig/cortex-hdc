package p2p

import (
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"sync"

	"github.com/hashicorp/memberlist"
	"github.com/jbraconig/cortex-hdc/internal/domain"
)

type GossipNode struct {
	list       *memberlist.Memberlist
	kb         *domain.KnowledgeBase
	mu         sync.Mutex
	broadcasts *memberlist.TransmitLimitedQueue
}

// gossipDelegate implements memberlist.Delegate
type gossipDelegate struct {
	node *GossipNode
}

func (d *gossipDelegate) NodeMeta(limit int) []byte {
	return nil
}

func (d *gossipDelegate) NotifyMsg(msg []byte) {
	vec, decay, err := unmarshalPeerMessage(msg)
	if err != nil {
		log.Printf("[P2P] Failed to parse cluster update: %v", err)
		return
	}

	d.node.mu.Lock()
	defer d.node.mu.Unlock()

	log.Printf("[P2P] Received baseline update from cluster. Mixing vector (decay: %.4f)", decay)
	if len(d.node.kb.Baselines) > 0 {
		bestIdx := domain.AssignToCluster(vec, d.node.kb.Baselines)
		d.node.kb.Baselines[bestIdx] = domain.DecayBlend(d.node.kb.Baselines[bestIdx], vec, decay)
	} else {
		d.node.kb.Baseline = domain.DecayBlend(d.node.kb.Baseline, vec, decay)
	}
}

func (d *gossipDelegate) GetBroadcasts(overhead, limit int) [][]byte {
	return d.node.broadcasts.GetBroadcasts(overhead, limit)
}

func (d *gossipDelegate) LocalState(join bool) []byte {
	return nil
}

func (d *gossipDelegate) MergeRemoteState(buf []byte, join bool) {}

// peerBroadcast implements memberlist.Broadcast
type peerBroadcast struct {
	msg []byte
}

func (b *peerBroadcast) Invalidates(other memberlist.Broadcast) bool {
	return false
}

func (b *peerBroadcast) Message() []byte {
	return b.msg
}

func (b *peerBroadcast) Finished() {}

// NewGossipNode creates and starts a new memberlist P2P node.
func NewGossipNode(bindPort int, joinAddrs []string, kb *domain.KnowledgeBase) (*GossipNode, error) {
	node := &GossipNode{
		kb: kb,
	}

	config := memberlist.DefaultLANConfig()
	config.BindPort = bindPort
	config.AdvertisePort = bindPort
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "cortex-node"
	}
	// Use random suffix to prevent memberlist conflicts when a node restarts with a new IP
	config.Name = fmt.Sprintf("%s-%d-%x", hostname, bindPort, rand.Int63())

	delegate := &gossipDelegate{node: node}
	config.Delegate = delegate

	list, err := memberlist.Create(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create memberlist: %w", err)
	}

	node.list = list
	node.broadcasts = &memberlist.TransmitLimitedQueue{
		NumNodes: func() int {
			return list.NumMembers()
		},
		RetransmitMult: 3,
	}

	if len(joinAddrs) > 0 {
		log.Printf("[P2P] Attempting to join cluster at: %v", joinAddrs)
		_, err := list.Join(joinAddrs)
		if err != nil {
			log.Printf("[P2P] Warning: failed to join some peers: %v", err)
		} else {
			log.Printf("[P2P] Successfully joined cluster. Members: %d", list.NumMembers())
		}
	}

	return node, nil
}

func (g *GossipNode) BroadcastBaseline(vec domain.HVector, decayRate float64) error {
	data := marshalPeerMessage(vec, decayRate)
	g.broadcasts.QueueBroadcast(&peerBroadcast{msg: data})
	return nil
}

func marshalPeerMessage(vec domain.HVector, decay float64) []byte {
	// Decay (float64, 8 bytes) + HVector (157 * 8 bytes) = 1264 bytes
	buf := make([]byte, 8+len(vec.Data)*8)
	bits := math.Float64bits(decay)
	binary.BigEndian.PutUint64(buf[0:8], bits)
	for i := 0; i < len(vec.Data); i++ {
		binary.BigEndian.PutUint64(buf[8+i*8:8+(i+1)*8], vec.Data[i])
	}
	return buf
}

func unmarshalPeerMessage(buf []byte) (domain.HVector, float64, error) {
	// Check if buffer size matches what we expect (1264 bytes)
	expectedLen := 8 + 157*8
	if len(buf) < expectedLen {
		return domain.HVector{}, 0, fmt.Errorf("buffer too short: got %d, expected %d", len(buf), expectedLen)
	}
	bits := binary.BigEndian.Uint64(buf[0:8])
	decay := math.Float64frombits(bits)
	var vec domain.HVector
	for i := 0; i < len(vec.Data); i++ {
		vec.Data[i] = binary.BigEndian.Uint64(buf[8+i*8 : 8+(i+1)*8])
	}
	return vec, decay, nil
}

func (g *GossipNode) Shutdown() error {
	log.Println("[P2P] Shutting down P2P cluster node...")
	if g.list != nil {
		_ = g.list.Leave(10)
		return g.list.Shutdown()
	}
	return nil
}
