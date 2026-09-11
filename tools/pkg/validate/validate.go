package validate

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// MinDeployBalance is a floor only — prefer KeyCanAffordDeploy, which prices
// the deploy at the live gas price instead of a fixed amount.
var MinDeployBalance = big.NewInt(10_000_000_000_000_000)

// DeployGasUnits is a deploy's worst case: ~1.7M for the scaffold's
// InstructionSender, rounded up for constructor work and gas-price drift.
const DeployGasUnits uint64 = 2_500_000

// rpcTimeout is the timeout for all RPC calls.
const rpcTimeout = 10 * time.Second

// AddressNotZero checks that addr is not the zero address.
// Returns an error with the label identifying which address was zero.
func AddressNotZero(addr common.Address, label string) error {
	if addr == (common.Address{}) {
		return fmt.Errorf(
			"%s is the zero address (%s) in the addresses file",
			label, addr.Hex(),
		)
	}
	return nil
}

// AddressHasCode calls eth_getCode to verify that a contract is deployed at addr.
// Returns a descriptive error if the client is nil, the RPC call fails, or no code exists.
func AddressHasCode(client *ethclient.Client, addr common.Address, label string) error {
	if client == nil {
		return fmt.Errorf("cannot check %s: no chain client connected", label)
	}

	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()

	code, err := client.CodeAt(ctx, addr, nil)
	if err != nil {
		return fmt.Errorf("cannot check %s: %w", label, err)
	}

	if len(code) == 0 {
		return fmt.Errorf(
			"%s at %s has no deployed code. "+
				"This address will be set as immutable in the contract constructor and cannot be changed. "+
				"Check your deployed-addresses.json file — are you on the right network?",
			label, addr.Hex(),
		)
	}

	return nil
}

// KeyHasFunds checks that the account derived from key has at least minWei balance.
// Returns a descriptive error if the client is nil, the RPC call fails, or funds are insufficient.
func KeyHasFunds(client *ethclient.Client, key *ecdsa.PrivateKey, minWei *big.Int) error {
	if client == nil {
		return fmt.Errorf("cannot check deployer balance: no chain client connected")
	}

	addr := crypto.PubkeyToAddress(key.PublicKey)

	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()

	balance, err := client.BalanceAt(ctx, addr, nil)
	if err != nil {
		return fmt.Errorf("cannot check deployer balance: %w", err)
	}

	if balance.Cmp(minWei) < 0 {
		return fmt.Errorf(
			"deployer %s has insufficient funds (balance: %s wei, minimum required: %s wei). "+
				"Fund this account before deploying",
			addr.Hex(), balance.String(), minWei.String(),
		)
	}

	return nil
}

// KeyCanAffordDeploy prices the deploy at the live gas price. A fixed floor is
// not enough: at 650 gwei, 0.01 ETH passes and the deploy still dies with
// "contract creation code storage out of gas", because eth_estimateGas caps the
// gas it will try at balance/gasPrice.
func KeyCanAffordDeploy(client *ethclient.Client, key *ecdsa.PrivateKey, gasUnits uint64) error {
	if client == nil {
		return fmt.Errorf("cannot check deployer balance: no chain client connected")
	}
	addr := crypto.PubkeyToAddress(key.PublicKey)

	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()

	balance, err := client.BalanceAt(ctx, addr, nil)
	if err != nil {
		return fmt.Errorf("cannot check deployer balance: %w", err)
	}
	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		return fmt.Errorf("cannot read gas price: %w", err)
	}
	return canAfford(addr, balance, gasPrice, gasUnits)
}

// canAfford is the comparison on its own, so it is testable without a chain.
func canAfford(addr common.Address, balance, gasPrice *big.Int, gasUnits uint64) error {
	if gasPrice == nil || gasPrice.Sign() <= 0 {
		return fmt.Errorf("cannot price the deploy: gas price is %v", gasPrice)
	}
	need := new(big.Int).Mul(gasPrice, new(big.Int).SetUint64(gasUnits))
	if balance.Cmp(need) >= 0 {
		return nil
	}
	return fmt.Errorf(
		"deployer %s cannot afford the deploy: has %s, needs ~%s for %d gas at %s gwei. "+
			"Fund this account before deploying",
		addr.Hex(), inEther(balance), inEther(need), gasUnits, inGwei(gasPrice),
	)
}

func inEther(wei *big.Int) string {
	f := new(big.Float).Quo(new(big.Float).SetInt(wei), big.NewFloat(1e18))
	return f.Text('f', 4)
}

func inGwei(wei *big.Int) string {
	f := new(big.Float).Quo(new(big.Float).SetInt(wei), big.NewFloat(1e9))
	return f.Text('f', 0)
}

// IsUsingDevKey returns true if the DEPLOYMENT_PRIVATE_KEY environment variable is empty,
// indicating the deployer is falling back to the hardcoded dev key.
func IsUsingDevKey() bool {
	return os.Getenv("DEPLOYMENT_PRIVATE_KEY") == ""
}
