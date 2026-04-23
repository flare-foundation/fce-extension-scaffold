// TEMPORARY: This command starts the extension TEE node and extension server
// as Go processes. It will be replaced by a Docker container once the Dockerfile
// is implemented. See EXTENSION-TEMPLATE-SPEC.md §5 for the Docker approach.
package main

import (
	"crypto/ecdsa"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"runtime"
	"syscall"
	"time"

	"extension-scaffold/tools/pkg/fccutils"
	echoserver "extension-scaffold/pkg/server"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/flare-foundation/go-flare-common/pkg/logger"
	teeServer "github.com/flare-foundation/tee-node/pkg/server"
	"github.com/joho/godotenv"
)

// Port constants matching the extension-e2e configs.
const (
	ExtConfigurationPort = 5501 // TEE configuration port (proxyURL, initialOwner, extensionID)
	ExtProxyInternalPort = 6663 // Internal port: TEE polls actions from proxy queue
	ExtensionServerPort  = 7701 // TEE signing port: extension calls TEE for signing/encrypting
	ExtensionPort        = 7702 // Extension server port: TEE forwards POST /action here
)

func main() {
	extensionID := flag.String("extensionID", "", "extension ID (bytes32 hex)")
	flag.Parse()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	loadEnv()

	_ = setOwnerAddress()

	if *extensionID != "" {
		os.Setenv("EXTENSION_ID", *extensionID)
	}

	runExtension()

	sig := <-signalChan
	logger.Infof("Received %v signal, shutting down", sig)
}

func loadEnv() {
	// Try project-root .env first (works even when CWD is tools/).
	_, thisFile, _, _ := runtime.Caller(0)
	rootEnv := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", ".env")
	if err := godotenv.Load(rootEnv); err != nil {
		// Fallback to CWD .env.
		if err := godotenv.Load(); err != nil {
			fmt.Printf("Warning: Error loading .env file: %v\n", err)
		}
	}
}

func runExtension() {
	// Start tee-node in extension mode.
	go teeServer.StartServerExtension(ExtConfigurationPort, ExtensionServerPort, ExtensionPort)

	// Start extension server — fail fast if port binding fails.
	extErrCh := echoserver.StartExtension(ExtensionPort, ExtensionServerPort)

	// Give server a moment to bind, then check for early failures.
	time.Sleep(100 * time.Millisecond)
	select {
	case err := <-extErrCh:
		logger.Fatalf("extension server failed to start: %v", err)
	default:
	}

	logger.Infof("Starting echo extension TEE on port %d", ExtConfigurationPort)

	time.Sleep(150 * time.Millisecond)

	err := fccutils.SetProxyUrl(ExtConfigurationPort, ExtProxyInternalPort)
	if err != nil {
		fccutils.FatalWithCause(err)
	}
}

func setOwnerAddress() common.Address {
	owner := os.Getenv("INITIAL_OWNER")
	if owner == "" {
		var privKey *ecdsa.PrivateKey
		var err error
		privKeyString := os.Getenv("DEPLOYMENT_PRIVATE_KEY")
		if privKeyString == "" {
			// Default Hardhat-funded key.
			privKey, err = crypto.HexToECDSA("804b01a8c27a65cc694a867be76edae3ccce7a7161cda1f67a8349df696d2207")
			if err != nil {
				panic("cannot parse default private key")
			}
		} else {
			if strings.HasPrefix(privKeyString, "0x") || strings.HasPrefix(privKeyString, "0X") {
				privKeyString = privKeyString[2:]
			}
			privKey, err = crypto.HexToECDSA(privKeyString)
			if err != nil {
				fccutils.FatalWithCause(err)
			}
		}

		ownerAddress := crypto.PubkeyToAddress(privKey.PublicKey)
		os.Setenv("INITIAL_OWNER", ownerAddress.String())
	}

	return common.HexToAddress(owner)
}
