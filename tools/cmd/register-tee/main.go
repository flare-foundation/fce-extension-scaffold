package main

import (
	"encoding/hex"
	"extension-scaffold/tools/pkg/configs"
	"extension-scaffold/tools/pkg/fccutils"
	"extension-scaffold/tools/pkg/support"
	"flag"
	"os"

	"github.com/flare-foundation/go-flare-common/pkg/logger"
)

func main() {
	af := flag.String("a", configs.AddressesFile, "file with deployed addresses")
	cf := flag.String("c", configs.ChainNodeURL, "chain node url")
	pf := flag.String("p", configs.ExtensionProxyURL, "extension proxy url (used to query TEE info)")
	hf := flag.String("h", "", "host url to register on-chain (defaults to -p if not set)")
	epf := flag.String("ep", "http://localhost:6662", "external proxy url (for FTDC)")
	instructionF := flag.String("i", "", "instructionID")
	command := flag.String("command", "rap", "command (rap)")
	stateFile := flag.String("state", "../config/register-tee.state", "state file for resume support")
	resume := flag.Bool("resume", false, "resume from state file (default: start fresh)")

	flag.Parse()

	// Default: start fresh. Only resume if --resume is explicitly passed.
	if !*resume {
		if err := os.Remove(*stateFile); err != nil && !os.IsNotExist(err) {
			logger.Warnf("WARNING: failed to remove stale state file: %v", err)
		}
	}

	testSupport, err := support.DefaultSupport(*af, *cf)
	if err != nil {
		fccutils.FatalWithCause(err)
	}

	// get teeID from proxy
	teeInfo, err := fccutils.TeeInfo(*pf)
	if err != nil {
		fccutils.FatalWithCause(err)
	}

	teeID, _, err := fccutils.TeeProxyId(teeInfo)
	if err != nil {
		fccutils.FatalWithCause(err)
	}

	ftdcTeeID, _, err := fccutils.GetTeeProxyID(*epf)
	if err != nil {
		fccutils.FatalWithCause(err)
	}

	// to check if things are ok
	_, _, err = fccutils.GetCodeHashAndPlatform(teeInfo)
	if err != nil {
		fccutils.FatalWithCause(err)
	}

	hostURL := *hf
	if hostURL == "" {
		hostURL = *pf
	}

	logger.Infof("Registration of TEE with ID %s", hex.EncodeToString(teeID[:]))
	err = fccutils.RegisterNode(testSupport, teeInfo, hostURL, *epf, ftdcTeeID, *command, *instructionF, *stateFile)
	if err != nil {
		fccutils.FatalWithCause(err)
	}

	logger.Infof("Registered TEE node with id %s", teeID)
}
