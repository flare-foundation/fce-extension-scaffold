package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"

	"extension-scaffold/pkg/types"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/flare-foundation/go-flare-common/pkg/logger"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs"
)

type decodeRequest struct {
	OPType    string `json:"opType"`
	OPCommand string `json:"opCommand"`
	Kind      string `json:"kind"`
	Data      string `json:"data"`
}

func main() {
	tf := flag.String("t", "http://localhost:8100", "types server URL")
	flag.Parse()

	baseURL := *tf

	passed := 0
	failed := 0

	run := func(name string, fn func() error) {
		logger.Infof("TEST: %s", name)
		if err := fn(); err != nil {
			logger.Errorf("  FAIL: %s", err)
			failed++
		} else {
			logger.Infof("  PASS")
			passed++
		}
	}

	// --- Success cases ---

	run("DEPOSIT message (ABI-encoded)", func() error {
		req := types.DepositRequest{
			Depositor: common.HexToAddress("0xed2B5717c9b936ecC76d75401026A99143e278F5"),
			Amount:    big.NewInt(1000000),
		}
		encoded, err := structs.Encode(types.DepositMessageArg, req)
		if err != nil {
			return fmt.Errorf("ABI encode: %w", err)
		}
		data := hexutil.Encode(encoded)
		resp, err := postDecode(baseURL, decodeRequest{
			OPType: "PREDICTION_MARKET", OPCommand: "DEPOSIT", Kind: "message", Data: data,
		})
		if err != nil {
			return err
		}
		return requireField(resp, "Depositor", "0xed2B5717c9b936ecC76d75401026A99143e278F5")
	})

	run("SETTLE message (ABI-encoded)", func() error {
		req := types.SettleRequest{
			MarketId:       big.NewInt(0),
			ContractAddr:   common.HexToAddress("0x072A3A0C04Cf8CDcaf5B4A73a4Ed4fF5A841531f"),
			Outcome:        true,
			ReferenceValue: big.NewInt(12345),
		}
		encoded, err := structs.Encode(types.SettleMessageArg, req)
		if err != nil {
			return fmt.Errorf("ABI encode: %w", err)
		}
		data := hexutil.Encode(encoded)
		resp, err := postDecode(baseURL, decodeRequest{
			OPType: "PREDICTION_MARKET", OPCommand: "SETTLE", Kind: "message", Data: data,
		})
		if err != nil {
			return err
		}
		return requireField(resp, "ContractAddr", "0x072A3A0C04Cf8CDcaf5B4A73a4Ed4fF5A841531f")
	})

	run("SETTLE result (ABI-encoded)", func() error {
		req := types.SettleRequest{
			MarketId:       big.NewInt(0),
			ContractAddr:   common.HexToAddress("0x072A3A0C04Cf8CDcaf5B4A73a4Ed4fF5A841531f"),
			Outcome:        false,
			ReferenceValue: big.NewInt(99),
		}
		encoded, err := structs.Encode(types.SettleMessageArg, req)
		if err != nil {
			return fmt.Errorf("ABI encode: %w", err)
		}
		data := hexutil.Encode(encoded)
		resp, err := postDecode(baseURL, decodeRequest{
			OPType: "PREDICTION_MARKET", OPCommand: "SETTLE", Kind: "result", Data: data,
		})
		if err != nil {
			return err
		}
		return requireFieldFloat(resp, "ReferenceValue", 99)
	})

	// --- Error cases ---

	run("unknown OPType → 404", func() error {
		return expectStatus(baseURL, decodeRequest{
			OPType: "UNKNOWN", OPCommand: "", Kind: "message", Data: "0x7b7d",
		}, http.StatusNotFound)
	})

	run("invalid kind → 400", func() error {
		return expectStatus(baseURL, decodeRequest{
			OPType: "PREDICTION_MARKET", OPCommand: "DEPOSIT", Kind: "invalid", Data: "0x7b7d",
		}, http.StatusBadRequest)
	})

	run("invalid hex → 400", func() error {
		return expectStatus(baseURL, decodeRequest{
			OPType: "PREDICTION_MARKET", OPCommand: "DEPOSIT", Kind: "message", Data: "not-hex",
		}, http.StatusBadRequest)
	})

	run("valid hex, bad payload → 422", func() error {
		return expectStatus(baseURL, decodeRequest{
			OPType: "PREDICTION_MARKET", OPCommand: "DEPOSIT", Kind: "message", Data: "0xdeadbeef",
		}, http.StatusUnprocessableEntity)
	})

	// --- Summary ---
	logger.Infof("")
	logger.Infof("Results: %d passed, %d failed", passed, failed)
	if failed > 0 {
		os.Exit(1)
	}
}

// postDecode sends a POST /decode request and returns the "decoded" field from the response.
func postDecode(baseURL string, req decodeRequest) (map[string]any, error) {
	body, _ := json.Marshal(req)
	resp, err := http.Post(baseURL+"/decode", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("POST /decode: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("expected 200, got %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Decoded map[string]any `json:"decoded"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return result.Decoded, nil
}

// expectStatus sends a POST /decode and asserts the HTTP status code.
func expectStatus(baseURL string, req decodeRequest, wantStatus int) error {
	body, _ := json.Marshal(req)
	resp, err := http.Post(baseURL+"/decode", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("POST /decode: %w", err)
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body) //nolint:errcheck

	if resp.StatusCode != wantStatus {
		return fmt.Errorf("expected status %d, got %d", wantStatus, resp.StatusCode)
	}
	return nil
}

// requireField asserts a string field in the decoded response.
func requireField(decoded map[string]any, key, want string) error {
	got, ok := decoded[key]
	if !ok {
		return fmt.Errorf("missing field %q", key)
	}
	gotStr, ok := got.(string)
	if !ok {
		return fmt.Errorf("field %q: expected string, got %T", key, got)
	}
	if gotStr != want {
		return fmt.Errorf("field %q: expected %q, got %q", key, want, gotStr)
	}
	return nil
}

// requireFieldFloat asserts a numeric field in the decoded response.
func requireFieldFloat(decoded map[string]any, key string, want float64) error {
	got, ok := decoded[key]
	if !ok {
		return fmt.Errorf("missing field %q", key)
	}
	gotFloat, ok := got.(float64)
	if !ok {
		return fmt.Errorf("field %q: expected number, got %T", key, got)
	}
	if gotFloat != want {
		return fmt.Errorf("field %q: expected %v, got %v", key, want, gotFloat)
	}
	return nil
}
