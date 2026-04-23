package fccutils

import (
	"os"

	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/tee-node/pkg/types"
	"github.com/pkg/errors"
)

func GetTeeProxyID(proxyURL string) (common.Address, common.Address, error) {
	teeInfo, err := TeeInfo(proxyURL)
	if err != nil {
		return common.Address{}, common.Address{}, err
	}

	teeID, proxyID, err := TeeProxyId(teeInfo)
	if err != nil {
		return common.Address{}, common.Address{}, err
	}

	return teeID, proxyID, nil
}

func GetCodeHashAndPlatform(teeInfo *types.SignedTeeInfoResponse) (common.Hash, common.Hash, error) {
	simulatedTee := os.Getenv("SIMULATED_TEE") == "true"
	codeHash := TeeCodeHash
	platform := TestPlatform
	var err error
	if !simulatedTee {
		codeHash, platform, err = CodeHashAndPlatform(string(teeInfo.TeeInfoResponse.Attestation))
		if err != nil {
			return common.Hash{}, common.Hash{}, err
		}
	}

	if codeHash != teeInfo.MachineData.CodeHash {
		return common.Hash{}, common.Hash{}, errors.Errorf("code hashes do not match: %s, %s", codeHash, teeInfo.MachineData.CodeHash)
	}
	if platform != teeInfo.MachineData.Platform {
		return common.Hash{}, common.Hash{}, errors.Errorf("platforms do not matc: %s, %s", platform, teeInfo.MachineData.Platform)
	}

	return codeHash, platform, nil
}

func HashArrayToBytes32Array(hashes []common.Hash) [][32]byte {
	bytes32Array := make([][32]byte, len(hashes))
	for i, hash := range hashes {
		bytes32Array[i] = [32]byte(hash)
	}
	return bytes32Array
}

func RequireNoError(err error) {
	if err != nil {
		FatalWithCause(err)
	}
}

func RequireTrue(condition bool, message string) {
	if !condition {
		FatalWithCause(errors.New(message))
	}
}
