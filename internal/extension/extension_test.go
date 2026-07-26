package extension

import (
	"bytes"
	"encoding/json"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"extension-scaffold/internal/config"
	"extension-scaffold/pkg/types"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	teetypes "github.com/flare-foundation/tee-node/pkg/types"
	teeutils "github.com/flare-foundation/tee-node/pkg/utils"
)

// toHash mirrors teeutils.ToHash for clarity: left-pads a string into a 32-byte hash.
func toHash(s string) common.Hash { return teeutils.ToHash(s) }

// dataFixedJSON is the structure that processorutils.Parse extracts from Data.Message.
type dataFixedJSON struct {
	InstructionID      common.Hash    `json:"instructionId"`
	TeeID              common.Address `json:"teeId"`
	Timestamp          uint64         `json:"timestamp"`
	RewardEpochID      uint32         `json:"rewardEpochId"`
	OPType             common.Hash    `json:"opType"`
	OPCommand          common.Hash    `json:"opCommand"`
	Cosigners          []string       `json:"cosigners"`
	CosignersThreshold uint64         `json:"cosignersThreshold"`
	OriginalMessage    hexutil.Bytes  `json:"originalMessage"`
}

// buildTestAction constructs a teetypes.Action whose Data.Message is the
// JSON-encoded DataFixed payload. This is what processAction expects to parse.
func buildTestAction(opType, opCommand common.Hash, originalMessage []byte) teetypes.Action {
	return buildTestActionWithInstructionID(opType, opCommand, originalMessage, common.Hash{})
}

// buildTestActionWithInstructionID is like buildTestAction but sets a specific
// InstructionID — needed for withdraw tests, where processWithdraw derives
// withdrawalId from df.InstructionID.
func buildTestActionWithInstructionID(opType, opCommand common.Hash, originalMessage []byte, instructionID common.Hash) teetypes.Action {
	df := dataFixedJSON{
		InstructionID:   instructionID,
		OPType:          opType,
		OPCommand:       opCommand,
		OriginalMessage: originalMessage,
	}
	msg, _ := json.Marshal(df)

	return teetypes.Action{
		Data: teetypes.ActionData{
			ID:            common.HexToHash("0x1234"),
			SubmissionTag: "submit",
			Message:       msg,
		},
	}
}

func newTestExtension() *Extension {
	return &Extension{
		betLedger: make(map[string][]types.Bet),
		// port 1 is not listening in test environments — used to exercise the
		// decrypt-call error path deterministically without a live sign server.
		signPort: 1,
	}
}

func abiEncodeDeposit(depositor common.Address, amount int64) []byte {
	args := abi.Arguments{types.DepositMessageArg}
	encoded, _ := args.Pack(types.DepositRequest{Depositor: depositor, Amount: big.NewInt(amount)})
	return encoded
}

func abiEncodePlaceBet(bettor common.Address, marketID int64, encryptedBet []byte) []byte {
	args := abi.Arguments{types.PlaceBetMessageArg}
	encoded, _ := args.Pack(types.PlaceBetRequest{Bettor: bettor, MarketId: big.NewInt(marketID), EncryptedBet: encryptedBet})
	return encoded
}

func abiEncodeSettle(marketID int64, contractAddr common.Address, outcome bool, referenceValue int64) []byte {
	args := abi.Arguments{types.SettleMessageArg}
	encoded, _ := args.Pack(types.SettleRequest{
		MarketId:       big.NewInt(marketID),
		ContractAddr:   contractAddr,
		Outcome:        outcome,
		ReferenceValue: big.NewInt(referenceValue),
	})
	return encoded
}

func abiEncodeWithdraw(requester common.Address, amount int64, to common.Address) []byte {
	args := abi.Arguments{types.WithdrawMessageArg}
	encoded, _ := args.Pack(types.WithdrawRequest{Requester: requester, Amount: big.NewInt(amount), To: to})
	return encoded
}

