package support

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	stderrors "errors"
	"extension-scaffold/tools/pkg/configs"
	"fmt"
	"math/big"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/fdc2/fdc2hub"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/system"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/tee/extensionmanager"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/tee/machinemanager"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/tee/ownerallowlist"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/tee/verification"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/tee/walletkeymanager"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/tee/walletmanager"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/tee/walletprojectmanager"
	"github.com/joho/godotenv"

	"github.com/pkg/errors"
)

type Support struct {
	Prv *ecdsa.PrivateKey // funded private key

	FlareSystemManager *system.FlareSystemsManager
	Fdc2Hub            *fdc2hub.Fdc2Hub

	// Diamond-pattern facet bindings. All are bound to the same FlareTeeManager
	// (diamond proxy) address; the diamond routes each call to the correct facet.
	TeeMachineRegistry      *machinemanager.MachineManager
	TeeWalletProjectManager *walletprojectmanager.WalletProjectManager
	TeeWalletManager        *walletmanager.WalletManager
	TeeWalletKeyManager     *walletkeymanager.WalletKeyManager
	TeeVerification         *verification.Verification
	TeeExtensionRegistry    *extensionmanager.ExtensionManager
	TeeOwnerAllowlist       *ownerallowlist.OwnerAllowlist

	Addresses *Addresses

	ChainClient *ethclient.Client
	ChainID     *big.Int
}

type Addresses struct {
	FlareSystemManager common.Address `json:"FlareSystemsManager"`
	Fdc2Hub            common.Address `json:"Fdc2Hub"`
	// FlareTeeManager is the diamond proxy that exposes every TEE facet
	// (ExtensionManager, MachineManager, Verification, WalletManager,
	// WalletKeyManager, WalletProjectManager, OwnerAllowlist) at a single address.
	FlareTeeManager common.Address `json:"FlareTeeManager"`
}

func DefaultSupport(AddressesFilePath, chainNodeURL string) (*Support, error) {
	addr := &Addresses{}
	err := configs.ReadAddresses(AddressesFilePath, addr)
	if err != nil {
		addr, err = ParseAddresses(AddressesFilePath)
		if err != nil {
			return nil, errors.Errorf("%s", err)
		}
	}

	cc, err := ethclient.Dial(chainNodeURL)
	if err != nil {
		return nil, errors.Errorf("%s", err)
	}

	privKey, err := DefaultPrivateKey()
	if err != nil {
		return nil, err
	}

	return NewSupport(privKey, cc, addr)
}

func DefaultPrivateKey() (*ecdsa.PrivateKey, error) {
	if err := godotenv.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Error loading .env file: %v\n", err)
	}
	privKeyString := os.Getenv("DEPLOYMENT_PRIVATE_KEY")

	if privKeyString == "" {
		fmt.Fprintln(os.Stderr, "WARNING: DEPLOYMENT_PRIVATE_KEY not set — using hardcoded Hardhat dev key")
		fmt.Fprintln(os.Stderr, "         This key only has funds on local devnets (Hardhat/Anvil)")
		return configs.PrvWithFunds, nil
	} else {
		if strings.HasPrefix(privKeyString, "0x") || strings.HasPrefix(privKeyString, "0X") {
			privKeyString = privKeyString[2:]
		}

		privKey, err := crypto.HexToECDSA(privKeyString)
		if err != nil {
			return nil, errors.Errorf("%s", err)
		}
		return privKey, nil
	}
}

func NewSupport(prv *ecdsa.PrivateKey, chainClient *ethclient.Client, addresses *Addresses) (*Support, error) {
	// Every TEE facet binding is wired to the FlareTeeManager diamond proxy.
	diamond := addresses.FlareTeeManager

	tr, err := machinemanager.NewMachineManager(diamond, chainClient)
	if err != nil {
		return nil, err
	}

	twm, err := walletmanager.NewWalletManager(diamond, chainClient)
	if err != nil {
		return nil, err
	}

	twkm, err := walletkeymanager.NewWalletKeyManager(diamond, chainClient)
	if err != nil {
		return nil, err
	}

	twpm, err := walletprojectmanager.NewWalletProjectManager(diamond, chainClient)
	if err != nil {
		return nil, err
	}

	sm, err := system.NewFlareSystemsManager(addresses.FlareSystemManager, chainClient)
	if err != nil {
		return nil, err
	}

	ftdc, err := fdc2hub.NewFdc2Hub(addresses.Fdc2Hub, chainClient)
	if err != nil {
		return nil, err
	}

	tv, err := verification.NewVerification(diamond, chainClient)
	if err != nil {
		return nil, err
	}

	ter, err := extensionmanager.NewExtensionManager(diamond, chainClient)
	if err != nil {
		return nil, err
	}

	toal, err := ownerallowlist.NewOwnerAllowlist(diamond, chainClient)
	if err != nil {
		return nil, err
	}

	chainID, err := chainClient.ChainID(context.Background())
	if err != nil {
		return nil, err
	}

	return &Support{
		Prv:                     prv,
		TeeMachineRegistry:      tr,
		TeeWalletManager:        twm,
		TeeWalletKeyManager:     twkm,
		TeeWalletProjectManager: twpm,
		FlareSystemManager:      sm,
		Fdc2Hub:                 ftdc,
		TeeVerification:         tv,
		TeeExtensionRegistry:    ter,
		ChainClient:             chainClient,
		ChainID:                 chainID,
		TeeOwnerAllowlist:       toal,
		Addresses:               addresses,
	}, nil
}

