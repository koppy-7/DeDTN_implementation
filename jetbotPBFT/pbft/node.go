package pbft

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ---------- Config & Node definition ----------

type Config struct {
	NodeID    string            `json:"node_id"`
	Address   string            `json:"address"`
	Peers     map[string]string `json:"peers"`
	IsPrimary bool              `json:"is_primary"`
}

type Node struct {
	cfg Config
	mu  sync.Mutex //排他的処理

	// State tracking
	state                 map[string]Phase
	result                map[string]string
	prepareVoters         map[string]map[string]bool
	commitVoters          map[string]map[string]bool
	preparedOnce          map[string]bool
	committedOnce         map[string]bool
	payloadCache          map[string][]byte
	startTS               map[string]time.Time //communication measurement for PBFT
	pbftStartPayloadBytes map[string]int
	roundStartMs          map[string]int64
	roundMu               sync.Mutex
}

// ---------- Node constructor ----------

func NewNode(cfg Config) *Node {
	return &Node{
		cfg:                   cfg,
		state:                 make(map[string]Phase),
		result:                make(map[string]string),
		prepareVoters:         make(map[string]map[string]bool),
		commitVoters:          make(map[string]map[string]bool),
		preparedOnce:          make(map[string]bool),
		committedOnce:         make(map[string]bool),
		payloadCache:          make(map[string][]byte),
		startTS:               make(map[string]time.Time), //communication measurement for PBFT
		pbftStartPayloadBytes: make(map[string]int),
		roundStartMs:          make(map[string]int64),
	}
}

// ---------- Start server ----------

func (n *Node) Start() {
	http.HandleFunc("/message", n.handleMessage)
	go func() {
		fmt.Printf("[%s] Node running at %s\n", n.cfg.NodeID, n.cfg.Address)
		if err := http.ListenAndServe(n.cfg.Address, nil); err != nil {
			panic(err)
		}
	}()
}

// ---------- Message handling ----------

func (n *Node) handleMessage(w http.ResponseWriter, r *http.Request) {
	var msg Message
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		w.WriteHeader(400)
		return
	}
	fmt.Printf("[%s] Received %s from %s\n", n.cfg.NodeID, msg.Type, msg.Sender)
	n.processMessage(msg)
}

// ---------- Proposal from Primary ----------

func (n *Node) Propose(sceneHash string, payload []byte) {
	n.payloadCache[sceneHash] = payload

	if _, ok := n.prepareVoters[sceneHash]; !ok {
		n.prepareVoters[sceneHash] = make(map[string]bool)
	}
	n.prepareVoters[sceneHash][n.cfg.NodeID] = true

	msg := Message{Type: PrePrepare, SceneHash: sceneHash, Sender: n.cfg.NodeID, Payload: payload}
	for peerID, addr := range n.cfg.Peers {
		go sendMessage(addr, msg)
		fmt.Printf("[%s] Sent PrePrepare to %s\n", n.cfg.NodeID, peerID)
	}
	prepareMsg := Message{
		Type:      Prepare,
		SceneHash: sceneHash,
		Sender:    n.cfg.NodeID,
	}
	for peerID, addr := range n.cfg.Peers {
		go sendMessage(addr, prepareMsg)
		fmt.Printf("[%s] Sent Prepare to %s\n", n.cfg.NodeID, peerID)
	}

	// count own prepare
	if _, ok := n.prepareVoters[sceneHash]; !ok {
		n.prepareVoters[sceneHash] = make(map[string]bool)
	}
	n.prepareVoters[sceneHash][n.cfg.NodeID] = true
}

// ---------- Quorum helper ----------

// quorumSize = 2f + 1 (for N >= 3f + 1)
func (n *Node) quorumSize() int {
	N := len(n.cfg.Peers) + 1 // include self
	f := (N - 1) / 3
	q := 2*f + 1
	return q
}

// ---------- PBFT core logic ----------