// --- 4.1: OPType/OPCommand Hash Debug Info ---

func TestProcessAction_UnknownOPType(t *testing.T) {
	e := newTestExtension()
	action := buildTestAction(
		toHash("UNKNOWN_TYPE"),
		toHash(config.OPCommandDeposit),
		nil,
	)

	status, body := e.processAction(action)

	if status != http.StatusNotImplemented {
		t.Fatalf("expected status %d, got %d", http.StatusNotImplemented, status)
	}

	bodyStr := string(body)
	t.Logf("501 body: %s", bodyStr)

	if !contains(bodyStr, "unsupported op type") {
		t.Error("expected body to contain 'unsupported op type'")
	}

	receivedHash := toHash("UNKNOWN_TYPE").Hex()
	if !contains(bodyStr, receivedHash) {
		t.Errorf("expected body to contain received hash %s", receivedHash)
	}

	expectedHash := toHash(config.OPTypePredictionMarket).Hex()
	if !contains(bodyStr, expectedHash) {
		t.Errorf("expected body to contain expected hash %s", expectedHash)
	}

	if !contains(bodyStr, config.OPTypePredictionMarket) {
		t.Errorf("expected body to contain %q", config.OPTypePredictionMarket)
	}
}

func TestProcessAction_UnknownOPCommand(t *testing.T) {
	e := newTestExtension()
	action := buildTestAction(
		toHash(config.OPTypePredictionMarket),
		toHash("UNKNOWN_COMMAND"),
		nil,
	)

	status, body := e.processAction(action)

	if status != http.StatusNotImplemented {
		t.Fatalf("expected status %d, got %d", http.StatusNotImplemented, status)
	}

	bodyStr := string(body)
	t.Logf("501 body: %s", bodyStr)

	if !contains(bodyStr, "unsupported op command") {
		t.Error("expected body to contain 'unsupported op command'")
	}

	receivedHash := toHash("UNKNOWN_COMMAND").Hex()
	if !contains(bodyStr, receivedHash) {
		t.Errorf("expected body to contain received hash %s", receivedHash)
	}

	for _, cmd := range []string{config.OPCommandDeposit, config.OPCommandPlaceBet, config.OPCommandSettle, config.OPCommandWithdraw} {
		if !contains(bodyStr, cmd) {
			t.Errorf("expected body to contain command name %q", cmd)
		}
	}
}

// --- Valid Actions ---

func TestProcessAction_ValidDeposit(t *testing.T) {
	e := newTestExtension()

	depositor := common.HexToAddress("0xed2B5717c9b936ecC76d75401026A99143e278F5")
	payload := abiEncodeDeposit(depositor, 1000000)
	action := buildTestAction(
		toHash(config.OPTypePredictionMarket),
		toHash(config.OPCommandDeposit),
		payload,
	)

	status, body := e.processAction(action)
	if status != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, status, body)
	}

	var result teetypes.ActionResult
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("failed to unmarshal ActionResult: %v", err)
	}
	if result.Status != 1 {
		t.Fatalf("expected ActionResult.Status=1 (success), got %d: %s", result.Status, result.Log)
	}

	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.depositCount != 1 {
		t.Errorf("expected depositCount=1, got %d", e.depositCount)
	}
	if !contains(e.lastDeposit, depositor.Hex()) {
		t.Errorf("expected lastDeposit to contain depositor address, got %q", e.lastDeposit)
	}
}

func TestProcessDeposit_InvalidPayload(t *testing.T) {
	e := newTestExtension()

	action := buildTestAction(
		toHash(config.OPTypePredictionMarket),
		toHash(config.OPCommandDeposit),
		[]byte{0x01, 0x02}, // too short to be a valid (address,uint256) ABI tuple
	)

	status, body := e.processAction(action)
	if status != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, status)
	}

	var result teetypes.ActionResult
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if result.Status != 0 {
		t.Fatalf("expected ActionResult.Status=0 (error), got %d", result.Status)
	}
	if !contains(result.Log, "decoding request") {
		t.Errorf("expected log to mention 'decoding request', got %q", result.Log)
	}
}

