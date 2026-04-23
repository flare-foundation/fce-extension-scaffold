package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"extension-scaffold/tools/pkg/configs"
	"extension-scaffold/tools/pkg/fccutils"
	"extension-scaffold/tools/pkg/support"
	instrutils "extension-scaffold/tools/pkg/utils"
	"extension-scaffold/tools/pkg/validate"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/flare-foundation/go-flare-common/pkg/logger"
)

func main() {
	af := flag.String("a", configs.AddressesFile, "file with deployed addresses")
	cf := flag.String("c", configs.ChainNodeURL, "chain node url")
	outFile := flag.String("o", "", "write deployed address to this file (optional)")
	preflightOnly := flag.Bool("preflight-only", false, "run validation checks and exit without deploying")
	flag.Parse()

	testSupport, err := support.DefaultSupport(*af, *cf)
	if err != nil {
		fccutils.FatalWithCause(err)
	}

	// --- Pre-flight validation ---
	deployer := crypto.PubkeyToAddress(testSupport.Prv.PublicKey)
	logger.Infof("Deployer:             %s", deployer.Hex())
	logger.Infof("Chain ID:             %s", testSupport.ChainID.String())
	logger.Infof("TeeExtensionRegistry: %s", testSupport.Addresses.TeeExtensionRegistry.Hex())
	logger.Infof("TeeMachineRegistry:   %s", testSupport.Addresses.TeeMachineRegistry.Hex())

	if err := validate.AddressNotZero(testSupport.Addresses.TeeExtensionRegistry, "TeeExtensionRegistry"); err != nil {
		fccutils.FatalWithCause(err)
	}
	if err := validate.AddressNotZero(testSupport.Addresses.TeeMachineRegistry, "TeeMachineRegistry"); err != nil {
		fccutils.FatalWithCause(err)
	}
	if err := validate.AddressHasCode(testSupport.ChainClient, testSupport.Addresses.TeeExtensionRegistry, "TeeExtensionRegistry"); err != nil {
		fccutils.FatalWithCause(err)
	}
	if err := validate.AddressHasCode(testSupport.ChainClient, testSupport.Addresses.TeeMachineRegistry, "TeeMachineRegistry"); err != nil {
		fccutils.FatalWithCause(err)
	}
	if err := validate.KeyHasFunds(testSupport.ChainClient, testSupport.Prv, validate.MinDeployBalance); err != nil {
		fccutils.FatalWithCause(err)
	}

	if *preflightOnly {
		logger.Infof("Pre-flight checks passed. Exiting without deploying.")
		return
	}

	logger.Infof("Deploying InstructionSender contract...")
	address, _, err := instrutils.DeployInstructionSender(testSupport)
	if err != nil {
		fccutils.FatalWithCause(err)
	}

	logger.Infof("InstructionSender deployed at: %s", address.Hex())

	// Optionally write address to file for script consumption
	if *outFile != "" {
		os.MkdirAll(filepath.Dir(*outFile), 0755)
		os.WriteFile(*outFile, []byte(address.Hex()), 0644)
	}

	// Machine-readable output on stdout (for scripts)
	fmt.Println(address.Hex())
}
