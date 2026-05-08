package pbft

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

const MagicPrefix = "DED1:"

type RawTxSender struct {
	RPCURL      string
	KeyfilePath string
	Password    string
	ToAddress   string
	GasLimit    uint64

	// Timeout is used as a fallback only.
	// SendTimeout and MineTimeout are preferred if set.
	Timeout     time.Duration
	SendTimeout time.Duration
	MineTimeout time.Duration
}

var (
	// sendMu serializes tx creation/signing/sending to avoid nonce races.
	sendMu sync.Mutex

	// logMu serializes JSONL writes to avoid line corruption.
	logMu sync.Mutex
)

// SendPayload sends payload as tx input data and logs PoA latency events into JSONL.
// It MUST NOT require round_id and MUST work with SendPayload(payload) only.
func (s *RawTxSender) SendPayload(payload []byte) (string, error) {
	if len(payload) == 0 {
		return "", fmt.Errorf("empty payload")
	}
	if s.RPCURL == "" {
		return "", fmt.Errorf("RPC_URL is required")
	}
	if s.KeyfilePath == "" {
		return "", fmt.Errorf("KEYFILE_PATH is required")
	}
	if s.Password == "" {
		return "", fmt.Errorf("KEY_PASSWORD is required")
	}
	if s.ToAddress == "" {
		return "", fmt.Errorf("TO_ADDRESS is required")
	}

	// Avoid nonce races by serializing per-sender (same account).
	sendMu.Lock()
	defer sendMu.Unlock()

	// Best-effort IDs from payload.
	nodeID, sceneHash := extractIDsFromPayload(payload)

	// Timeouts (separate).
	sendTimeout := s.SendTimeout
	mineTimeout := s.MineTimeout

	if sendTimeout <= 0 {
		if s.Timeout > 0 {
			sendTimeout = s.Timeout
		} else {
			sendTimeout = 5 * time.Second
		}
	}
	if mineTimeout <= 0 {
		if s.Timeout > 0 {
			mineTimeout = s.Timeout
		} else {
			mineTimeout = 20 * time.Second
		}
	}

	ctxSend, cancelSend := context.WithTimeout(context.Background(), sendTimeout)
	defer cancelSend()

	client, err := ethclient.DialContext(ctxSend, s.RPCURL)
	if err != nil {
		return "", fmt.Errorf("dial rpc: %w", err)
	}
	defer client.Close()

	keyJSON, err := os.ReadFile(s.KeyfilePath)
	if err != nil {
		return "", fmt.Errorf("read keyfile: %w", err)
	}
	key, err := keystore.DecryptKey(keyJSON, s.Password)
	if err != nil {
		return "", fmt.Errorf("decrypt keyfile: %w", err)
	}

	from := key.Address
	to := common.HexToAddress(s.ToAddress)

	chainID, err := client.ChainID(ctxSend)
	if err != nil {
		return "", fmt.Errorf("chain id: %w", err)
	}

	// Record the best-effort "current head" at send time to compute block_wait.
	sentBlock, _ := client.BlockNumber(ctxSend)

	nonce, err := client.PendingNonceAt(ctxSend, from)
	if err != nil {
		return "", fmt.Errorf("nonce: %w", err)
	}
	gasPrice, err := client.SuggestGasPrice(ctxSend)
	if err != nil {
		return "", fmt.Errorf("gas price: %w", err)
	}

	// Build tx input data.
	data := append([]byte(MagicPrefix), payload...)
	payloadBytes := len(data)

	value := big.NewInt(0)
	gasLimit := s.GasLimit
	if gasLimit == 0 {
		gasLimit = 800000
	}

	// Create and sign tx.
	tx := types.NewTransaction(nonce, to, value, gasLimit, gasPrice, data)
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), key.PrivateKey)
	if err != nil {
		return "", fmt.Errorf("sign tx: %w", err)
	}

	rawBytes, err := signedTx.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("marshal tx: %w", err)
	}
	rawHex := "0x" + hex.EncodeToString(rawBytes)

	// Send raw tx via JSON-RPC.
	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "eth_sendRawTransaction",
		"params":  []interface{}{rawHex},
		"id":      1,
	}
	b, _ := json.Marshal(req)

	httpClient := &http.Client{Timeout: sendTimeout}
	resp, err := httpClient.Post(s.RPCURL, "application/json", bytes.NewReader(b))
	if err != nil {
		return "", fmt.Errorf("post raw tx: %w", err)
	}
	defer resp.Body.Close()

	var out struct {
		Result string `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	if out.Error != nil {
		return "", fmt.Errorf("rpc error %d: %s", out.Error.Code, out.Error.Message)
	}
	if out.Result == "" {
		return "", fmt.Errorf("empty tx hash")
	}

	// tx_sent: record the moment we received txHash (use monotonic for latency).
	txSentTime := time.Now()
	txHashStr := out.Result
	txHash := common.HexToHash(txHashStr)

	_ = appendJSONLSafe("/workspace/PoA_latency.jsonl", map[string]interface{}{
		"kind":               "poa",
		"event":              "tx_sent",
		"event_time_unix_ms": txSentTime.UnixMilli(),
		"node_id":            nodeID,
		"scene_hash":         sceneHash,
		"tx_hash":            txHashStr,
		"payload_bytes":      payloadBytes,

		// Extra diagnostics (recommended).
		"nonce":      nonce,
		"sent_block": sentBlock,
	})

	// Wait for receipt (mined) using a separate context.
	ctxMine, cancelMine := context.WithTimeout(context.Background(), mineTimeout)
	defer cancelMine()

	receipt, rerr := waitReceipt(ctxMine, client, txHash, 300*time.Millisecond)
	if rerr != nil {
		_ = appendJSONLSafe("/workspace/PoA_latency.jsonl", map[string]interface{}{
			"kind":               "poa",
			"event":              "tx_timeout",
			"event_time_unix_ms": time.Now().UnixMilli(),
			"node_id":            nodeID,
			"scene_hash":         sceneHash,
			"tx_hash":            txHashStr,
			"timeout_ms":         mineTimeout.Milliseconds(),
			"error":              rerr.Error(),

			// Repeat useful fields for debugging.
			"nonce":      nonce,
			"sent_block": sentBlock,
		})

		// Return txHash even on timeout (tx may still be mined later).
		return txHashStr, nil
	}

	// tx_mined: receipt obtained successfully.
	latencyMineMs := time.Since(txSentTime).Milliseconds()
	minedBlock := receipt.BlockNumber.Uint64()
	blockWait := int64(0)
	if minedBlock >= sentBlock {
		blockWait = int64(minedBlock - sentBlock)
	}

	_ = appendJSONLSafe("/workspace/PoA_latency.jsonl", map[string]interface{}{
		"kind":               "poa",
		"event":              "tx_mined",
		"event_time_unix_ms": time.Now().UnixMilli(),
		"node_id":            nodeID,
		"scene_hash":         sceneHash,
		"tx_hash":            txHashStr,
		"block_number":       minedBlock,
		"latency_mine_ms":    latencyMineMs,

		// Extra diagnostics.
		"nonce":      nonce,
		"sent_block": sentBlock,
		"block_wait": blockWait,
	})

	return txHashStr, nil
}

func waitReceipt(
	ctx context.Context,
	client *ethclient.Client,
	txHash common.Hash,
	interval time.Duration,
) (*types.Receipt, error) {
	for {
		r, err := client.TransactionReceipt(ctx, txHash)
		if err == nil && r != nil {
			return r, nil
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("receipt timeout: %w", ctx.Err())
		case <-time.After(interval):
		}
	}
}

func appendJSONLSafe(path string, obj map[string]interface{}) error {
	logMu.Lock()
	defer logMu.Unlock()
	return appendJSONL(path, obj)
}

func appendJSONL(path string, obj map[string]interface{}) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	b, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	_, err = f.Write(append(b, '\n'))
	return err
}

// extractIDsFromPayload extracts node_id and scene_hash from the semantic JSON payload.
// Best-effort mapping:
// - node_id: device_id (fallback: "unknown")
// - scene_hash: frame_hash (fallback: tx_id, fallback: "unknown")
func extractIDsFromPayload(payload []byte) (string, string) {
	type semanticPayload struct {
		TxID      string `json:"tx_id"`
		FrameHash string `json:"frame_hash"`
		DeviceID  string `json:"device_id"`
	}

	var sp semanticPayload
	if err := json.Unmarshal(payload, &sp); err != nil {
		return "unknown", "unknown"
	}

	nodeID := sp.DeviceID
	if nodeID == "" {
		nodeID = "unknown"
	}

	sceneHash := sp.FrameHash
	if sceneHash == "" {
		sceneHash = sp.TxID
	}
	if sceneHash == "" {
		sceneHash = "unknown"
	}

	return nodeID, sceneHash
}