func (n *Node) processMessage(msg Message) {
	n.mu.Lock()
	defer n.mu.Unlock()

	scene := msg.SceneHash
	q := n.quorumSize()

	switch msg.Type {

	// --------- Phase 1: PrePrepare ---------
	case PrePrepare:
		if n.result[scene] == "COMMITTED" {
			return
		}

		if msg.Payload != nil {
			n.payloadCache[scene] = msg.Payload
		}

		if !VerifySemantic(msg.Payload) {
			fmt.Printf("[%s] Semantic verification failed\n", n.cfg.NodeID)
			return
		}
		fmt.Printf("[%s] Semantic verified OK\n", n.cfg.NodeID)

		// まだ self-prepare していないときだけ送る
		if _, ok := n.prepareVoters[scene]; !ok {
			n.prepareVoters[scene] = make(map[string]bool)
		}
		if n.prepareVoters[scene][n.cfg.NodeID] {
			// すでに自分の Prepare を出しているなら何もしない
			return
		}

		// self-prepare を記録
		n.prepareVoters[scene][n.cfg.NodeID] = true

		// 他ノードへ PREPARE を送信
		n.broadcast(Prepare, scene)

	// --------- Phase 2: Prepare ---------
	case Prepare:
		if _, ok := n.prepareVoters[scene]; !ok {
			n.prepareVoters[scene] = make(map[string]bool)
		}
		if !n.prepareVoters[scene][msg.Sender] {
			n.prepareVoters[scene][msg.Sender] = true
		}

		// Check if enough PREPARE votes received
		if len(n.prepareVoters[scene]) >= q && !n.preparedOnce[scene] {

			// Mark as prepared to avoid re-broadcasting
			n.preparedOnce[scene] = true

			// Broadcast COMMIT
			n.broadcast(Commit, scene)

			// Count self-commit
			if _, ok := n.commitVoters[scene]; !ok {
				n.commitVoters[scene] = make(map[string]bool)
			}
			n.commitVoters[scene][n.cfg.NodeID] = true

			fmt.Printf("[%s] PREPARE quorum reached → sending COMMIT (%d/%d)\n",
				n.cfg.NodeID, len(n.prepareVoters[scene]), q)

		} //else {
		// 	fmt.Printf("[%s] PREPARE votes: %d/%d (not enough yet)\n",
		// 		n.cfg.NodeID, len(n.prepareVoters[scene]), q)
		// }

	// --------- Phase 3: Commit ---------
	case Commit:
		if _, exists := n.commitVoters[scene]; !exists {
			n.commitVoters[scene] = make(map[string]bool)
			n.commitVoters[scene][n.cfg.NodeID] = true
		}
		n.commitVoters[scene][msg.Sender] = true

		if len(n.commitVoters[scene]) >= q && !n.committedOnce[scene] {
			n.committedOnce[scene] = true
			n.result[scene] = "COMMITTED"
			nowMs := time.Now().UnixNano() / 1e6
			tRead := n.getRoundStartMs(scene)
			latency := int64(0)
			if tRead > 0 {
				latency = nowMs - tRead
			}
			n.appendPBFTCommit(scene, nowMs, latency)
			fmt.Printf("[%s] ✅ Consensus reached for %s (%d/%d)\n",
				n.cfg.NodeID, scene, len(n.commitVoters[scene]), q)
			// get cached payload if available
			payload := n.payloadCache[scene]
			payloadCopy := append([]byte(nil), payload...)
			sceneCopy := scene
			go n.saveConsensusResult(sceneCopy, payloadCopy)
		}
	}
}

// ---------- Broadcasting helper ----------

func (n *Node) broadcast(phase Phase, sceneHash string) {
	msg := Message{Type: phase, SceneHash: sceneHash, Sender: n.cfg.NodeID}
	for peerID, addr := range n.cfg.Peers {
		go sendMessage(addr, msg)
		fmt.Printf("[%s] Sent %s to %s\n", n.cfg.NodeID, phase, peerID)
	}
}

// Save consensus result to /workspace/results/
func (n *Node) saveConsensusResult(sceneHash string, payload []byte) {
	dir := "/workspace/results"
	os.MkdirAll(dir, 0755)

	file := filepath.Join(dir, fmt.Sprintf("scene_%s.json", sceneHash))

	// include timestamp and node info
	record := map[string]interface{}{
		"scene_hash": sceneHash,
		"node_id":    n.cfg.NodeID,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
		"payload":    json.RawMessage(payload),
	}

	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		fmt.Printf("[%s] ⚠️ Failed to marshal result: %v\n", n.cfg.NodeID, err)
		return
	}

	if err := os.WriteFile(file, data, 0644); err != nil {
		fmt.Printf("[%s] ⚠️ Failed to write result file: %v\n", n.cfg.NodeID, err)
		return
	}

	n.appendResultSaved(sceneHash, file)
	fmt.Printf("[%s] 💾 Saved consensus result -> %s\n", n.cfg.NodeID, file)

	//geth
	// If payload is empty, nothing to send
	if !n.cfg.IsPrimary {
		return
	}
	sender := &RawTxSender{
		RPCURL:      EnvString("RPC_URL", ""),
		KeyfilePath: EnvString("KEYFILE_PATH", ""),
		Password:    EnvString("KEY_PASSWORD", ""),
		ToAddress:   EnvString("TO_ADDRESS", ""),
		GasLimit:    EnvUint64("GAS_LIMIT", 800000),
		SendTimeout: 5 * time.Second,
		MineTimeout: 20 * time.Second,
	}
	txHash, err := sender.SendPayload(payload)
	if err != nil {
		fmt.Printf("[%s] Failed to send raw tx to Geth: %v\n", n.cfg.NodeID, err)
		return
	}
	fmt.Printf("[%s]  Sent raw tx to Geth: %s\n", n.cfg.NodeID, txHash)

}
