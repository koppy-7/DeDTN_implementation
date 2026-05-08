from ultralytics import YOLO
import json, hashlib, time
from datetime import datetime

# Load YOLO model
model = YOLO("/workspace/yolo11n.pt")

# tx_id 用のカウンタ（device内単調増加）
TX_COUNTER = 0
CONF_THRESHOLD = 0.5

def canonical_json(obj):
    return json.dumps(
        obj,
        sort_keys=True,
        separators=(",", ":"),
        ensure_ascii=False
    )

def make_tx(result):
    global TX_COUNTER
    TX_COUNTER += 1

    now = datetime.utcnow().isoformat() + "Z"

    semantic = []
    for box in result.boxes:
        cls_id = int(box.cls[0])
        name = result.names[cls_id]
        conf = round(float(box.conf[0]), 3)
        
        if conf<CONF_THRESHOLD:
            continue

        xyxy = [round(float(x), 2) for x in box.xyxy[0].tolist()]

        semantic.append({
            "object": name,
            "bbox": xyxy,
            "confidence": conf
        })

    # semantic から scene_Hash を計算（PoGのcommit対象）
    semantic_str = canonical_json(semantic)
    scene_Hash = hashlib.sha256(semantic_str.encode()).hexdigest()

    tx = {
        "tx_id": str(TX_COUNTER),          # ← 連番
        "scene_Hash": scene_Hash,     # semantic由来ハッシュ
        "device_id": "jetson-A",
        "model_name": "YOLO11n",
        "timestamp": now,
        "semantic": semantic,
        "meta": {"source": "camera0"}
    }

    return tx

# Start real-time detection
for result in model(source=0, stream=True, show=False):
    tx = make_tx(result)
    json_path = "/workspace/semantic/latest.json"
    with open(json_path, "w") as f:
        json.dump(tx, f, indent=2, ensure_ascii=False)

    print(f"[YOLO] tx_id={tx['tx_id']} scene_Hash={tx['scene_Hash'][:16]}")
    time.sleep(2)

