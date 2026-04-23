package fccutils

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"os"
	"strings"
	"extension-scaffold/tools/pkg/support"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/teemachineregistry"
	"github.com/flare-foundation/go-flare-common/pkg/encoding"
	"github.com/flare-foundation/go-flare-common/pkg/logger"
	"github.com/flare-foundation/tee-node/pkg/fdc"
	"github.com/flare-foundation/tee-node/pkg/types"
	"github.com/pkg/errors"
	"time"
)

// registrationState tracks progress through the multi-step registration flow.
// Saved to a state file after each step so registration can resume after failures.
type registrationState struct {
	CompletedSteps         string      `json:"completed_steps"`
	TeeAttestInstructionID common.Hash `json:"tee_attest_instruction_id"`
	InstructionID          common.Hash `json:"instruction_id"`
}

func loadState(path string) (*registrationState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &registrationState{}, nil
		}
		return nil, errors.Errorf("failed to read state file: %s", err)
	}
	var state registrationState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, errors.Errorf("failed to parse state file: %s", err)
	}
	return &state, nil
}

func saveState(path string, state *registrationState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return errors.Errorf("failed to marshal state: %s", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return errors.Errorf("failed to write state file: %s", err)
	}
	return nil
}

func RegisterNode(s *support.Support, teeInfo *types.SignedTeeInfoResponse, hostURL, ftdcTeeURL string, ftdcTee common.Address, command, instructionIDstring, stateFilePath string) error {
	teeID, proxyID, err := TeeProxyId(teeInfo)
	if err != nil {
		return err
	}

	// Load existing state for resume support
	state, err := loadState(stateFilePath)
	if err != nil {
		return err
	}
	if state.CompletedSteps != "" {
		logger.Infof("Resuming registration from state file (completed: %s)", state.CompletedSteps)
	}

	var teeAttestInstructionID common.Hash
	if strings.Contains(command, "r") {
		if strings.Contains(state.CompletedSteps, "r") {
			logger.Infof("Pre-registration already completed, skipping (from state file)")
			teeAttestInstructionID = state.TeeAttestInstructionID
		} else {
			// Check if machine is already registered on-chain
			callOpts := &bind.CallOpts{Context: context.Background()}
			machineInfo, machineErr := s.TeeMachineRegistry.GetTeeMachine(callOpts, teeID)
			if machineErr == nil && machineInfo.TeeId != (common.Address{}) {
				logger.Infof("TEE machine %s already registered on-chain, skipping pre-registration", teeID.Hex())
			} else {
				_, teeAttestInstructionID, err = PreRegistration(s, hostURL, teeID, proxyID, teeInfo)
				if err != nil {
					return err
				}
			}
			state.CompletedSteps += "r"
			state.TeeAttestInstructionID = teeAttestInstructionID
			if saveErr := saveState(stateFilePath, state); saveErr != nil {
				logger.Warnf("WARNING: failed to save state: %v", saveErr)
			}
		}
		time.Sleep(1 * time.Second)
	}

	if strings.Contains(command, "R") {
		if strings.Contains(state.CompletedSteps, "R") {
			logger.Infof("TEE attestation request already completed, skipping (from state file)")
			teeAttestInstructionID = state.TeeAttestInstructionID
		} else {
			teeAttestInstructionID, err = RequestTeeAttestation(s, teeID)
			if err != nil {
				return err
			}
			state.CompletedSteps += "R"
			state.TeeAttestInstructionID = teeAttestInstructionID
			if saveErr := saveState(stateFilePath, state); saveErr != nil {
				logger.Warnf("WARNING: failed to save state: %v", saveErr)
			}
		}
		time.Sleep(1 * time.Second)
	}

	var instructionID common.Hash
	if strings.Contains(command, "a") {
		if strings.Contains(state.CompletedSteps, "a") {
			logger.Infof("FTDC availability check already completed, skipping (from state file)")
			instructionID = state.InstructionID
		} else {
			instructionID, err = RequestFTDCAvailabilityCheck(s, teeID, ftdcTee, teeAttestInstructionID)
			if err != nil {
				return err
			}
			state.CompletedSteps += "a"
			state.InstructionID = instructionID
			if saveErr := saveState(stateFilePath, state); saveErr != nil {
				logger.Warnf("WARNING: failed to save state: %v", saveErr)
			}
		}
		time.Sleep(1 * time.Second)
	} else {
		instructionID = common.HexToHash(instructionIDstring)
	}

	if strings.Contains(command, "p") {
		toProductionProof, err := GetFTDCAvailabilityCheckResult(ftdcTeeURL, instructionID)
		if err != nil {
			return err
		}

		err = ToProduction(s, toProductionProof)
		if err != nil {
			return err
		}
	}

	// All steps completed — delete state file
	os.Remove(stateFilePath)
	return nil
}

