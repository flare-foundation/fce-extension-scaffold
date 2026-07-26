package extension

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"sync"

	"extension-scaffold/internal/config"
	"extension-scaffold/pkg/types"

	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-flare-common/pkg/tee/instruction"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs"
	teetypes "github.com/flare-foundation/tee-node/pkg/types"
	teeutils "github.com/flare-foundation/tee-node/pkg/utils"

	"github.com/flare-foundation/tee-node/pkg/processorutils"
)

type Extension struct {
	mu     sync.RWMutex
	Server *http.Server

	signPort int

	// betLedger holds bets per marketId (decimal string), decrypted at PLACE_BET
	// time and consumed at SETTLE time — mirrors extension/src/server.ts's
	// in-memory betLedger.
	betLedger map[string][]types.Bet

	depositCount int
	lastDeposit  string
	settleCount  int
}

// --- DO NOT MODIFY: New(), actionHandler() are boilerplate.
func New(extensionPort, signPort int) *Extension {
	e := &Extension{
		signPort:  signPort,
		betLedger: make(map[string][]types.Bet),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /state", e.stateHandler)
	mux.HandleFunc("POST /action", e.actionHandler)

	e.Server = &http.Server{Addr: fmt.Sprintf(":%d", extensionPort), Handler: mux}
	return e
}

// stateHandler() structure is boilerplate but update the State field mapping to match your Extension fields.
func (e *Extension) stateHandler(w http.ResponseWriter, r *http.Request) {
	e.mu.RLock()
	betCount := 0
	for _, bets := range e.betLedger {
		betCount += len(bets)
	}
	stateResponse := types.StateResponse{
		StateVersion: teeutils.ToHash(config.Version),
		State: types.State{
			DepositCount: e.depositCount,
			LastDeposit:  e.lastDeposit,
			BetCount:     betCount,
			SettleCount:  e.settleCount,
		},
	}
	e.mu.RUnlock()

	err := json.NewEncoder(w).Encode(stateResponse)
	if err != nil {
		http.Error(w, fmt.Sprintf("sending response: %v", err), http.StatusInternalServerError)
		return
	}
}

func (e *Extension) processAction(action teetypes.Action) (int, []byte) {
	dataFixed, err := processorutils.Parse[instruction.DataFixed](action.Data.Message)
	if err != nil {
		return http.StatusBadRequest, []byte(fmt.Sprintf("decoding fixed data: %v", err))
	}

	switch {
	case dataFixed.OPType == teeutils.ToHash(config.OPTypePredictionMarket):
		return e.processPredictionMarket(action, dataFixed)

	default:
		return http.StatusNotImplemented, []byte(fmt.Sprintf(
			"unsupported op type: received %s, expected %s (%s)",
			dataFixed.OPType.Hex(), teeutils.ToHash(config.OPTypePredictionMarket).Hex(), config.OPTypePredictionMarket,
		))
	}
}

// processPredictionMarket routes PREDICTION_MARKET instructions by OPCommand.
// Mirrors extension/src/server.ts's handlePredictionMarket in the TypeScript
// reference implementation (flare-prediction-market repo) — same four commands,
// same wire shapes, ported to the scaffold's Go extension pattern.
func (e *Extension) processPredictionMarket(action teetypes.Action, df *instruction.DataFixed) (int, []byte) {
	switch {
	case df.OPCommand == teeutils.ToHash(config.OPCommandDeposit):
		ar := e.processDeposit(action, df)
		b, _ := json.Marshal(ar)
		return http.StatusOK, b

	case df.OPCommand == teeutils.ToHash(config.OPCommandPlaceBet):
		ar := e.processPlaceBet(action, df)
		b, _ := json.Marshal(ar)
		return http.StatusOK, b

	case df.OPCommand == teeutils.ToHash(config.OPCommandSettle):
		ar := e.processSettle(action, df)
		b, _ := json.Marshal(ar)
		return http.StatusOK, b

	case df.OPCommand == teeutils.ToHash(config.OPCommandWithdraw):
		ar := e.processWithdraw(action, df)
		b, _ := json.Marshal(ar)
		return http.StatusOK, b

	default:
		return http.StatusNotImplemented, []byte(fmt.Sprintf(
			"unsupported op command: received %s, expected one of [%s, %s, %s, %s]",
			df.OPCommand.Hex(),
			config.OPCommandDeposit, config.OPCommandPlaceBet, config.OPCommandSettle, config.OPCommandWithdraw,
		))
	}
}

// processDeposit handles DEPOSIT instructions: PredictionMarket.deposit() already
// moved the ERC-20 funds on-chain (transferFrom) before sending this instruction —
// there is nothing left to verify here, it's an acknowledgement/log point,
// matching the TS reference's handling of OP_DEPOSIT.
func (e *Extension) processDeposit(action teetypes.Action, df *instruction.DataFixed) teetypes.ActionResult {
	var req types.DepositRequest
	if err := structs.DecodeTo(types.DepositMessageArg, df.OriginalMessage, &req); err != nil {
		return buildResult(action, df, nil, 0, fmt.Errorf("decoding request: %w", err))
	}

	e.mu.Lock()
	e.depositCount++
	e.lastDeposit = fmt.Sprintf("depositor=%s amount=%s", req.Depositor.Hex(), req.Amount.String())
	e.mu.Unlock()

	return buildResult(action, df, nil, 1, nil)
}

// processPlaceBet handles PLACE_BET instructions: decrypts EncryptedBet via the
// TEE's own key (POST /decrypt on the sign port — the TEE node decrypts with the
// same private key whose public key is published in /info, so the caller must
// have encrypted against that key) and records the bet in the in-memory ledger.
func (e *Extension) processPlaceBet(action teetypes.Action, df *instruction.DataFixed) teetypes.ActionResult {
	var req types.PlaceBetRequest
	if err := structs.DecodeTo(types.PlaceBetMessageArg, df.OriginalMessage, &req); err != nil {
		return buildResult(action, df, nil, 0, fmt.Errorf("decoding request: %w", err))
	}

	plaintext, err := e.decryptViaTee(req.EncryptedBet)
	if err != nil {
		return buildResult(action, df, nil, 0, fmt.Errorf("decrypting bet: %w", err))
	}

	var payload types.BetPayload
	if err := structs.DecodeTo(types.BetPayloadArg, plaintext, &payload); err != nil {
		return buildResult(action, df, nil, 0, fmt.Errorf("decoding decrypted bet payload: %w", err))
	}

	marketKey := req.MarketId.String()
	e.mu.Lock()
	e.betLedger[marketKey] = append(e.betLedger[marketKey], types.Bet{
		Bettor: req.Bettor,
		IsUp:   payload.IsUp,
		Amount: payload.Amount.String(),
	})
	e.mu.Unlock()

	return buildResult(action, df, nil, 1, nil)
}

// processSettle handles SETTLE instructions. The outcome/referenceValue have
// already been resolved on-chain (FTSO read or FDC proof) before this
// instruction is sent — see PredictionMarket.sol's requestPriceSettlement/
// requestWeatherSettlement. This handler's job is limited to echoing the
// settlement fields back as ActionResult.Data; PredictionMarket.sol's
// _verifyAndDecodeSettleResult() ABI-decodes them as
// (uint256 marketId, address contractAddr, bool outcome, uint256 referenceValue) —
// all four fields, matching SettleMessage exactly. The framework signs this
// ActionResult automatically (tee-node's router wraps + signs whatever we
// return here), so no manual /sign call is needed.
//
// Payout computation against the decrypted bet ledger is not wired up yet —
// this mirrors the TS reference implementation's current scope (proves the
// sign-and-return path).
func (e *Extension) processSettle(action teetypes.Action, df *instruction.DataFixed) teetypes.ActionResult {
	var req types.SettleRequest
	if err := structs.DecodeTo(types.SettleMessageArg, df.OriginalMessage, &req); err != nil {
		return buildResult(action, df, nil, 0, fmt.Errorf("decoding request: %w", err))
	}

	e.mu.Lock()
	e.settleCount++
	e.mu.Unlock()

	resultData, err := structs.Encode(types.SettleMessageArg, req)
	if err != nil {
		return buildResult(action, df, nil, 0, fmt.Errorf("encoding result: %w", err))
	}

	return buildResult(action, df, resultData, 1, nil)
}

// processWithdraw handles WITHDRAW instructions: builds the TEE-authorized
// withdrawal signature over abi.encodePacked(amount, to, withdrawalId) and
// returns it (JSON) as ActionResult.Data, ported from fce-orderbook's
// internal/extension/withdraw.go. PredictionMarket.sol has a single fixed
// payToken (not a per-call token like fce-orderbook's vault), so the signed
// message drops the token field: contract-side, executeWithdrawal() verifies
// keccak256(abi.encodePacked(amount, to, withdrawalId)) via _recoverEthSigned.
//
// Note: this signature lives in the JSON body of ActionResult.Data, not in
// the outer /action/result response's top-level `signature` field — that
// outer field is tee-node's automatic TEE_ACTION_RESULT domain-separated
// signature over the whole ActionResult, which executeWithdrawal() does not
// verify against.
func (e *Extension) processWithdraw(action teetypes.Action, df *instruction.DataFixed) teetypes.ActionResult {
	var req types.WithdrawRequest
	if err := structs.DecodeTo(types.WithdrawMessageArg, df.OriginalMessage, &req); err != nil {
		return buildResult(action, df, nil, 0, fmt.Errorf("decoding request: %w", err))
	}

	withdrawalID := df.InstructionID
	message := packWithdrawalMessage(req.Amount, req.To, withdrawalID)

	sig, err := e.signWithTEE(message)
	if err != nil {
		return buildResult(action, df, nil, 0, fmt.Errorf("signing withdrawal: %w", err))
	}

	resp := types.WithdrawResponse{
		Amount:       req.Amount,
		To:           req.To,
		WithdrawalID: withdrawalID,
		Signature:    sig,
	}
	data, err := json.Marshal(resp)
	if err != nil {
		return buildResult(action, df, nil, 0, fmt.Errorf("encoding response: %w", err))
	}

	return buildResult(action, df, data, 1, nil)
}

// packWithdrawalMessage returns abi.encodePacked(amount, to, withdrawalId) as
// raw bytes (32 + 20 + 32 = 84 bytes), matching PredictionMarket.sol's
// executeWithdrawal(): keccak256(abi.encodePacked(amount, to, withdrawalId)).
// signWithTEE keccak256's this input and EIP-191-signs the digest, which is
// what the contract's _recoverEthSigned expects.
func packWithdrawalMessage(amount *big.Int, to common.Address, withdrawalID common.Hash) []byte {
	buf := make([]byte, 0, 84)

	amountBytes := make([]byte, 32)
	amount.FillBytes(amountBytes)
	buf = append(buf, amountBytes...)

	buf = append(buf, to.Bytes()...)
	buf = append(buf, withdrawalID.Bytes()...)

	return buf
}

// signWithTEE calls the tee-node's own sign-port /sign endpoint (loopback
// only, within the same container) to EIP-191-sign keccak256(message) with
// the TEE's own private key — see tee-node's internal/extension/server/server.go
// (signWithTeeHandler). Mirrors decryptViaTee's call pattern for /decrypt.
func (e *Extension) signWithTEE(message []byte) ([]byte, error) {
	reqBody, err := json.Marshal(teetypes.SignRequest{Message: message})
	if err != nil {
		return nil, fmt.Errorf("marshaling sign request: %w", err)
	}

	url := fmt.Sprintf("http://localhost:%d/sign", e.signPort)
	resp, err := http.Post(url, "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("calling %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sign endpoint returned status %d", resp.StatusCode)
	}

	var signResp teetypes.SignResponse
	if err := json.NewDecoder(resp.Body).Decode(&signResp); err != nil {
		return nil, fmt.Errorf("decoding sign response: %w", err)
	}

	return signResp.Signature, nil
}

// decryptViaTee calls the tee-node's own sign-port /decrypt endpoint (loopback
// only, within the same container) to decrypt ciphertext with the TEE's own
// private key — the same key whose public key is published via /info. See
// github.com/flare-foundation/tee-node's internal/extension/server.go
// (decryptWithTeeHandler).
func (e *Extension) decryptViaTee(cipher []byte) ([]byte, error) {
	reqBody, err := json.Marshal(teetypes.DecryptRequest{EncryptedMessage: cipher})
	if err != nil {
		return nil, fmt.Errorf("marshaling decrypt request: %w", err)
	}

	url := fmt.Sprintf("http://localhost:%d/decrypt", e.signPort)
	resp, err := http.Post(url, "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("calling %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("decrypt endpoint returned status %d", resp.StatusCode)
	}

	var decrypted teetypes.DecryptResponse
	if err := json.NewDecoder(resp.Body).Decode(&decrypted); err != nil {
		return nil, fmt.Errorf("decoding decrypt response: %w", err)
	}

	return decrypted.DecryptedMessage, nil
}
