package pbft

type Phase string

const (
	PrePrepare Phase = "PREPREPARE"
	Prepare    Phase = "PREPARE"
	Commit     Phase = "COMMIT"
)

type Message struct {
	Type      Phase  `json:"type"`
	SceneHash string `json:"scene_hash"`
	Sender    string `json:"sender"`
	Payload   []byte `json:"payload,omitempty"`
}
