package p2p

import (
	"encoding/json"
	"fmt"
	"log"
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
	var pm domain.PeerMessage
	if err := json.Unmarshal(msg, &pm); err != nil {
		log.Printf("[P2P] Failed to parse cluster update: %v", err)
		return
	}

	d.node.mu.Lock()
	defer d.node.mu.Unlock()

	log.Printf("[P2P] Received baseline update from cluster. Mixing vector (decay: %.4f)", pm.Decay)
	if len(d.node.kb.Baselines) > 0 {
		bestIdx := domain.AssignToCluster(pm.Vector, d.node.kb.Baselines)
		d.node.kb.Baselines[bestIdx] = domain.DecayBlend(d.node.kb.Baselines[bestIdx], pm.Vector, pm.Decay)
	} else {
		d.node.kb.Baseline = domain.DecayBlend(d.node.kb.Baseline, pm.Vector, pm.Decay)
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
	config.Name = fmt.Sprintf("%s-%d", hostname, bindPort)

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
	pm := domain.PeerMessage{
		Vector: vec,
		Decay:  decayRate,
	}

	data, err := json.Marshal(pm)
	if err != nil {
		return fmt.Errorf("failed to marshal peer message: %w", err)
	}

	g.broadcasts.QueueBroadcast(&peerBroadcast{msg: data})
	return nil
}

func (g *GossipNode) Shutdown() error {
	log.Println("[P2P] Shutting down P2P cluster node...")
	if g.list != nil {
		_ = g.list.Leave(10)
		return g.list.Shutdown()
	}
	return nil
}
