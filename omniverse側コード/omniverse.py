# Comments are in English only.

import json
import os
import urllib.request
import time
import msvcrt

from pxr import UsdGeom, Gf
import omni.usd

LATEST_URL = "http://127.0.0.1:8000/latest"
 # Reader endpoint

ROOT = "/World/Detections"

Z_PLANE = 0.5
WORLD_W = 10.0
WORLD_H = 6.0
MIN_CONF = 0.0

FIXED_SCALE_DEFAULT = 0.01
FIXED_SCALE_PERSON = 0.7

ASSETS = {
    "chair": "file:///C:/Users/icnl2/Downloads/classroom_local_assets_origin/omniverse-content-production.s3-us-west-2.amazonaws.com/Assets/ArchVis/Commercial/Seating/Newman.usd",
    "laptop": "file:///C:/Users/icnl2/Downloads/TV/TV.usd",
    "tv": "file:///C:/Users/icnl2/Downloads/TV/TV.usd",
    "table": "file:///C:/Users/icnl2/Downloads/stool.usd",
    "person": "file:///C:/Users/icnl2/Downloads/Person/Person.usd",
}

FALLBACK_ASSET = None

# Shared JSONL path (must match FastAPI)
OMNIVERSE_JSONL_PATH = r"D:\Users\icnl2\DeDTN_code\code\omniverse.jsonl"

NODE_ID = "omniverse_kit"


def _now_unix_ms() -> int:
    return int(time.time() * 1000)


def _append_jsonl_locked(path: str, obj: dict) -> None:
    os.makedirs(os.path.dirname(path), exist_ok=True)
    line = json.dumps(obj, ensure_ascii=False) + "\n"

    with open(path, "a", encoding="utf-8") as f:
        try:
            msvcrt.locking(f.fileno(), msvcrt.LK_LOCK, 1)
            f.write(line)
            f.flush()
            os.fsync(f.fileno())
        finally:
            try:
                msvcrt.locking(f.fileno(), msvcrt.LK_UNLCK, 1)
            except Exception:
                pass


def _log_event(event: str, scene_hash: str, extra: dict = None) -> None:
    rec = {
        "kind": "omniverse",
        "event": event,
        "event_time_unix_ms": _now_unix_ms(),
        "node_id": NODE_ID,
        "scene_hash": scene_hash,
    }
    if extra:
        rec.update(extra)
    _append_jsonl_locked(OMNIVERSE_JSONL_PATH, rec)


def fetch_latest():
    with urllib.request.urlopen(LATEST_URL, timeout=5) as resp:
        return json.loads(resp.read().decode("utf-8"))


def _get_scene_hash_from_latest(data: dict) -> str:
    raw = data.get("raw", {})
    scene_hash = raw.get("scene_hash")
    return scene_hash if isinstance(scene_hash, str) else ""


def ensure_xform(stage, path):
    prim = stage.GetPrimAtPath(path)
    if prim and prim.IsValid():
        return prim
    return UsdGeom.Xform.Define(stage, path).GetPrim()


def clear_children(stage, root_path):
    root_prim = stage.GetPrimAtPath(root_path)
    if not (root_prim and root_prim.IsValid()):
        return
    for child in list(root_prim.GetChildren()):
        stage.RemovePrim(child.GetPath())


def normalize_bbox(bbox):
    x1, y1, x2, y2 = bbox
    if max(bbox) <= 1.5:
        return x1, y1, x2, y2
    IMG_W = 640.0
    IMG_H = 480.0
    return x1 / IMG_W, y1 / IMG_H, x2 / IMG_W, y2 / IMG_H


def spawn_asset(stage, prim_path, usd_url):
    xform_prim = ensure_xform(stage, prim_path)
    refs = xform_prim.GetReferences()
    refs.ClearReferences()
    refs.AddReference(usd_url)
    return xform_prim


def set_translate_and_scale(prim, x, y, z, s):
    xform = UsdGeom.Xformable(prim)
    xform.ClearXformOpOrder()
    xform.AddTranslateOp().Set(Gf.Vec3d(float(x), float(y), float(z)))
    xform.AddScaleOp().Set(Gf.Vec3f(float(s), float(s), float(s)))


stage = omni.usd.get_context().get_stage()
if stage is None:
    raise RuntimeError("No USD stage is open.")

ensure_xform(stage, ROOT)

t_fetch_start = _now_unix_ms()
data = fetch_latest()
t_fetch_end = _now_unix_ms()

scene_hash = _get_scene_hash_from_latest(data)
if not scene_hash:
    raise RuntimeError("scene_hash is missing in /latest response. Check FastAPI /latest raw.scene_hash.")

_log_event("latest_read", scene_hash, extra={
    "latency_ms": t_fetch_end - t_fetch_start,
    "source": "latest",
})

semantic = data.get("raw", {}).get("semantic")
if semantic is None:
    semantic = data.get("semantic", [])
if semantic is None:
    semantic = []

clear_children(stage, ROOT)

count = 0

PERSON_Z_OFFSET = -0.7
CHAIR_Z_OFFSET = -0.7

for obj in semantic:
    label = obj.get("object") or obj.get("label") or "obj"
    conf = float(obj.get("confidence", obj.get("conf", 0.0)))
    if conf < MIN_CONF:
        continue

    bbox = obj.get("bbox")
    if not bbox or len(bbox) != 4:
        continue

    usd_url = ASSETS.get(label, FALLBACK_ASSET)
    if not usd_url:
        continue

    x1, y1, x2, y2 = normalize_bbox(bbox)
    cx = (x1 + x2) / 2.0
    cy = (y1 + y2) / 2.0

    xw = (cx - 0.5) * WORLD_W
    yw = (0.5 - cy) * WORLD_H

    prim_path = f"{ROOT}/{label}_{count}"
    prim = spawn_asset(stage, prim_path, usd_url)

    scale = FIXED_SCALE_PERSON if label == "person" else FIXED_SCALE_DEFAULT

    z = Z_PLANE
    if label == "person":
        z += PERSON_Z_OFFSET
    elif label == "chair":
        z += CHAIR_Z_OFFSET

    set_translate_and_scale(prim, xw, yw, z, scale)
    count += 1

_log_event("omniverse_done", scene_hash, extra={
    "rendered_assets": count,
})

print(f"Rendered {count} assets from /latest (scene_hash={scene_hash})")
