//go:build integration

// Package integration contains on-chain integration tests for the deployment
// hardening work. These tests require a running Ethereum node (Hardhat, Anvil,
// or a public testnet) and deployed registry contracts.
//
// Run with:
//
//	cd tools && go test -tags integration ./integration/ -v -count=1
//
// Against Coston2:
//
//	cd tools && CHAIN_URL=https://coston2-api.flare.network/ext/C/rpc \
//	  DEPLOYMENT_PRIVATE_KEY=<funded-key> \
//	  go test -tags integration ./integration/ -v -count=1
//
// Environment variables:
//
//	CHAIN_URL       — RPC endpoint (default: http://127.0.0.1:8545)
//	ADDRESSES_FILE  — path to deployed-addresses.json (default: ../../config/coston2/deployed-addresses.json)
//	DEPLOYMENT_PRIVATE_KEY        — funded private key hex (default: Hardhat dev key)
package integration

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"testing"
	"time"

	"extension-scaffold/tools/pkg/contracts/helloworld"
	"extension-scaffold/tools/pkg/fccutils"
	"extension-scaffold/tools/pkg/support"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// testSupport is the shared chain connection and deployer key.
// Initialized once in TestMain.
var testSupport *support.Support

// rpcDelay is a brief pause between chain operations to avoid rate limiting
// on public RPC endpoints (e.g. Coston2). Set to 0 for local nodes.
var rpcDelay = 2 * time.Second

func TestMain(m *testing.M) {
	chainURL := os.Getenv("CHAIN_URL")
	if chainURL == "" {
		chainURL = "http://127.0.0.1:8545"
	}

	addressesFile := os.Getenv("ADDRESSES_FILE")
	if addressesFile == "" {
		// Go tests run from the package directory (tools/integration/),
		// so go up two levels to reach the project root.
		addressesFile = "../../config/coston2/deployed-addresses.json"
	}

	var err error
	testSupport, err = support.DefaultSupport(addressesFile, chainURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Integration test setup failed: %v\n", err)
		fmt.Fprintf(os.Stderr, "  CHAIN_URL=%s\n", chainURL)
		fmt.Fprintf(os.Stderr, "  ADDRESSES_FILE=%s\n", addressesFile)
		fmt.Fprintf(os.Stderr, "\nMake sure a chain node is running and the addresses file exists.\n")
		os.Exit(1)
	}

	// Quick connectivity check
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	chainID, err := testSupport.ChainClient.ChainID(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot reach chain at %s: %v\n", chainURL, err)
		os.Exit(1)
	}
	deployer := crypto.PubkeyToAddress(testSupport.Prv.PublicKey)
	fmt.Printf("Integration tests connected: chain ID=%s, deployer=%s\n", chainID.String(), deployer.Hex())

	// Local nodes don't need rate-limit delays
	if chainURL == "http://127.0.0.1:8545" || chainURL == "http://localhost:8545" {
		rpcDelay = 0
	}

	os.Exit(m.Run())
}

// deployFreshInstructionSender deploys a new InstructionSender contract using
// the registry addresses from testSupport. Returns the deployed address and
// bound contract instance.
func deployFreshInstructionSender(t *testing.T) (common.Address, *helloworld.HelloWorldInstructionSender) {
	t.Helper()
	time.Sleep(rpcDelay)

	opts, err := bind.NewKeyedTransactorWithChainID(testSupport.Prv, testSupport.ChainID)
	if err != nil {
		t.Fatalf("failed to create transactor: %v", err)
	}

	address, tx, contract, err := helloworld.DeployHelloWorldInstructionSender(
		opts, testSupport.ChainClient,
		testSupport.Addresses.TeeExtensionRegistry,
		testSupport.Addresses.TeeMachineRegistry,
	)
	if err != nil {
		t.Fatalf("failed to deploy InstructionSender: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	receipt, err := bind.WaitMined(ctx, testSupport.ChainClient, tx)
	if err != nil {
		t.Fatalf("deployment tx not mined: %v", err)
	}
	if receipt.Status != 1 {
		t.Fatalf("deployment tx failed with status %d", receipt.Status)
	}

	t.Logf("Deployed InstructionSender at %s (tx: %s)", address.Hex(), tx.Hash().Hex())
	return address, contract
}

// deployInstructionSenderRaw attempts to deploy an InstructionSender with
// arbitrary registry addresses. Returns the error directly without fataling,
// so the caller can assert on the error.
func deployInstructionSenderRaw(t *testing.T, registryAddr, machineRegistryAddr common.Address) error {
	t.Helper()
	time.Sleep(rpcDelay)

	opts, err := bind.NewKeyedTransactorWithChainID(testSupport.Prv, testSupport.ChainID)
	if err != nil {
		t.Fatalf("failed to create transactor: %v", err)
	}

	_, _, _, err = helloworld.DeployHelloWorldInstructionSender(
		opts, testSupport.ChainClient,
		registryAddr, machineRegistryAddr,
	)
	return err
}

// registerExtensionForSender registers an extension for the given instruction
// sender address and returns the extension ID.
func registerExtensionForSender(t *testing.T, senderAddr common.Address) *big.Int {
	t.Helper()
	time.Sleep(rpcDelay)

	governanceHash := common.Hash{}
	extID, err := fccutils.SetupExtension(testSupport, governanceHash, senderAddr, common.Address{})
	if err != nil {
		t.Fatalf("SetupExtension failed: %v", err)
	}
	t.Logf("Extension registered with ID: %s", extID.String())
	return extID
}
