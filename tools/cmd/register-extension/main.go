package main

import (
	"flag"
	"fmt"

	"extension-scaffold/tools/pkg/configs"
	"extension-scaffold/tools/pkg/fccutils"
	"extension-scaffold/tools/pkg/support"
	"extension-scaffold/tools/pkg/validate"

	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-flare-common/pkg/logger"
)

func main() {
	af := flag.String("a", configs.AddressesFile, "file with deployed addresses")
	cf := flag.String("c", configs.ChainNodeURL, "chain node url")
	instructionSenderF := flag.String("instructionSender", "", "InstructionSender contract address (required)")
	governanceHashF := flag.String("governanceHash", "", "governance hash (optional)")
	flag.Parse()

	if *instructionSenderF == "" {
		logger.Fatal("--instructionSender flag is required")
	}

	testSupport, err := support.DefaultSupport(*af, *cf)
	if err != nil {
		fccutils.FatalWithCause(err)
	}

	governanceHash := common.HexToHash(*governanceHashF)
	instructionSenderAddress := common.HexToAddress(*instructionSenderF)

	// Pre-flight: verify instruction sender has code on-chain
	if err := validate.AddressHasCode(testSupport.ChainClient, instructionSenderAddress, "InstructionSender"); err != nil {
		fccutils.FatalWithCause(err)
	}

	logger.Infof("Registering extension with InstructionSender %s...", instructionSenderAddress.Hex())
	extensionID, err := fccutils.SetupExtension(testSupport, governanceHash, instructionSenderAddress, common.Address{})
	if err != nil {
		fccutils.FatalWithCause(err)
	}

	if extensionID == nil || extensionID.Sign() <= 0 {
		logger.Warnf("WARNING: extension ID is %v — verify this is expected", extensionID)
	}

	extensionIDHex := fmt.Sprintf("0x%064x", extensionID)
	logger.Infof("Extension registered with ID: %s", extensionIDHex)

	// Machine-readable output on stdout
	fmt.Println(extensionIDHex)
}