func TestProcessPlaceBet_DecryptFails(t *testing.T) {
	e := newTestExtension() // signPort=1: nothing listens there, decrypt call fails fast

	bettor := common.HexToAddress("0xed2B5717c9b936ecC76d75401026A99143e278F5")
	payload := abiEncodePlaceBet(bettor, 0, []byte("not-real-ciphertext"))
	action := buildTestAction(
		toHash(config.OPTypePredictionMarket),
		toHash(config.OPCommandPlaceBet),
		payload,
	)

	status, body := e.processAction(action)
	if status != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, status)
	}

	var result teetypes.ActionResult
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if result.Status != 0 {
		t.Fatalf("expected ActionResult.Status=0 (decrypt call should fail), got %d: %s", result.Status, result.Log)
	}
	if !contains(result.Log, "decrypting bet") {
		t.Errorf("expected log to mention 'decrypting bet', got %q", result.Log)
	}
}

func TestProcessAction_ValidSettle(t *testing.T) {
	e := newTestExtension()

	contractAddr := common.HexToAddress("0x072A3A0C04Cf8CDcaf5B4A73a4Ed4fF5A841531f")
	payload := abiEncodeSettle(0, contractAddr, true, 12345)
	action := buildTestAction(
		toHash(config.OPTypePredictionMarket),
		toHash(config.OPCommandSettle),
		payload,
	)

	status, body := e.processAction(action)
	if status != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, status, body)
	}

	var result teetypes.ActionResult
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("failed to unmarshal ActionResult: %v", err)
	}
	if result.Status != 1 {
		t.Fatalf("expected ActionResult.Status=1 (success), got %d: %s", result.Status, result.Log)
	}

	// resultData must decode back to the same 4-field shape PredictionMarket.sol's
	// _verifyAndDecodeSettleResult expects: (uint256, address, bool, uint256).
	args := abi.Arguments{types.SettleMessageArg}
	unpacked, err := args.Unpack(result.Data)
	if err != nil {
		t.Fatalf("failed to ABI-decode result.Data: %v", err)
	}
	if len(unpacked) != 1 {
		t.Fatalf("expected 1 unpacked tuple, got %d", len(unpacked))
	}

	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.settleCount != 1 {
		t.Errorf("expected settleCount=1, got %d", e.settleCount)
	}
}

// TestProcessAction_ValidWithdraw exercises the full sign path against a fake
// /sign server (standing in for tee-node's signWithTeeHandler), verifying
// processWithdraw packs (amount, to, withdrawalId), calls out for a
// signature, and returns all four fields PredictionMarket.executeWithdrawal()
// needs inside ActionResult.Data.
func TestProcessAction_ValidWithdraw(t *testing.T) {
	fakeSignature := []byte("fake-signature-for-plumbing-test-only")

	teeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sign" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var req teetypes.SignRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decoding sign request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(teetypes.SignResponse{Message: req.Message, Signature: fakeSignature})
	}))
	defer teeServer.Close()

	signPort := teeServer.Listener.Addr().(*net.TCPAddr).Port
	e := &Extension{betLedger: make(map[string][]types.Bet), signPort: signPort}

	requester := common.HexToAddress("0xed2B5717c9b936ecC76d75401026A99143e278F5")
	to := common.HexToAddress("0x1111111111111111111111111111111111111111")
	instructionID := common.HexToHash("0xdeadbeef")
	payload := abiEncodeWithdraw(requester, 500000, to)
	action := buildTestActionWithInstructionID(
		toHash(config.OPTypePredictionMarket),
		toHash(config.OPCommandWithdraw),
		payload,
		instructionID,
	)

	status, body := e.processAction(action)
	if status != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, status, body)
	}

	var result teetypes.ActionResult
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("failed to unmarshal ActionResult: %v", err)
	}
	if result.Status != 1 {
		t.Fatalf("expected ActionResult.Status=1 (success), got %d: %s", result.Status, result.Log)
	}

	var resp types.WithdrawResponse
	if err := json.Unmarshal(result.Data, &resp); err != nil {
		t.Fatalf("failed to unmarshal WithdrawResponse from result.Data: %v", err)
	}
	if resp.Amount == nil || resp.Amount.Cmp(big.NewInt(500000)) != 0 {
		t.Errorf("expected amount=500000, got %v", resp.Amount)
	}
	if resp.To != to {
		t.Errorf("expected to=%s, got %s", to, resp.To)
	}
	if resp.WithdrawalID != instructionID {
		t.Errorf("expected withdrawalId=%s, got %s", instructionID, resp.WithdrawalID)
	}
	if !bytes.Equal(resp.Signature, fakeSignature) {
		t.Errorf("expected signature=%x, got %x", fakeSignature, resp.Signature)
	}
}

