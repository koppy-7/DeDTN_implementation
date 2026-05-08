package pbft

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"time"
)

// Struct for incoming payload (from Primary)
type SemanticPayload struct {
	TxID      string       `json:"tx_id"`
	SceneHash string       `json:"scene_hash"`
	DeviceID  string       `json:"device_id"`
	ModelName string       `json:"model_name"`
	Timestamp string       `json:"timestamp"`
	Semantic  []ObjectInfo `json:"semantic"`
}

type ObjectInfo struct {
	Object     string    `json:"object"`
	BBox       []float64 `json:"bbox"`
	Confidence float64   `json:"confidence"`
}

// Similarity threshold
const IOU_THRESHOLD = 0.3
const TIME_THRESHOLD = 10 * time.Second

// ---- Main verification function ----
func VerifySemantic(payload []byte) bool {

	// 1. decode of Primarysemantic
	var primary SemanticPayload
	if err := json.Unmarshal(payload, &primary); err != nil {
		return false
	}

	// 2. timestamp±3s
	t1, _ := time.Parse(time.RFC3339, primary.Timestamp)

	if time.Since(t1) > TIME_THRESHOLD {
		fmt.Println("differnt time")
		return false
	}

	// 3. read semantic
	localPath := "/workspace/semantic/latest.json"
	raw, err := os.ReadFile(localPath)
	if err != nil {
		fmt.Println("can't read semantic")
		return false
	}

	var local SemanticPayload
	if err := json.Unmarshal(raw, &local); err != nil {
		return false
	}

	// 4. compare semantic（IoU）
	// If either primary or local semantic list is empty, fail
	if len(primary.Semantic) == 0 || len(local.Semantic) == 0 {
		return false
	}

	// Find best matching pair (max IoU with same object name)
	bestIoU := 0.0
	matched := false

	for _, pObj := range primary.Semantic {
		for _, lObj := range local.Semantic {

			// object name must match
			if pObj.Object != lObj.Object {
				continue
			}

			// compute IoU
			iou := computeIoU(pObj.BBox, lObj.BBox)
			if iou > bestIoU {
				bestIoU = iou
				matched = true
			}
		}
	}

	// If no object name matched at all → fail immediately
	if !matched {
		fmt.Println("No matching object classes found")
		return false
	}

	// IoU threshold check
	if bestIoU < IOU_THRESHOLD {
		fmt.Printf("IoU too low: %f\n", bestIoU)
		return false
	}

	return true

}

// IoU（重なり率）caluculate
func computeIoU(a, b []float64) float64 {
	ax1, ay1, ax2, ay2 := a[0], a[1], a[2], a[3]
	bx1, by1, bx2, by2 := b[0], b[1], b[2], b[3]

	interX1 := math.Max(ax1, bx1)
	interY1 := math.Max(ay1, by1)
	interX2 := math.Min(ax2, bx2)
	interY2 := math.Min(ay2, by2)

	interArea := math.Max(0, interX2-interX1) * math.Max(0, interY2-interY1)
	areaA := (ax2 - ax1) * (ay2 - ay1)
	areaB := (bx2 - bx1) * (by2 - by1)

	return interArea / (areaA + areaB - interArea)
}
