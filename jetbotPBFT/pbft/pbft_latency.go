package pbft

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

const pbftLatencyPath = "/workspace/pbft_latency.jsonl"

var pbftJSONLMu sync.Mutex

type PBFTLatencyRecord struct {
	Kind            string                 `json:"kind"`
	Event           string                 `json:"event"`
	EventTimeUnixMs int64                  `json:"event_time_unix_ms"`
	NodeID          string                 `json:"node_id"`
	SceneHash       string                 `json:"scene_hash,omitempty"`
	ResultPath      string                 `json:"result_path,omitempty"`
	LatencyMs       int64                  `json:"latency_ms,omitempty"`
	Extra           map[string]interface{} `json:"extra,omitempty"`
}

// AppendPBFTJSONL appends one record per line to the target JSONL file.
func AppendPBFTJSONL(path string, record PBFTLatencyRecord) error {
	pbftJSONLMu.Lock()
	defer pbftJSONLMu.Unlock()

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	line, err := json.Marshal(record)
	if err != nil {
		return err
	}

	if _, err := f.Write(append(line, '\n')); err != nil {
		return err
	}
	return nil
}

func (n *Node) setRoundStart(sceneHash string, nowMs int64) {
	n.roundMu.Lock()
	n.roundStartMs[sceneHash] = nowMs
	n.roundMu.Unlock()
}

func (n *Node) getRoundStartMs(sceneHash string) int64 {
	n.roundMu.Lock()
	start := n.roundStartMs[sceneHash]
	n.roundMu.Unlock()
	return start
}

func (n *Node) appendReadLatest(sceneHash string, nowMs int64) {
	rec := PBFTLatencyRecord{
		Kind:            "pbft",
		Event:           "read_latest",
		EventTimeUnixMs: nowMs,
		NodeID:          n.cfg.NodeID,
		SceneHash:       sceneHash,
	}
	if err := AppendPBFTJSONL(pbftLatencyPath, rec); err != nil {
		fmt.Printf("[PBFT] Failed to append read_latest: %v\n", err)
	}
}

func (n *Node) appendPBFTCommit(sceneHash string, nowMs int64, latencyMs int64) {
	rec := PBFTLatencyRecord{
		Kind:            "pbft",
		Event:           "pbft_commit",
		EventTimeUnixMs: nowMs,
		NodeID:          n.cfg.NodeID,
		SceneHash:       sceneHash,
		LatencyMs:       latencyMs,
	}
	if err := AppendPBFTJSONL(pbftLatencyPath, rec); err != nil {
		fmt.Printf("[PBFT] Failed to append pbft_commit: %v\n", err)
	}
}

func (n *Node) appendResultSaved(sceneHash string, resultPath string) {
	nowMs := time.Now().UnixNano() / 1e6
	rec := PBFTLatencyRecord{
		Kind:            "pbft",
		Event:           "result_saved",
		EventTimeUnixMs: nowMs,
		NodeID:          n.cfg.NodeID,
		SceneHash:       sceneHash,
		ResultPath:      resultPath,
	}

	if start := n.getRoundStartMs(sceneHash); start > 0 && nowMs >= start {
		rec.LatencyMs = nowMs - start
	}

	if err := AppendPBFTJSONL(pbftLatencyPath, rec); err != nil {
		fmt.Printf("[PBFT] Failed to append result_saved: %v\n", err)
	}
}
