package fccutils

import (
	"bytes"
	"context"
	stderrors "errors"
	"fmt"

	"extension-scaffold/tools/pkg/support"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/fdc2/fdc2hub"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/tee/extensionmanager"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/tee/machinemanager"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/tee/verification"
	"github.com/flare-foundation/go-flare-common/pkg/logger"
)

// diagAvailabilityCheckRevert runs an eth_call against TeeVerification with the same
// calldata as RequestAvailabilityCheckAttestation, then decodes the revert payload and
// matches the selector against custom errors from Verification, Fdc2Hub, MachineManager,
// and ExtensionManager. Purely diagnostic — never returns; logs whatever it can resolve.
func diagAvailabilityCheckRevert(
	s *support.Support,
	opts *bind.TransactOpts,
	teeID common.Address,
	instructionID [32]byte,
	externalTeeID, proofOwner, claimBackAddress common.Address,
) {
	verABI, err := verification.VerificationMetaData.GetAbi()
	if err != nil {
		logger.Warnf("diag: load Verification ABI: %v", err)
		return
	}
	calldata, err := verABI.Pack(
		"requestAvailabilityCheckAttestation",
		teeID, instructionID, externalTeeID, proofOwner, claimBackAddress,
	)
	if err != nil {
		logger.Warnf("diag: pack calldata: %v", err)
		return
	}

	from := crypto.PubkeyToAddress(s.Prv.PublicKey)
	msg := ethereum.CallMsg{
		From:  from,
		To:    &s.Addresses.FlareTeeManager,
		Value: opts.Value,
		Data:  calldata,
	}
	logger.Infof("diag: eth_call requestAvailabilityCheckAttestation from=%s to=%s value=%s",
		from.Hex(), s.Addresses.FlareTeeManager.Hex(), opts.Value.String())
	logger.Infof("diag:   teeID=%s instructionID=0x%x externalTeeID=%s proofOwner=%s claimBack=%s",
		teeID.Hex(), instructionID, externalTeeID.Hex(), proofOwner.Hex(), claimBackAddress.Hex())

	_, callErr := s.ChainClient.CallContract(context.Background(), msg, nil)
	if callErr == nil {
		logger.Warnf("diag: eth_call unexpectedly succeeded — revert may be tx-time/gas-related")
		return
	}

	revertData := extractRevertData(callErr)
	if len(revertData) == 0 {
		logger.Warnf("diag: eth_call error has no revert data: %v", callErr)
		return
	}
	if len(revertData) < 4 {
		logger.Warnf("diag: revert data too short to contain a selector: 0x%x", revertData)
		return
	}
	selector := revertData[:4]

	// Standard solidity Error(string) — selector 0x08c379a0
	if bytes.Equal(selector, []byte{0x08, 0xc3, 0x79, 0xa0}) {
		if reason, err := abi.UnpackRevert(revertData); err == nil {
			logger.Errorf("diag: revert reason (string): %q", reason)
			return
		}
	}

	for _, src := range []struct {
		name string
		md   *bind.MetaData
	}{
		{"Verification", verification.VerificationMetaData},
		{"Fdc2Hub", fdc2hub.Fdc2HubMetaData},
		{"MachineManager", machinemanager.MachineManagerMetaData},
		{"ExtensionManager", extensionmanager.ExtensionManagerMetaData},
	} {
		a, err := src.md.GetAbi()
		if err != nil {
			continue
		}
		for errName, eDef := range a.Errors {
			if !bytes.Equal(eDef.ID[:4], selector) {
				continue
			}
			args, unpackErr := eDef.Inputs.Unpack(revertData[4:])
			if unpackErr != nil {
				logger.Errorf("diag: revert -> %s.%s (arg unpack failed: %v) data=0x%x", src.name, errName, unpackErr, revertData)
			} else {
				logger.Errorf("diag: revert -> %s.%s%s", src.name, errName, fmt.Sprintf("(%v)", args))
			}
			return
		}
	}

	logger.Errorf("diag: revert with unknown selector 0x%x; full data: 0x%x", selector, revertData)
}

func extractRevertData(err error) []byte {
	if err == nil {
		return nil
	}
	var dataErr rpc.DataError
	if stderrors.As(err, &dataErr) {
		switch d := dataErr.ErrorData().(type) {
		case string:
			return common.FromHex(d)
		case []byte:
			return d
		}
	}
	return nil
}
