// Package types contains types that could be useful to other apps when interacting with this extension.
package types

import (
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

// DepositRequest is the ABI-decoded payload sent via PredictionMarket.deposit()
// (Solidity DepositMessage{address depositor, uint256 amount}).
type DepositRequest struct {
	Depositor common.Address
	Amount    *big.Int
}

// DepositMessageArg describes the ABI layout of DepositMessage from the Solidity contract.
var DepositMessageArg abi.Argument

// PlaceBetRequest is the ABI-decoded payload sent via PredictionMarket.placeBet()
// (Solidity PlaceBetMessage{address bettor, uint256 marketId, bytes encryptedBet}).
// EncryptedBet is ECIES ciphertext, encrypted against this TEE's public key.
type PlaceBetRequest struct {
	Bettor       common.Address
	MarketId     *big.Int
	EncryptedBet []byte
}

// PlaceBetMessageArg describes the ABI layout of PlaceBetMessage from the Solidity contract.
var PlaceBetMessageArg abi.Argument

// BetPayload is the ABI-decoded plaintext obtained after decrypting a bet's
// EncryptedBet field: abi.encode(bool isUp, uint256 amount).
type BetPayload struct {
	IsUp   bool
	Amount *big.Int
}

// BetPayloadArg describes the ABI layout of the decrypted bet payload.
var BetPayloadArg abi.Argument

// SettleRequest is the ABI-decoded payload sent via PredictionMarket's
// requestPriceSettlement()/requestWeatherSettlement()
// (Solidity SettleMessage{uint256 marketId, address contractAddr, bool outcome, uint256 referenceValue}).
// The same shape is also used to ABI-encode the ActionResult data returned for SETTLE,
// matching what settlePriceMarket()/settleWeatherMarket() decode on-chain.
type SettleRequest struct {
	MarketId       *big.Int
	ContractAddr   common.Address
	Outcome        bool
	ReferenceValue *big.Int
}

// SettleMessageArg describes the ABI layout of SettleMessage from the Solidity contract.
var SettleMessageArg abi.Argument

// WithdrawRequest is the ABI-decoded payload sent via PredictionMarket.withdraw()
// (Solidity abi.encode(msg.sender, amount, to) — no named struct on-chain).
type WithdrawRequest struct {
	Requester common.Address
	Amount    *big.Int
	To        common.Address
}

// WithdrawMessageArg describes the ABI layout of the withdraw request tuple.
var WithdrawMessageArg abi.Argument

// WithdrawResponse is the JSON payload embedded in ActionResult.Data for a
// successful WITHDRAW: the parameters and TEE signature the caller submits to
// PredictionMarket.executeWithdrawal(amount, to, withdrawalId, signature).
type WithdrawResponse struct {
	Amount       *big.Int       `json:"amount"`
	To           common.Address `json:"to"`
	WithdrawalID common.Hash    `json:"withdrawalId"`
	Signature    []byte         `json:"signature"`
}

func init() {
	depositTy, _ := abi.NewType("tuple", "", []abi.ArgumentMarshaling{
		{Name: "Depositor", Type: "address"},
		{Name: "Amount", Type: "uint256"},
	})
	DepositMessageArg = abi.Argument{Type: depositTy}

	placeBetTy, _ := abi.NewType("tuple", "", []abi.ArgumentMarshaling{
		{Name: "Bettor", Type: "address"},
		{Name: "MarketId", Type: "uint256"},
		{Name: "EncryptedBet", Type: "bytes"},
	})
	PlaceBetMessageArg = abi.Argument{Type: placeBetTy}

	betPayloadTy, _ := abi.NewType("tuple", "", []abi.ArgumentMarshaling{
		{Name: "IsUp", Type: "bool"},
		{Name: "Amount", Type: "uint256"},
	})
	BetPayloadArg = abi.Argument{Type: betPayloadTy}

	settleTy, _ := abi.NewType("tuple", "", []abi.ArgumentMarshaling{
		{Name: "MarketId", Type: "uint256"},
		{Name: "ContractAddr", Type: "address"},
		{Name: "Outcome", Type: "bool"},
		{Name: "ReferenceValue", Type: "uint256"},
	})
	SettleMessageArg = abi.Argument{Type: settleTy}

	withdrawTy, _ := abi.NewType("tuple", "", []abi.ArgumentMarshaling{
		{Name: "Requester", Type: "address"},
		{Name: "Amount", Type: "uint256"},
		{Name: "To", Type: "address"},
	})
	WithdrawMessageArg = abi.Argument{Type: withdrawTy}
}

// Bet is one recorded bet in the in-memory ledger for a market.
type Bet struct {
	Bettor common.Address `json:"bettor"`
	IsUp   bool           `json:"isUp"`
	Amount string         `json:"amount"`
}

// State holds the extension's observable state, returned by GET /state.
type State struct {
	DepositCount int    `json:"depositCount"`
	LastDeposit  string `json:"lastDeposit"`
	BetCount     int    `json:"betCount"`
	SettleCount  int    `json:"settleCount"`
}

// --- DO NOT MODIFY below this line. ---

// StateResponse is the envelope returned by GET /state.
type StateResponse struct {
	StateVersion common.Hash `json:"stateVersion"`
	State        State       `json:"state"`
}