func PreRegistration(
	s *support.Support,
	hostURL string,
	teeID common.Address,
	proxyID common.Address,
	teeInfo *types.SignedTeeInfoResponse,
) ([32]byte, common.Hash, error) {
	opts, err := bind.NewKeyedTransactorWithChainID(s.Prv, s.ChainID)
	if err != nil {
		return [32]byte{}, common.Hash{}, errors.Errorf("%s", err)
	}
	opts.Value = big.NewInt(int64(1000000000))

	teeMachineDataRegistry := teemachineregistry.ITeeMachineRegistryTeeMachineData{
		ExtensionId:  new(big.Int).SetBytes(teeInfo.MachineData.ExtensionID.Bytes()),
		InitialOwner: teeInfo.MachineData.InitialOwner,
		CodeHash:     teeInfo.MachineData.CodeHash,
		Platform:     teeInfo.MachineData.Platform,
		PublicKey:    teemachineregistry.PublicKey{X: teeInfo.MachineData.PublicKey.X, Y: teeInfo.MachineData.PublicKey.Y},
	}

	if len(teeInfo.DataSignature) != 65 {
		return [32]byte{}, common.Hash{}, errors.New("signature error")
	}
	sigVRS := encoding.TransformSignatureRSVtoVRS(teeInfo.DataSignature)

	signature := teemachineregistry.Signature{
		V: sigVRS[0],
		R: [32]byte(sigVRS[1:33]),
		S: [32]byte(sigVRS[33:65]),
	}

	claimBackAddress := crypto.PubkeyToAddress(s.Prv.PublicKey)
	tx, err := s.TeeMachineRegistry.Register(opts, teeMachineDataRegistry, signature, proxyID, hostURL, claimBackAddress)
	if err != nil {
		return [32]byte{}, common.Hash{}, errors.Errorf("error: %s", err)
	}

	receipt, err := support.CheckTx(tx, s.ChainClient)
	if err != nil {
		return [32]byte{}, common.Hash{}, err
	}
	logger.Infof("(pre)registration of TEE with ID %s succeeded", hex.EncodeToString(teeID[:]))

	if len(receipt.Logs) < 2 {
		return common.Hash{}, common.Hash{}, errors.New("unexpected logs, this should not happen")
	}
	attestEvent, err := s.TeeVerification.ParseTeeAttestationRequested(*receipt.Logs[1])
	if err != nil {
		return [32]byte{}, common.Hash{}, errors.Errorf("failed to parse TeeAttestationRequested event: %s", err)
	}
	challenge := attestEvent.Challenge

	event, err := s.TeeExtensionRegistry.ParseTeeInstructionsSent(*receipt.Logs[0])
	if err != nil {
		return common.Hash{}, common.Hash{}, errors.Errorf("failed to parse TeeInstructionsSent event: %s", err)
	}
	instructionID := common.Hash(event.InstructionId)
	logger.Infof("tee-attestation requested, instructionId: %s", hex.EncodeToString(instructionID[:]))

	return challenge, instructionID, nil
}

func RequestTeeAttestation(s *support.Support, teeID common.Address) (common.Hash, error) {
	opts, err := bind.NewKeyedTransactorWithChainID(s.Prv, s.ChainID)
	if err != nil {
		return [32]byte{}, errors.Errorf("%s", err)
	}
	opts.Value = big.NewInt(int64(1000000000))

	claimBackAddress := crypto.PubkeyToAddress(s.Prv.PublicKey)
	tx, err := s.TeeVerification.RequestTeeAttestation(opts, teeID, claimBackAddress)
	if err != nil {
		return [32]byte{}, errors.Errorf("error: %s", err)
	}

	receipt, err := support.CheckTx(tx, s.ChainClient)
	if err != nil {
		return [32]byte{}, err
	}

	if len(receipt.Logs) < 2 {
		return common.Hash{}, errors.New("unexpected logs, this should not happen")
	}
	event, err := s.TeeExtensionRegistry.ParseTeeInstructionsSent(*receipt.Logs[0])
	if err != nil {
		return common.Hash{}, errors.Errorf("failed to parse TeeInstructionsSent event: %s", err)
	}
	instructionID := common.Hash(event.InstructionId)
	logger.Infof("tee attestation requested, instructionId: %s", hex.EncodeToString(instructionID[:]))

	return instructionID, nil
}

