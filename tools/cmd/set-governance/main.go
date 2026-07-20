// set-governance: register the TEE governance signer set + threshold for this
// extension on-chain.
//
// Reads the signer set from GOVERNANCE_SIGNERS (comma-separated 0x addresses)
// and the threshold from GOVERNANCE_THRESHOLD. Both default to "the deployer
// alone, threshold 1" when unset, so a developer who configures nothing still
// gets a working setup. These MUST match the same env vars passed to the TEE
// node (it derives its governanceHash from them), or register-tee fails with
// InvalidGovernanceHash.
package main

import (
	"crypto/ecdsa"
	"flag"
	"os"
	"strconv"
	"strings"

	"extension-scaffold/tools/pkg/configs"
	"extension-scaffold/tools/pkg/fccutils"
	"extension-scaffold/tools/pkg/support"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/flare-foundation/go-flare-common/pkg/logger"
)

func main() {
	af := flag.String("a", configs.AddressesFile, "file with deployed addresses")
	cf := flag.String("c", configs.ChainNodeURL, "chain node url")
	pf := flag.String("p", configs.ExtensionProxyURL, "extension proxy url (used to query the extension id)")
	flag.Parse()

	testSupport, err := support.DefaultSupport(*af, *cf)
	if err != nil {
		fccutils.FatalWithCause(err)
	}

	// Extension owner key (falls back to the deployment key, like the other tools).
	var privKey *ecdsa.PrivateKey
	if privKeyString := os.Getenv("EXTENSION_OWNER_KEY"); privKeyString != "" {
		privKeyString = strings.TrimPrefix(privKeyString, "0x")
		privKeyString = strings.TrimPrefix(privKeyString, "0X")
		privKey, err = crypto.HexToECDSA(privKeyString)
		if err != nil {
			fccutils.FatalWithCause(err)
		}
	} else {
		privKey = testSupport.Prv
	}
	deployer := crypto.PubkeyToAddress(privKey.PublicKey)

	// Default governance signer: INITIAL_OWNER if set (this is what the node's
	// compose env defaults to), else the deployer. Keeping both sides on the
	// same default is what makes the node's governanceHash match the chain.
	defaultSigner := deployer
	if io := strings.TrimSpace(os.Getenv("INITIAL_OWNER")); io != "" {
		defaultSigner = common.HexToAddress(io)
	}

	// Resolve the extension id from the proxy /info.
	teeInfo, err := fccutils.TeeInfo(*pf)
	if err != nil {
		fccutils.FatalWithCause(err)
	}
	extensionID := teeInfo.MachineData.ExtensionID.Big()

	signers := parseSigners(os.Getenv("GOVERNANCE_SIGNERS"), defaultSigner)
	threshold := parseThreshold(os.Getenv("GOVERNANCE_THRESHOLD"))

	logger.Infof("Extension ID:        %s", extensionID.String())
	logger.Infof("Governance signers:  %v", signers)
	logger.Infof("Governance threshold: %d", threshold)

	if err := fccutils.SetTeeGovernance(testSupport, privKey, extensionID, signers, threshold); err != nil {
		fccutils.FatalWithCause(err)
	}
}

// parseSigners parses a comma-separated list of 0x addresses, falling back to
// the deployer as the sole signer when empty.
func parseSigners(raw string, fallback common.Address) []common.Address {
	var signers []common.Address
	for _, part := range strings.Split(raw, ",") {
		if part = strings.TrimSpace(part); part != "" {
			signers = append(signers, common.HexToAddress(part))
		}
	}
	if len(signers) == 0 {
		return []common.Address{fallback}
	}
	return signers
}

// parseThreshold parses GOVERNANCE_THRESHOLD, defaulting to 1 on empty/invalid.
func parseThreshold(raw string) uint64 {
	if v, err := strconv.ParseUint(strings.TrimSpace(raw), 10, 64); err == nil && v > 0 {
		return v
	}
	return 1
}