// TestProcessWithdraw_SignFails mirrors TestProcessPlaceBet_DecryptFails: with
// no sign server listening, processWithdraw's HTTP call to /sign must fail
// fast and surface as ActionResult.Status=0, not panic or hang.
func TestProcessWithdraw_SignFails(t *testing.T) {
	e := newTestExtension() // signPort=1: nothing listens there, sign call fails fast

	requester := common.HexToAddress("0xed2B5717c9b936ecC76d75401026A99143e278F5")
	to := common.HexToAddress("0x1111111111111111111111111111111111111111")
	payload := abiEncodeWithdraw(requester, 500000, to)
	action := buildTestAction(
		toHash(config.OPTypePredictionMarket),
		toHash(config.OPCommandWithdraw),
		payload,
	)

	status, body := e.processAction(action)
	if status != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, status)
	}

	var result teetypes.ActionResult
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if result.Status != 0 {
		t.Fatalf("expected ActionResult.Status=0 (sign call should fail), got %d: %s", result.Status, result.Log)
	}
	if !contains(result.Log, "signing withdrawal") {
		t.Errorf("expected log to mention 'signing withdrawal', got %q", result.Log)
	}
}

// --- State Tracking ---

func TestProcessAction_DepositCountIncrementsAcrossCalls(t *testing.T) {
	e := newTestExtension()
	depositor := common.HexToAddress("0xed2B5717c9b936ecC76d75401026A99143e278F5")

	for i := 1; i <= 3; i++ {
		payload := abiEncodeDeposit(depositor, int64(i))
		action := buildTestAction(
			toHash(config.OPTypePredictionMarket),
			toHash(config.OPCommandDeposit),
			payload,
		)

		status, _ := e.processAction(action)
		if status != http.StatusOK {
			t.Fatalf("call %d: expected status %d, got %d", i, http.StatusOK, status)
		}
	}

	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.depositCount != 3 {
		t.Errorf("expected depositCount=3, got %d", e.depositCount)
	}
}

func TestProcessAction_InvalidDataMessage(t *testing.T) {
	e := newTestExtension()

	// Data.Message is not valid JSON — processorutils.Parse should fail
	action := teetypes.Action{
		Data: teetypes.ActionData{
			ID:      common.HexToHash("0xabcd"),
			Message: []byte(`not json at all`),
		},
	}

	status, body := e.processAction(action)

	if status != http.StatusBadRequest {
		t.Fatalf("expected status %d for invalid Data.Message, got %d: %s",
			http.StatusBadRequest, status, body)
	}

	bodyStr := string(body)
	if !contains(bodyStr, "decoding fixed data") {
		t.Errorf("expected body to mention 'decoding fixed data', got %q", bodyStr)
	}
	t.Logf("400 body: %s", bodyStr)
}

// contains is a simple helper to check substring presence.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