func CheckTx(tx *types.Transaction, client *ethclient.Client) (*types.Receipt, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	receipt, err := bind.WaitMined(ctx, client, tx)
	if err != nil {
		return nil, errors.Errorf("transaction not mined within 2 minutes (tx: %s): %s", tx.Hash().Hex(), err)
	}
	if receipt.Status == 0 {
		reason, err := getFailingMessage(client, tx.Hash())
		if err != nil {
			return nil, err
		}
		return nil, errors.Errorf("error: Transaction fail: %s", reason)
	}

	return receipt, nil
}

func getFailingMessage(client *ethclient.Client, hash common.Hash) (string, error) {
	tx, _, err := client.TransactionByHash(context.Background(), hash)
	if err != nil {
		return "", errors.Errorf("%s", err)
	}

	from, err := types.Sender(types.NewEIP155Signer(tx.ChainId()), tx)
	if err != nil {
		return "", errors.Errorf("%s", err)
	}

	msg := ethereum.CallMsg{
		From:     from,
		To:       tx.To(),
		Gas:      tx.Gas(),
		GasPrice: tx.GasPrice(),
		Value:    tx.Value(),
		Data:     tx.Data(),
	}

	res, err := client.CallContract(context.Background(), msg, nil)
	if err != nil {
		// Try to decode revert reason from the error itself
		if reason := decodeRevertFromError(err); reason != "" {
			return reason, nil
		}
		return err.Error(), nil
	}

	// Decode ABI-encoded revert reason from call result
	if reason, unpackErr := abi.UnpackRevert(res); unpackErr == nil {
		return reason, nil
	}

	// Fallback: return hex-encoded bytes instead of raw binary garbage
	if len(res) > 0 {
		return fmt.Sprintf("0x%x", res), nil
	}

	return "unknown revert reason", nil
}

// decodeRevertFromError extracts a revert reason from an RPC error's data field.
func decodeRevertFromError(err error) string {
	type dataError interface {
		ErrorData() interface{}
	}
	var de dataError
	if stderrors.As(err, &de) {
		if data := de.ErrorData(); data != nil {
			if hexStr, ok := data.(string); ok {
				hexStr = strings.TrimPrefix(hexStr, "0x")
				decoded, decErr := hex.DecodeString(hexStr)
				if decErr == nil {
					if reason, unpackErr := abi.UnpackRevert(decoded); unpackErr == nil {
						return reason
					}
				}
			}
		}
	}
	return ""
}

// RawContract mirrors the JSON entries
type RawContract struct {
	Name         string `json:"name"`
	ContractName string `json:"contractName"`
	Address      string `json:"address"`
}

func ParseAddresses(filePath string) (*Addresses, error) {
	file, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var raw []RawContract
	err = json.Unmarshal(file, &raw)
	if err != nil {
		return nil, err
	}

	var addrs Addresses
	targets := fieldMap(&addrs)

	for _, c := range raw {
		if dest, ok := targets[c.Name]; ok {
			*dest = common.HexToAddress(c.Address)
		}
	}

	return &addrs, err
}

// fieldMap builds: "<json tag or field name>" -> pointer to field
func fieldMap(addrStruct *Addresses) map[string]*common.Address {
	out := make(map[string]*common.Address)

	v := reflect.ValueOf(addrStruct).Elem()
	t := v.Type()
	addrType := reflect.TypeOf(common.Address{})

	for i := 0; i < v.NumField(); i++ {
		fv := v.Field(i)
		ft := t.Field(i)

		if fv.Type() != addrType {
			continue
		}

		tag := ft.Tag.Get("json")
		key := tag
		if key == "" || key == "-" {
			key = ft.Name
		}

		out[key] = fv.Addr().Interface().(*common.Address)
	}

	return out
}
