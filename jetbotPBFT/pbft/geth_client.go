package pbft

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type GethConfig struct {
	RPCURL   string
	From     string
	Gas      string
	GasPrice string
	To       string
}

// Parameters for eth_sendTransaction
type ethTxParams struct {
	From     string `json:"from"`
	To       string `json:"to,omitempty"`
	Gas      string `json:"gas,omitempty"`
	GasPrice string `json:"gasPrice,omitempty"`
	Value    string `json:"value,omitempty"`
	Data     string `json:"data,omitempty"`
}

type ethRPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      int           `json:"id"`
}

type ethRPCResponse struct {
	JSONRPC string       `json:"jsonrpc"`
	ID      int          `json:"id"`
	Result  string       `json:"result"`
	Error   *ethRPCError `json:"error,omitempty"`
}

type ethRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// SendSemanticTx sends the semantic payload as Ethereum transaction data
func SendSemanticTx(cfg GethConfig, payload []byte) (string, error) {
	// Encode JSON payload into hex string (0x...)
	dataHex := "0x" + hex.EncodeToString(payload)

	tx := ethTxParams{
		From:     cfg.From,
		To:       cfg.To,
		Gas:      cfg.Gas,
		GasPrice: cfg.GasPrice,
		Value:    "0x0",
		Data:     dataHex,
	}

	req := ethRPCRequest{
		JSONRPC: "2.0",
		Method:  "eth_sendTransaction",
		Params:  []interface{}{tx},
		ID:      int(time.Now().Unix()),
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal RPC request: %w", err)
	}

	resp, err := http.Post(cfg.RPCURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to POST to geth: %w", err)
	}
	defer resp.Body.Close()

	var rpcResp ethRPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return "", fmt.Errorf("failed to decode RPC response: %w", err)
	}

	if rpcResp.Error != nil {
		return "", fmt.Errorf("geth RPC error: %d %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	return rpcResp.Result, nil // tx hash
}
