package types

import "extension-scaffold/pkg/decoder"

// RegisterDecoders registers all type decoders for this extension.
// Extension developers: add new registrations here for each OPType/OPCommand.
func RegisterDecoders(r *decoder.Registry) {
	// DEPOSIT message (ABI-encoded)
	r.Register(
		decoder.RegistryKey{OPType: "PREDICTION_MARKET", OPCommand: "DEPOSIT", Kind: decoder.KindMessage},
		decoder.NewABIDecoder[DepositRequest](DepositMessageArg),
	)
	// PLACE_BET message (ABI-encoded; EncryptedBet stays ciphertext at this layer)
	r.Register(
		decoder.RegistryKey{OPType: "PREDICTION_MARKET", OPCommand: "PLACE_BET", Kind: decoder.KindMessage},
		decoder.NewABIDecoder[PlaceBetRequest](PlaceBetMessageArg),
	)
	// SETTLE message (ABI-encoded)
	r.Register(
		decoder.RegistryKey{OPType: "PREDICTION_MARKET", OPCommand: "SETTLE", Kind: decoder.KindMessage},
		decoder.NewABIDecoder[SettleRequest](SettleMessageArg),
	)
	// SETTLE result (ABI-encoded — same shape as the message)
	r.Register(
		decoder.RegistryKey{OPType: "PREDICTION_MARKET", OPCommand: "SETTLE", Kind: decoder.KindResult},
		decoder.NewABIDecoder[SettleRequest](SettleMessageArg),
	)
	// WITHDRAW message (ABI-encoded)
	r.Register(
		decoder.RegistryKey{OPType: "PREDICTION_MARKET", OPCommand: "WITHDRAW", Kind: decoder.KindMessage},
		decoder.NewABIDecoder[WithdrawRequest](WithdrawMessageArg),
	)
}
