# Comments are in English only.

import json
import os
import time
from typing import Any, Dict, Optional, Tuple, Union

from fastapi import FastAPI, HTTPException, Query
from fastapi.middleware.cors import CORSMiddleware
from web3 import Web3
from web3.middleware import ExtraDataToPOAMiddleware

import msvcrt

app = FastAPI(title="DeDTN Blockchain Reader", version="0.2.1")

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=False,
    allow_methods=["*"],
    allow_headers=["*"],
)

RPC_URL = "http://192.168.0.45:8545"
CONFIRMATIONS = 0
MAGIC_PREFIX = "DED1:"
MAX_LOOKBACK = 5000

# Shared JSONL path (must match Omniverse script)
OMNIVERSE_JSONL_PATH =r"D:\Users\icnl2\DeDTN_code\code\omniverse.jsonl"
NODE_ID = "server_reader"

w3 = Web3(Web3.HTTPProvider(RPC_URL))
w3.middleware_onion.inject(ExtraDataToPOAMiddleware, layer=0)


def _now_unix_ms() -> int:
    return int(time.time() * 1000)


def _append_jsonl_locked(path: str, obj: Dict[str, Any]) -> None:
    # Best-effort file lock for multi-process append on Windows.
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


def _log_event(event: str, scene_hash: Optional[str], extra: Optional[Dict[str, Any]] = None) -> None:
    rec: Dict[str, Any] = {
        "kind": "omniverse",
        "event": event,
        "event_time_unix_ms": _now_unix_ms(),
        "node_id": NODE_ID,
    }
    if scene_hash:
        rec["scene_hash"] = scene_hash
    if extra:
        rec.update(extra)
    _append_jsonl_locked(OMNIVERSE_JSONL_PATH, rec)


def _extract_scene_hash(payload: Dict[str, Any]) -> Optional[str]:
    # Use canonical key "scene_hash".
    v = payload.get("scene_hash")
    if isinstance(v, str) and v.strip():
        return v.strip()
    return None


def _to_text_input(inp: Union[str, bytes, Any]) -> Optional[str]:
    # Normalize tx.input from web3.py to a UTF-8 string like: "DED1:{...}"
    if inp is None:
        return None

    if isinstance(inp, (bytes, bytearray)):
        try:
            return inp.decode("utf-8", errors="strict")
        except Exception:
            return None

    if hasattr(inp, "hex") and not isinstance(inp, str):
        try:
            hx = inp.hex()
            if hx.startswith("0x"):
                hx = hx[2:]
            raw = bytes.fromhex(hx).decode("utf-8", errors="strict")
            return raw
        except Exception:
            return None

    if isinstance(inp, str):
        if inp.startswith("0x"):
            try:
                raw = bytes.fromhex(inp[2:]).decode("utf-8", errors="strict")
                return raw
            except Exception:
                return None
        return inp

    return None


def _decode_input_to_json(tx_input: Any) -> Optional[Dict[str, Any]]:
    raw = _to_text_input(tx_input)
    if not raw:
        return None

    if MAGIC_PREFIX:
        if not raw.startswith(MAGIC_PREFIX):
            return None
        raw = raw[len(MAGIC_PREFIX):]

    try:
        obj = json.loads(raw)
    except Exception:
        return None

    if not isinstance(obj, dict):
        return None
    return obj


def _get_stable_block_number() -> int:
    latest = w3.eth.block_number
    stable = latest - CONFIRMATIONS
    return stable if stable > 0 else 0


def _find_latest_semantic() -> Tuple[int, str, Dict[str, Any]]:
    stable_bn = _get_stable_block_number()
    start_bn = stable_bn
    end_bn = max(stable_bn - MAX_LOOKBACK, 0)

    for bn in range(start_bn, end_bn - 1, -1):
        block = w3.eth.get_block(bn, full_transactions=True)
        for tx in block["transactions"]:
            payload = _decode_input_to_json(tx.get("input", None))
            if payload is not None:
                return bn, tx["hash"].hex(), payload

    raise HTTPException(status_code=404, detail="No semantic JSON tx found in recent blocks")


@app.get("/health")
def health() -> Dict[str, Any]:
    ok = w3.is_connected()
    return {
        "connected": ok,
        "rpc_url": RPC_URL,
        "chain_id": w3.eth.chain_id if ok else None,
        "latest_block": w3.eth.block_number if ok else None,
        "stable_block": _get_stable_block_number() if ok else None,
        "confirmations": CONFIRMATIONS,
        "max_lookback": MAX_LOOKBACK,
        "magic_prefix": MAGIC_PREFIX,
    }


@app.get("/latest")
def latest() -> Dict[str, Any]:
    if not w3.is_connected():
        raise HTTPException(status_code=503, detail="Cannot connect to Geth RPC")

    req_in_ms = _now_unix_ms()

    bn, txhash, payload = _find_latest_semantic()

    # Ensure canonical "scene_hash" exists.
    scene_hash = _extract_scene_hash(payload)

    # Log chain_read right after payload decode succeeded.
    _log_event("chain_read", scene_hash, extra={
        "tx_hash": txhash,
        "semantic_block_number": bn,
        "stable_block_number": _get_stable_block_number(),
        "reader_latency_ms": _now_unix_ms() - req_in_ms,
    })

    semantic_list = payload.get("semantic")
    if not isinstance(semantic_list, list):
        semantic_list = []

    return {
        "source": "ethereum_clique",
        "stable_block_number": _get_stable_block_number(),
        "semantic_block_number": bn,
        "tx_hash": txhash,
        "timestamp_unix": int(time.time()),
        "raw": {
            "tx_hash": txhash,
            "scene_hash": scene_hash,  # Canonical key for Omniverse/logger
            "device_id": payload.get("device_id"),
            "model_name": payload.get("model_name"),
            "timestamp": payload.get("timestamp"),
            "meta": payload.get("meta", {}),
            "semantic": semantic_list,
        }
    }


@app.get("/by_tx")
def by_tx(tx_hash: str = Query(...)) -> Dict[str, Any]:
    if not w3.is_connected():
        raise HTTPException(status_code=503, detail="Cannot connect to Geth RPC")

    tx = w3.eth.get_transaction(tx_hash)
    raw_text = _to_text_input(tx.get("input", None))
    payload = _decode_input_to_json(tx.get("input", None))

    return {
        "tx_hash": tx_hash,
        "found": payload is not None,
        "input_preview": (raw_text[:120] if raw_text else None),
        "tx": payload,
        "semantic": payload.get("semantic") if payload else None,
    }