func RequestFTDCAvailabilityCheck(s *support.Support, teeID, externalTeeID common.Address, teeAttestInstructionID [32]byte) (common.Hash, error) {
	opts, err := bind.NewKeyedTransactorWithChainID(s.Prv, s.ChainID)
	if err != nil {
		return common.Hash{}, errors.Errorf("%s", err)
	}
	opts.Value = big.NewInt(int64(1000000000))

	claimBackAddress := crypto.PubkeyToAddress(s.Prv.PublicKey)
	proofOwner := claimBackAddress
	tx, err := s.TeeVerification.RequestAvailabilityCheckAttestation(opts, teeID, teeAttestInstructionID, externalTeeID, proofOwner, claimBackAddress)
	if err != nil {
		return common.Hash{}, errors.Errorf("%s", err)
	}
	receipt, err := support.CheckTx(tx, s.ChainClient)
	if err != nil {
		return common.Hash{}, errors.Errorf("%s", err)
	}
	if len(receipt.Logs) == 0 {
		return common.Hash{}, errors.New("no logs found in receipt")
	}
	event, err := s.TeeExtensionRegistry.ParseTeeInstructionsSent(*receipt.Logs[0])
	if err != nil {
		return common.Hash{}, errors.Errorf("failed to parse TeeInstructionsSent event: %s", err)
	}
	instructionID := common.Hash(event.InstructionId)

	logger.Infof("availability check sent, instructionId: %s", hex.EncodeToString(instructionID[:]))

	return instructionID, nil
}

func GetFTDCAvailabilityCheckResult(hostURL string, instructionId common.Hash) (*teemachineregistry.ITeeAvailabilityCheckProof, error) {
	actionResult, err := ActionResult(hostURL, instructionId)
	if err != nil {
		return nil, err
	}
	var ftdcProof fdc.ProveResponse
	err = json.Unmarshal(actionResult.Result.Data, &ftdcProof)
	if err != nil {
		return nil, errors.Errorf("%s", err)
	}

	header, err := fdc.DecodeResponse(ftdcProof.ResponseHeader)
	if err != nil {
		return nil, errors.Errorf("%s", err)
	}

	request, err := DecodeFTDCTeeAvailabilityCheckRequest(ftdcProof.RequestBody)
	if err != nil {
		return nil, errors.Errorf("%s", err)
	}
	response, err := DecodeFTDCTeeAvailabilityCheckResponse(ftdcProof.ResponseBody)
	if err != nil {
		return nil, errors.Errorf("%s", err)
	}

	toProductionProof := teemachineregistry.ITeeAvailabilityCheckProof{
		Signatures:  teemachineregistry.IFdc2VerificationFdc2Signatures{SigningPolicySignatures: ftdcProof.DataProviderSignatures},
		Header:      teemachineregistry.IFdc2HubFdc2ResponseHeader(header),
		RequestBody: teemachineregistry.ITeeAvailabilityCheckRequestBody(request),
		ResponseBody: teemachineregistry.ITeeAvailabilityCheckResponseBody{
			Status:                 response.Status,
			TeeTimestamp:           response.TeeTimestamp,
			CodeHash:               response.CodeHash,
			Platform:               response.Platform,
			InitialSigningPolicyId: response.InitialSigningPolicyId,
			LastSigningPolicyId:    response.LastSigningPolicyId,
			State:                  teemachineregistry.ITeeAvailabilityCheckTeeState(response.State),
		},
	}

	logger.Infof("availability check proof obtained")

	return &toProductionProof, nil
}

func ToProduction(s *support.Support, toProductionProof *teemachineregistry.ITeeAvailabilityCheckProof) error {
	opts, err := bind.NewKeyedTransactorWithChainID(s.Prv, s.ChainID)
	if err != nil {
		return errors.Errorf("%s", err)
	}

	tx, err := s.TeeMachineRegistry.ToProduction(opts, *toProductionProof)
	if err != nil {
		return errors.Errorf("%s", err)
	}
	_, err = support.CheckTx(tx, s.ChainClient)
	if err != nil {
		return errors.Errorf("%s", err)
	}

	teeMachineInfo, err := s.TeeMachineRegistry.GetTeeMachine(nil, toProductionProof.RequestBody.TeeId)
	if err != nil {
		return errors.Errorf("%s", err)
	}
	if teeMachineInfo.TeeId != toProductionProof.RequestBody.TeeId {
		return errors.New("tee machine not set up correctly")
	}

	return nil
}

func AddTeeVersion(s *support.Support, privKey *ecdsa.PrivateKey, extensionId *big.Int, codeHash common.Hash, platform common.Hash, governanceHash common.Hash, version string) error {
	opts, err := bind.NewKeyedTransactorWithChainID(privKey, s.ChainID)
	if err != nil {
		return errors.Errorf("%s", err)
	}

	tx, err := s.TeeExtensionRegistry.AddTeeVersion(opts, extensionId, version, codeHash, [][32]byte{platform}, governanceHash)
	if err != nil {
		return errors.Errorf("TeeExtensionRegistry.AddTeeVersion failed: %s", err)
	}

	_, err = support.CheckTx(tx, s.ChainClient)
	if err != nil {
		return errors.Errorf("%s", err)
	}

	return nil
}
