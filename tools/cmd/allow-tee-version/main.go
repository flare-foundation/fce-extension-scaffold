package main

import (
	"crypto/ecdsa"
	"flag"
	"os"
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
	pf := flag.String("p", configs.ExtensionProxyURL, "proxy url")
	versionF := flag.String("version", "v0.1.0", "version")
	flag.Parse()

	testSupport, err := support.DefaultSupport(*af, *cf)
	if err != nil {
		fccutils.FatalWithCause(err)
	}

	// get teeID from proxy
	teeInfo, err := fccutils.TeeInfo(*pf)
	if err != nil {
		fccutils.FatalWithCause(err)
	}

	var privKey *ecdsa.PrivateKey
	privKeyString := os.Getenv("EXTENSION_OWNER_KEY")
	if privKeyString != "" {
		if strings.HasPrefix(privKeyString, "0x") || strings.HasPrefix(privKeyString, "0X") {
			privKeyString = privKeyString[2:]
		}
		privKey, err = crypto.HexToECDSA(privKeyString)
		if err != nil {
			fccutils.FatalWithCause(err)
		}
	} else {
		privKey = testSupport.Prv
	}

	keySource := "EXTENSION_OWNER_KEY"
	if privKeyString == "" {
		keySource = "DEPLOYMENT_PRIVATE_KEY (default)"
	}
	logger.Infof("Using key: %s (deployer: %s)", keySource, crypto.PubkeyToAddress(privKey.PublicKey).Hex())

	teeID, _, err := fccutils.TeeProxyId(teeInfo)
	if err != nil {
		fccutils.FatalWithCause(err)
	}

	logger.Infof("Code hash:    %s (source: proxy /info)", teeInfo.MachineData.CodeHash.Hex())
	logger.Infof("Platform:     %s (source: proxy /info)", teeInfo.MachineData.Platform.Hex())
	logger.Infof("Extension ID: %s", teeInfo.MachineData.ExtensionID.Big().String())
	logger.Infof("TEE ID:       %s", teeID.Hex())
	logger.Infof("Version:      %s", *versionF)
	logger.Warnf("NOTE: Code hash is from proxy /info response — not independently verified against attestation")
	err = fccutils.AddTeeVersion(testSupport, privKey, teeInfo.MachineData.ExtensionID.Big(), teeInfo.MachineData.CodeHash, teeInfo.MachineData.Platform, common.Hash{}, *versionF)
	if err != nil {
		if strings.Contains(err.Error(), "VersionAlreadyExists") {
			logger.Infof("version already registered, skipping")
		} else {
			fccutils.FatalWithCause(err)
		}
	}
}
