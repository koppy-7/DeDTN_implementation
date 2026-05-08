package pbft

// Transaction represents semantic data with metadata for PBFT consensus
type Transaction struct {
	TxID      string                   `json:"tx_id"`
	SceneHash string                   `json:"scene_hash"`
	DeviceID  string                   `json:"device_id"`
	ModelName string                   `json:"model_name"`
	Timestamp string                   `json:"timestamp"` //stringに変更
	Semantic  []map[string]interface{} `json:"semantic"`
	Meta      map[string]interface{}   `json:"meta"`
}
