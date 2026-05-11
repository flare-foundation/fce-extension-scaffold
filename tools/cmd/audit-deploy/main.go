// audit-deploy: sanity-check deployed-addresses.json against a chain.
//   - Code present at each address
//   - For FlareTeeManager (diamond proxy): list system-supported platforms via the
//     ExtensionManager facet, and for a given extensionId print the owner +
//     supported codeHashes.
//   - Replays the most recent failing call (addTeeVersion) as eth_call to decode
//     custom errors when -addTeeVersion is provided.
package main

import (
	"bytes"
	"context"
	stderrors "errors"
	"flag"
	"fmt"
	"math/big"
	"os"
	"strings"

	"extension-scaffold/tools/pkg/support"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/tee/extensionmanager"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/tee/machinemanager"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/tee/verification"
)

func main() {
	af := flag.String("a", "./config/coston/deployed-addresses.json", "addresses file")
	rpcURL := flag.String("rpc", "https://coston-api.flare.network/ext/C/rpc", "rpc url")
	extID := flag.Int64("ext", -1, "extension id to inspect (e.g. 10)")
	codeHash := flag.String("codeHash", "", "0x... code hash for addTeeVersion replay")
	platform := flag.String("platform", "", "0x... platform for addTeeVersion replay")
	from := flag.String("from", "0xDa50A19aF65655785ab1F4c6a5b0592AFC497C88", "from address for eth_call replay")
	version := flag.String("version", "v0.1.0", "version string for addTeeVersion replay")
	flag.Parse()

	addr, err := support.ParseAddresses(*af)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse addresses: %v\n", err)
		os.Exit(1)
	}

	cc, err := ethclient.Dial(*rpcURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dial: %v\n", err)
		os.Exit(1)
	}
	ctx := context.Background()

	chainID, err := cc.ChainID(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "chainId: %v\n", err)
		os.Exit(1)
	}
	bn, _ := cc.BlockNumber(ctx)
	fmt.Printf("Chain ID: %s   block: %d\n", chainID.String(), bn)

	for _, e := range []struct {
		name string
		a    common.Address
	}{
		{"FlareTeeManager", addr.FlareTeeManager},
		{"FlareSystemsManager", addr.FlareSystemManager},
		{"Fdc2Hub", addr.Fdc2Hub},
	} {
		code, err := cc.CodeAt(ctx, e.a, nil)
		var status string
		switch {
		case err != nil:
			status = "ERROR " + err.Error()
		case e.a == (common.Address{}):
			status = "ZERO ADDRESS"
		case len(code) == 0:
			status = "NO CODE"
		default:
			status = fmt.Sprintf("ok (%d bytes)", len(code))
		}
		fmt.Printf("  %-25s %s   %s\n", e.name, e.a.Hex(), status)
	}

	if addr.FlareTeeManager == (common.Address{}) {
		return
	}
	er, err := extensionmanager.NewExtensionManager(addr.FlareTeeManager, cc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "bind ExtensionManager: %v\n", err)
		return
	}
	opts := &bind.CallOpts{Context: ctx}

	plats, perr := er.GetSystemSupportedPlatforms(opts)
	fmt.Printf("\n=== ExtensionManager facet @ FlareTeeManager %s ===\n", addr.FlareTeeManager.Hex())
	if perr != nil {
		fmt.Printf("  getSystemSupportedPlatforms ERROR: %v\n", perr)
	} else {
		fmt.Printf("  systemSupportedPlatforms (%d):\n", len(plats))
		for _, p := range plats {
			fmt.Printf("    %s   %q\n", common.Hash(p).Hex(), bytes32ToString(p))
		}
	}
	cnt, cerr := er.ExtensionsCounter(opts)
	if cerr == nil {
		fmt.Printf("  extensionsCounter: %s\n", cnt.String())
	}

	if *extID >= 0 {
		eid := big.NewInt(*extID)
		fmt.Printf("\n=== Extension id=%s ===\n", eid.String())
		owner, err := er.GetExtensionOwner(opts, eid)
		if err != nil {
			fmt.Printf("  getExtensionOwner ERROR: %v\n", err)
		} else {
			fmt.Printf("  owner: %s\n", owner.Hex())
		}
		ch, err := er.GetSupportedCodeHashes(opts, eid)
		if err != nil {
			fmt.Printf("  getSupportedCodeHashes ERROR: %v\n", err)
		} else {
			fmt.Printf("  supportedCodeHashes (%d):\n", len(ch))
			for _, h := range ch {
				fmt.Printf("    %s\n", common.Hash(h).Hex())
			}
		}
		kt, err := er.GetSupportedKeyTypes(opts, eid)
		if err == nil {
			fmt.Printf("  supportedKeyTypes (%d):\n", len(kt))
			for _, k := range kt {
				fmt.Printf("    %s   %q\n", common.Hash(k).Hex(), bytes32ToString(k))
			}
		}
	}

	if *codeHash != "" && *platform != "" && *extID >= 0 {
		fmt.Printf("\n=== addTeeVersion eth_call replay (decoding revert) ===\n")
		replayAddTeeVersion(cc, addr.FlareTeeManager, common.HexToAddress(*from), big.NewInt(*extID), *version, common.HexToHash(*codeHash), common.HexToHash(*platform))
	}
}

