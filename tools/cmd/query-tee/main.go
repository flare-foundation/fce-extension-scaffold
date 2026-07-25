package main

import (
	"context"
	"flag"
	"fmt"
	"math/big"
	"os"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/tee/machinemanager"

	"extension-scaffold/tools/pkg/configs"
	"extension-scaffold/tools/pkg/support"
)

func main() {
	rpc := flag.String("rpc", "https://coston2-api.flare.network/ext/C/rpc", "rpc url")
	af := flag.String("a", configs.AddressesFile, "file with deployed addresses")
	reg := flag.String("reg", "", "TeeMachineRegistry address (overrides -a; the registry is the FlareTeeManager diamond)")
	listExt := flag.Int64("ext", -1, "list active TEEs in extension id (e.g. 0 for FTDC, 1588 for user)")
	flag.Parse()

	// Resolve the registry from the deployed-addresses file like every other tool,
	// so a redeployed diamond is picked up automatically. A retired deployment still
	// answers these calls with stale-but-plausible data, so a hardcoded default here
	// is actively misleading; -reg stays available as an explicit override.
	if *reg == "" {
		// Same two-form handling as support.DefaultSupport: name-keyed object first,
		// then the array-of-contracts layout used by the per-chain address files.
		addr := &support.Addresses{}
		if err := configs.ReadAddresses(*af, addr); err != nil {
			parsed, perr := support.ParseAddresses(*af)
			if perr != nil {
				fmt.Fprintf(os.Stderr, "read addresses %q: %v\n(pass -a <deployed-addresses.json> or -reg <address>)\n", *af, perr)
				os.Exit(1)
			}
			addr = parsed
		}
		if addr.FlareTeeManager == (common.Address{}) {
			fmt.Fprintf(os.Stderr, "no FlareTeeManager address in %q; pass -reg <address>\n", *af)
			os.Exit(1)
		}
		*reg = addr.FlareTeeManager.Hex()
	}
	fmt.Printf("registry: %s\n", *reg)

	cc, err := ethclient.Dial(*rpc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dial: %v\n", err)
		os.Exit(1)
	}
	mm, err := machinemanager.NewMachineManager(common.HexToAddress(*reg), cc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "bind: %v\n", err)
		os.Exit(1)
	}

	opts := &bind.CallOpts{Context: context.Background()}

	if *listExt >= 0 {
		ext := big.NewInt(*listExt)
		fmt.Printf("\n=== Active TEEs for extensionId=%s ===\n", ext.String())
		out, err := mm.GetActiveTeeMachines(opts, ext)
		if err != nil {
			fmt.Printf("getActiveTeeMachines ERROR: %v\n", err)
		} else {
			for i, id := range out.TeeIds {
				fmt.Printf("  %d: %s url=%q\n", i, id.Hex(), out.Urls[i])
			}
			if len(out.TeeIds) == 0 {
				fmt.Println("  (none)")
			}
		}
	}

	for _, raw := range flag.Args() {
		id := common.HexToAddress(raw)
		fmt.Printf("\n=== TEE %s ===\n", id.Hex())

		m, err := mm.GetTeeMachine(opts, id)
		if err != nil {
			fmt.Printf("  getTeeMachine ERROR: %v\n", err)
		} else {
			fmt.Printf("  getTeeMachine: teeId=%s teeProxyId=%s url=%q\n", m.TeeId.Hex(), m.TeeProxyId.Hex(), m.Url)
			if m.TeeId == (common.Address{}) {
				fmt.Println("  -> EMPTY/UNREGISTERED")
			}
		}

		st, err := mm.GetTeeMachineStatus(opts, id)
		if err != nil {
			fmt.Printf("  getTeeMachineStatus ERROR: %v\n", err)
		} else {
			fmt.Printf("  getTeeMachineStatus: %d\n", st)
		}

		owner, err := mm.GetTeeMachineOwner(opts, id)
		if err != nil {
			fmt.Printf("  getTeeMachineOwner ERROR: %v\n", err)
		} else {
			fmt.Printf("  getTeeMachineOwner: %s\n", owner.Hex())
		}

		extID, err := mm.GetExtensionId(opts, id)
		if err != nil {
			fmt.Printf("  getExtensionId ERROR: %v\n", err)
		} else {
			fmt.Printf("  getExtensionId: %s\n", extID.String())
		}

		ts, err := mm.GetLastStatusChangeTs(opts, id)
		if err != nil {
			fmt.Printf("  getLastStatusChangeTs ERROR: %v\n", err)
		} else {
			fmt.Printf("  getLastStatusChangeTs: %s\n", ts.String())
		}

		spid, err := mm.GetInitialSigningPolicyId(opts, id)
		if err != nil {
			fmt.Printf("  getInitialSigningPolicyId ERROR: %v\n", err)
		} else {
			fmt.Printf("  getInitialSigningPolicyId: %d\n", spid)
		}
	}
}
