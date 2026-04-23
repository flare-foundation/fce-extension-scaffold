package fccutils

import (
	"context"
	"math/big"
	"extension-scaffold/tools/pkg/support"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/teeextensionregistry"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/teeownerallowlist"
	"github.com/flare-foundation/go-flare-common/pkg/logger"
	"github.com/flare-foundation/tee-node/pkg/wallets"
	"github.com/pkg/errors"
)

var DefaultExtensionId = big.NewInt(0)

func SetupExtension(
	s *support.Support,
	governanceHash common.Hash,
	instructionsSenderAddress, stateVerifierAddress common.Address,
) (*big.Int, error) {
	callOpts := &bind.CallOpts{
		From:    crypto.PubkeyToAddress(s.Prv.PublicKey),
		Context: context.Background(),
	}
	deployerAddr := crypto.PubkeyToAddress(s.Prv.PublicKey)

	opts, err := bind.NewKeyedTransactorWithChainID(s.Prv, s.ChainID)
	if err != nil {
		return nil, err
	}

	// Step 1: Register the extension
	extRegistered, _, err := registerExtension(s, opts, instructionsSenderAddress, stateVerifierAddress)
	if err != nil {
		return nil, err
	}
	extensionID := extRegistered.ExtensionId
	logger.Infof("Extension registered with ID: %s", extensionID.String())

	// Step 2: Allow TEE machine owners for this extension
	alreadyMachineOwner, err := s.TeeOwnerAllowlist.IsAllowedTeeMachineOwner(callOpts, extensionID, deployerAddr)
	if err != nil {
		return nil, errors.Errorf("failed checking TEE machine owner status: %s", err)
	}
	if alreadyMachineOwner {
		logger.Infof("Deployer already allowed as TEE machine owner for extension %s, skipping", extensionID.String())
	} else {
		_, err = allowTeeMachineOwners(s, opts, extensionID, []common.Address{deployerAddr})
		if err != nil {
			return nil, errors.Errorf("failed adding TEE machine owners (extension exists as ID %s but owners not set): %s", extensionID.String(), err)
		}
		logger.Infof("TEE machine owners allowed for extension %s", extensionID.String())
	}

	// Step 3: Allow wallet project owners for this extension
	alreadyProjectOwner, err := s.TeeOwnerAllowlist.IsAllowedTeeWalletProjectOwner(callOpts, extensionID, deployerAddr)
	if err != nil {
		return nil, errors.Errorf("failed checking wallet project owner status: %s", err)
	}
	if alreadyProjectOwner {
		logger.Infof("Deployer already allowed as wallet project owner for extension %s, skipping", extensionID.String())
	} else {
		_, err = allowTeeProjectManagerOwners(s, opts, extensionID, []common.Address{deployerAddr})
		if err != nil {
			return nil, errors.Errorf("failed adding wallet project owners (extension exists as ID %s but owners not set): %s", extensionID.String(), err)
		}
		logger.Infof("Wallet project owners allowed for extension %s", extensionID.String())
	}

	// Step 4: Allow an EVM type of keys on the extension
	isKeyTypeSupported, err := IsKeyTypeSupported(s, extensionID, wallets.EVMType)
	if err != nil {
		return nil, err
	}
	if isKeyTypeSupported {
		logger.Infof("Key type %s already supported for extension %s, skipping", wallets.EVMType, extensionID.String())
	} else {
		logger.Infof("Adding key type %s to extension %s", wallets.EVMType, extensionID)
		err = AddSupportedKeyTypes(s, extensionID, []common.Hash{wallets.EVMType})
		if err != nil {
			return nil, err
		}
	}

	return extensionID, nil
}


func AddSupportedKeyTypes(s *support.Support, extensionId *big.Int, keyTypes []common.Hash) error {
	opts, err := bind.NewKeyedTransactorWithChainID(s.Prv, s.ChainID)
	if err != nil {
		return errors.Errorf("%s", err)
	}

	keyTypesBytes32 := HashArrayToBytes32Array(keyTypes)

	err = addSupportedKeyTypesTx(s, opts, extensionId, keyTypesBytes32)
	if err != nil {
		return errors.Errorf("%s", err)
	}

	return nil
}

func IsKeyTypeSupported(s *support.Support, extensionId *big.Int, keyType common.Hash) (bool, error) {
	callOpts := &bind.CallOpts{
		From:    crypto.PubkeyToAddress(s.Prv.PublicKey),
		Context: context.Background(),
	}

	isSupported, err := isKeyTypeSupportedCall(s, callOpts, extensionId, keyType)
	if err != nil {
		return false, errors.Errorf("%s", err)
	}

	return isSupported, nil
}