func replayAddTeeVersion(cc *ethclient.Client, to, from common.Address, extID *big.Int, version string, codeHash, platform common.Hash) {
	emABI, err := extensionmanager.ExtensionManagerMetaData.GetAbi()
	if err != nil {
		fmt.Printf("  load ABI: %v\n", err)
		return
	}
	calldata, err := emABI.Pack("addTeeVersion", extID, version, codeHash, [][32]byte{platform}, common.Hash{})
	if err != nil {
		fmt.Printf("  pack: %v\n", err)
		return
	}
	msg := ethereum.CallMsg{From: from, To: &to, Data: calldata}
	_, callErr := cc.CallContract(context.Background(), msg, nil)
	if callErr == nil {
		fmt.Println("  eth_call succeeded — tx-time-only revert (gas/state race?)")
		return
	}
	revertData := extractRevertData(callErr)
	if len(revertData) < 4 {
		fmt.Printf("  no/short revert data: %v\n", callErr)
		return
	}
	selector := revertData[:4]
	if bytes.Equal(selector, []byte{0x08, 0xc3, 0x79, 0xa0}) {
		if reason, err := abi.UnpackRevert(revertData); err == nil {
			fmt.Printf("  revert reason (string): %q\n", reason)
			return
		}
	}
	for _, src := range []struct {
		name string
		md   *bind.MetaData
	}{
		{"ExtensionManager", extensionmanager.ExtensionManagerMetaData},
		{"MachineManager", machinemanager.MachineManagerMetaData},
		{"Verification", verification.VerificationMetaData},
	} {
		a, err := src.md.GetAbi()
		if err != nil {
			continue
		}
		for errName, eDef := range a.Errors {
			if !bytes.Equal(eDef.ID[:4], selector) {
				continue
			}
			args, unpackErr := eDef.Inputs.Unpack(revertData[4:])
			if unpackErr != nil {
				fmt.Printf("  revert -> %s.%s (unpack failed: %v) data=0x%x\n", src.name, errName, unpackErr, revertData)
			} else {
				fmt.Printf("  revert -> %s.%s%v\n", src.name, errName, args)
			}
			return
		}
	}
	fmt.Printf("  revert with unknown selector 0x%x (full: 0x%x)\n", selector, revertData)
}

func extractRevertData(err error) []byte {
	if err == nil {
		return nil
	}
	var dataErr rpc.DataError
	if stderrors.As(err, &dataErr) {
		switch d := dataErr.ErrorData().(type) {
		case string:
			return common.FromHex(d)
		case []byte:
			return d
		}
	}
	return nil
}

func bytes32ToString(b [32]byte) string {
	end := len(b)
	for end > 0 && b[end-1] == 0 {
		end--
	}
	return strings.TrimSpace(string(b[:end]))
}