func registerExtension(
	s *support.Support, opts *bind.TransactOpts, instructionsSenderAddress, stateVerifierAddress common.Address,
) (
	*teeextensionregistry.TeeExtensionRegistryTeeExtensionRegistered, *teeextensionregistry.TeeExtensionRegistryTeeExtensionContractsSet, error,
) {
	tx, err := s.TeeExtensionRegistry.Register(opts, stateVerifierAddress, instructionsSenderAddress)
	if err != nil {
		return nil, nil, errors.Errorf("TeeExtensionRegistry.Register failed: %s", err)
	}

	receipt, err := support.CheckTx(tx, s.ChainClient)
	if err != nil {
		return nil, nil, errors.Errorf("Register transaction failed: %s", err)
	}

	if len(receipt.Logs) < 2 {
		return nil, nil, errors.Errorf(
			"expected at least 2 logs from Register() transaction, got %d — "+
				"the registry contract may have changed or be behind a proxy",
			len(receipt.Logs),
		)
	}

	extensionRegistered, err := s.TeeExtensionRegistry.ParseTeeExtensionRegistered(*receipt.Logs[0])
	if err != nil {
		return nil, nil, errors.Errorf("failed to parse TeeExtensionRegistered event: %s", err)
	}

	if extensionRegistered.ExtensionId == nil || extensionRegistered.ExtensionId.Sign() == 0 {
		logger.Warnf("WARNING: extension ID is 0 — this may cause issues with setExtensionId() sentinel logic")
	}

	extensionContractsSet, err := s.TeeExtensionRegistry.ParseTeeExtensionContractsSet(*receipt.Logs[1])
	if err != nil {
		return nil, nil, errors.Errorf("failed to parse TeeExtensionContractsSet event: %s", err)
	}

	return extensionRegistered, extensionContractsSet, nil
}

func allowTeeMachineOwners(s *support.Support, opts *bind.TransactOpts, extensionId *big.Int, owners []common.Address) (*teeownerallowlist.TeeOwnerAllowlistAllowedTeeMachineOwnersAdded, error) {
	tx, err := s.TeeOwnerAllowlist.AddAllowedTeeMachineOwners(opts, extensionId, owners)
	if err != nil {
		return nil, errors.Errorf("AddAllowedTeeMachineOwners failed: %s", err)
	}

	receipt, err := support.CheckTx(tx, s.ChainClient)
	if err != nil {
		return nil, errors.Errorf("AddAllowedTeeMachineOwners transaction failed: %s", err)
	}

	if len(receipt.Logs) == 0 {
		return nil, errors.New("no logs in AddAllowedTeeMachineOwners transaction — unexpected")
	}

	ownersAdded, err := s.TeeOwnerAllowlist.ParseAllowedTeeMachineOwnersAdded(*receipt.Logs[0])
	if err != nil {
		return nil, errors.Errorf("failed to parse AllowedTeeMachineOwnersAdded event: %s", err)
	}

	return ownersAdded, nil
}

func allowTeeProjectManagerOwners(s *support.Support, opts *bind.TransactOpts, extensionId *big.Int, owners []common.Address) (*teeownerallowlist.TeeOwnerAllowlistAllowedTeeWalletProjectOwnersAdded, error) {
	tx, err := s.TeeOwnerAllowlist.AddAllowedTeeWalletProjectOwners(opts, extensionId, owners)
	if err != nil {
		return nil, errors.Errorf("AddAllowedTeeWalletProjectOwners failed: %s", err)
	}

	receipt, err := support.CheckTx(tx, s.ChainClient)
	if err != nil {
		return nil, errors.Errorf("AddAllowedTeeWalletProjectOwners transaction failed: %s", err)
	}

	if len(receipt.Logs) == 0 {
		return nil, errors.New("no logs in AddAllowedTeeWalletProjectOwners transaction — unexpected")
	}

	ownersAdded, err := s.TeeOwnerAllowlist.ParseAllowedTeeWalletProjectOwnersAdded(*receipt.Logs[0])
	if err != nil {
		return nil, errors.Errorf("failed to parse AllowedTeeWalletProjectOwnersAdded event: %s", err)
	}

	return ownersAdded, nil
}

func addSupportedKeyTypesTx(s *support.Support, opts *bind.TransactOpts, extensionId *big.Int, keyTypesBytes32 [][32]byte) error {
	tx, err := s.TeeExtensionRegistry.AddSupportedKeyTypes(opts, extensionId, keyTypesBytes32)
	if err != nil {
		return errors.Errorf("%s", err)
	}

	_, err = support.CheckTx(tx, s.ChainClient)
	if err != nil {
		return errors.Errorf("%s", err)
	}
	return nil
}

func isKeyTypeSupportedCall(s *support.Support, opts *bind.CallOpts, extensionId *big.Int, keyType common.Hash) (bool, error) {
	return s.TeeExtensionRegistry.IsKeyTypeSupported(opts, extensionId, keyType)
}
