package validate

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/teeextensionregistry"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/teeownerallowlist"
	"github.com/flare-foundation/tee-node/pkg/wallets"
)

var (
	addressRegex     = regexp.MustCompile(`^0x[0-9a-fA-F]{40}$`)
	extensionIDRegex = regexp.MustCompile(`^0x[0-9a-fA-F]{64}$`)
)

// RegisterDeployChecks adds all Step 1 (Contract Deployment) checks to a Report.
func RegisterDeployChecks(r *Report, client *ethclient.Client, key *ecdsa.PrivateKey, addresses map[string]common.Address, extensionEnvPath string) {
	// D3: For each address, check that it is not zero.
	zeroAddrs := make(map[string]bool)
	for label, addr := range addresses {
		if err := AddressNotZero(addr, label); err != nil {
			r.Add(CheckResult{
				Step:    "deploy",
				ID:      "D3",
				Name:    label + " address not zero",
				Status:  FAIL,
				Message: err.Error(),
				Fix:     "Re-run the deploy script or check deployed-addresses.json",
			})
			zeroAddrs[label] = true
		} else {
			r.Add(CheckResult{
				Step:    "deploy",
				ID:      "D3",
				Name:    label + " address not zero",
				Status:  PASS,
				Message: addr.Hex(),
			})
		}
	}

	// D1/D2: For each non-zero address, check that code is deployed.
	for label, addr := range addresses {
		if zeroAddrs[label] {
			continue
		}
		if err := AddressHasCode(client, addr, label); err != nil {
			r.Add(CheckResult{
				Step:    "deploy",
				ID:      "D1",
				Name:    label + " has deployed code",
				Status:  FAIL,
				Message: err.Error(),
				Fix:     "Deploy contracts or check CHAIN_URL / deployed-addresses.json",
			})
		} else {
			r.Add(CheckResult{
				Step:    "deploy",
				ID:      "D1",
				Name:    label + " has deployed code",
				Status:  PASS,
				Message: fmt.Sprintf("code found at %s", addr.Hex()),
			})
		}
	}

	// D5: Check deployer key source.
	r.Add(CheckDeployerKeySource())

	// D5: Check deployer funds.
	if key != nil && client != nil {
		if err := KeyHasFunds(client, key, MinDeployBalance); err != nil {
			r.Add(CheckResult{
				Step:    "deploy",
				ID:      "D5",
				Name:    "deployer has sufficient funds",
				Status:  FAIL,
				Message: err.Error(),
				Fix:     "Fund the deployer account before deploying",
			})
		} else {
			r.Add(CheckResult{
				Step:    "deploy",
				ID:      "D5",
				Name:    "deployer has sufficient funds",
				Status:  PASS,
				Message: "balance above minimum threshold",
			})
		}
	}

	// D6: Check chain ID.
	if client != nil {
		r.Add(checkChainID(client))
	}

	// D7: Check extension.env format.
	if extensionEnvPath != "" {
		results := CheckExtensionEnvFormat(extensionEnvPath)
		for _, res := range results {
			r.Add(res)
		}
	}
}

// CheckDeployerKeySource checks whether the deployer private key is configured.
func CheckDeployerKeySource() CheckResult {
	privKey := os.Getenv("DEPLOYMENT_PRIVATE_KEY")
	if privKey != "" {
		return CheckResult{
			Step:    "deploy",
			ID:      "D5",
			Name:    "deployer key source",
			Status:  PASS,
			Message: "DEPLOYMENT_PRIVATE_KEY is set",
		}
	}

	localMode := os.Getenv("LOCAL_MODE")
	if localMode == "true" || localMode == "" {
		return CheckResult{
			Step:    "deploy",
			ID:      "D5",
			Name:    "deployer key source",
			Status:  PASS,
			Message: "using Hardhat dev key (LOCAL_MODE)",
		}
	}

	return CheckResult{
		Step:    "deploy",
		ID:      "D5",
		Name:    "deployer key source",
		Status:  WARN,
		Message: "DEPLOYMENT_PRIVATE_KEY not set — using Hardhat dev key which has no funds on Coston2",
		Fix:     "Set DEPLOYMENT_PRIVATE_KEY in .env to a funded account on the target network",
	}
}

// CheckExtensionEnvFormat parses an extension.env file and validates its entries.
func CheckExtensionEnvFormat(path string) []CheckResult {
	f, err := os.Open(path)
	if err != nil {
		return []CheckResult{{
			Step:    "deploy",
			ID:      "D7",
			Name:    "extension.env exists",
			Status:  SKIP,
			Message: "file not found: " + path,
		}}
	}
	defer f.Close()

	// Parse key=value pairs.
	vars := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			vars[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}

	var results []CheckResult
	fix := "Delete config/extension.env and re-run scripts/pre-build.sh"

	// Check INSTRUCTION_SENDER.
	if val, ok := vars["INSTRUCTION_SENDER"]; !ok {
		results = append(results, CheckResult{
			Step:    "deploy",
			ID:      "D7",
			Name:    "INSTRUCTION_SENDER in extension.env",
			Status:  FAIL,
			Message: "INSTRUCTION_SENDER not found in extension.env",
			Fix:     fix,
		})
	} else if !addressRegex.MatchString(val) {
		results = append(results, CheckResult{
			Step:    "deploy",
			ID:      "D7",
			Name:    "INSTRUCTION_SENDER in extension.env",
			Status:  FAIL,
			Message: fmt.Sprintf("INSTRUCTION_SENDER has invalid format: %q", val),
			Fix:     fix,
		})
	} else {
		results = append(results, CheckResult{
			Step:    "deploy",
			ID:      "D7",
			Name:    "INSTRUCTION_SENDER in extension.env",
			Status:  PASS,
			Message: val,
		})
	}

	// Check EXTENSION_ID.
	if val, ok := vars["EXTENSION_ID"]; !ok {
		results = append(results, CheckResult{
			Step:    "deploy",
			ID:      "D7",
			Name:    "EXTENSION_ID in extension.env",
			Status:  FAIL,
			Message: "EXTENSION_ID not found in extension.env",
			Fix:     fix,
		})
	} else if !extensionIDRegex.MatchString(val) {
		results = append(results, CheckResult{
			Step:    "deploy",
			ID:      "D7",
			Name:    "EXTENSION_ID in extension.env",
			Status:  FAIL,
			Message: fmt.Sprintf("EXTENSION_ID has invalid format: %q", val),
			Fix:     fix,
		})
	} else {
		results = append(results, CheckResult{
			Step:    "deploy",
			ID:      "D7",
			Name:    "EXTENSION_ID in extension.env",
			Status:  PASS,
			Message: val,
		})
	}

	return results
}

// parseExtensionEnv reads EXTENSION_ID and INSTRUCTION_SENDER from an extension.env file.
// Returns empty strings and nil error if file doesn't exist.
func parseExtensionEnv(path string) (extensionID string, instructionSender string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return "", "", nil
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		switch key {
		case "EXTENSION_ID":
			extensionID = val
		case "INSTRUCTION_SENDER":
			instructionSender = val
		}
	}
	return extensionID, instructionSender, nil
}

// RegisterRegistrationChecks adds all registration (R1-R7) checks to a Report.
func RegisterRegistrationChecks(
	r *Report,
	client *ethclient.Client,
	key *ecdsa.PrivateKey,
	registry *teeextensionregistry.TeeExtensionRegistry,
	allowlist *teeownerallowlist.TeeOwnerAllowlist,
	extensionEnvPath string,
) {
	// If no config path provided, skip all registration checks.
	if extensionEnvPath == "" {
		r.Add(CheckResult{
			Step:    "register",
			ID:      "R1-R7",
			Name:    "registration checks",
			Status:  SKIP,
			Message: "no --config path provided",
		})
		return
	}

	// Parse extension.env.
	extIDStr, instrSender, err := parseExtensionEnv(extensionEnvPath)
	if err != nil {
		r.Add(CheckResult{
			Step:    "register",
			ID:      "R1-R7",
			Name:    "registration checks",
			Status:  FAIL,
			Message: fmt.Sprintf("failed to parse extension.env: %v", err),
		})
		return
	}
	if extIDStr == "" && instrSender == "" {
		r.Add(CheckResult{
			Step:    "register",
			ID:      "R1-R7",
			Name:    "registration checks",
			Status:  SKIP,
			Message: "config/extension.env not found — run pre-build.sh first",
		})
		return
	}

	callOpts := &bind.CallOpts{Context: context.Background()}

	// R1: Check extensions counter.
	var counter *big.Int
	if registry != nil && client != nil {
		counter, err = registry.ExtensionsCounter(callOpts)
		if err != nil {
			r.Add(CheckResult{
				Step:    "register",
				ID:      "R1",
				Name:    "extensions counter",
				Status:  FAIL,
				Message: fmt.Sprintf("failed to query extensions counter: %v", err),
			})
		} else if counter.Sign() == 0 {
			r.Add(CheckResult{
				Step:    "register",
				ID:      "R1",
				Name:    "extensions counter",
				Status:  WARN,
				Message: "extensions counter is 0 — no extensions registered yet",
			})
		} else {
			r.Add(CheckResult{
				Step:    "register",
				ID:      "R1",
				Name:    "extensions counter",
				Status:  PASS,
				Message: fmt.Sprintf("%s extension(s) registered", counter.String()),
			})
		}
	}

	// Validate EXTENSION_ID format.
	if !extensionIDRegex.MatchString(extIDStr) {
		r.Add(CheckResult{
			Step:    "register",
			ID:      "R2",
			Name:    "EXTENSION_ID format",
			Status:  FAIL,
			Message: fmt.Sprintf("malformed EXTENSION_ID: %q", extIDStr),
			Fix:     "Delete config/extension.env and re-run scripts/pre-build.sh",
		})
		return
	}

	// Parse extension ID from hex.
	extensionID := new(big.Int)
	extensionID.SetString(extIDStr[2:], 16)

	// R2: Check instruction sender matches on-chain.
	if registry != nil && client != nil && addressRegex.MatchString(instrSender) {
		onChainSender, err := registry.GetTeeExtensionInstructionsSender(callOpts, extensionID)
		if err != nil {
			r.Add(CheckResult{
				Step:    "register",
				ID:      "R2",
				Name:    "instruction sender matches on-chain",
				Status:  FAIL,
				Message: fmt.Sprintf("failed to query instruction sender: %v", err),
			})
		} else {
			expectedAddr := common.HexToAddress(instrSender)
			if onChainSender == expectedAddr {
				r.Add(CheckResult{
					Step:    "register",
					ID:      "R2",
					Name:    "instruction sender matches on-chain",
					Status:  PASS,
					Message: fmt.Sprintf("on-chain sender %s matches extension.env", onChainSender.Hex()),
				})
			} else {
				r.Add(CheckResult{
					Step:    "register",
					ID:      "R2",
					Name:    "instruction sender matches on-chain",
					Status:  FAIL,
					Message: fmt.Sprintf("on-chain sender %s != extension.env %s", onChainSender.Hex(), instrSender),
					Fix:     "Re-register the extension or update extension.env",
				})
			}
		}
	}

	// R3: Check deployer is allowed TEE machine owner.
	if key != nil && allowlist != nil && client != nil {
		deployerAddr := crypto.PubkeyToAddress(key.PublicKey)
		allowed, err := allowlist.IsAllowedTeeMachineOwner(callOpts, extensionID, deployerAddr)
		if err != nil {
			r.Add(CheckResult{
				Step:    "register",
				ID:      "R3",
				Name:    "deployer is allowed TEE machine owner",
				Status:  FAIL,
				Message: fmt.Sprintf("failed to query allowlist: %v", err),
			})
		} else if allowed {
			r.Add(CheckResult{
				Step:    "register",
				ID:      "R3",
				Name:    "deployer is allowed TEE machine owner",
				Status:  PASS,
				Message: "deployer is on the TEE machine owner allowlist",
			})
		} else {
			r.Add(CheckResult{
				Step:    "register",
				ID:      "R3",
				Name:    "deployer is allowed TEE machine owner",
				Status:  FAIL,
				Message: "deployer is NOT on the TEE machine owner allowlist",
				Fix:     "Run the register step to add the deployer as a TEE machine owner",
			})
		}
	}

	// R4: Check EVM key type is supported.
	if registry != nil && client != nil {
		var evmTypeBytes [32]byte
		copy(evmTypeBytes[:], wallets.EVMType[:])
		supported, err := registry.IsKeyTypeSupported(callOpts, extensionID, evmTypeBytes)
		if err != nil {
			r.Add(CheckResult{
				Step:    "register",
				ID:      "R4",
				Name:    "EVM key type supported",
				Status:  FAIL,
				Message: fmt.Sprintf("failed to query key type support: %v", err),
			})
		} else if supported {
			r.Add(CheckResult{
				Step:    "register",
				ID:      "R4",
				Name:    "EVM key type supported",
				Status:  PASS,
				Message: "EVM key type is supported for this extension",
			})
		} else {
			r.Add(CheckResult{
				Step:    "register",
				ID:      "R4",
				Name:    "EVM key type supported",
				Status:  FAIL,
				Message: "EVM key type is NOT supported for this extension",
				Fix:     "Run the register step to add EVM key type support",
			})
		}
	}

	// R7: Check for duplicate instruction sender across extensions.
	if counter != nil && counter.Sign() > 0 && addressRegex.MatchString(instrSender) && registry != nil && client != nil {
		expectedAddr := common.HexToAddress(instrSender)
		var matchingIDs []string
		for i := int64(0); i < counter.Int64(); i++ {
			id := big.NewInt(i)
			sender, err := registry.GetTeeExtensionInstructionsSender(callOpts, id)
			if err != nil {
				continue
			}
			if sender == expectedAddr {
				matchingIDs = append(matchingIDs, id.String())
			}
		}
		if len(matchingIDs) > 1 {
			r.Add(CheckResult{
				Step:    "register",
				ID:      "R7",
				Name:    "unique instruction sender",
				Status:  WARN,
				Message: fmt.Sprintf("INSTRUCTION_SENDER %s is used by multiple extensions: %s", instrSender, strings.Join(matchingIDs, ", ")),
			})
		} else {
			r.Add(CheckResult{
				Step:    "register",
				ID:      "R7",
				Name:    "unique instruction sender",
				Status:  PASS,
				Message: "instruction sender is unique across registered extensions",
			})
		}
	}
}

// checkChainID queries the connected chain for its ID and reports the network name.
func checkChainID(client *ethclient.Client) CheckResult {
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()

	chainID, err := client.ChainID(ctx)
	if err != nil {
		return CheckResult{
			Step:    "deploy",
			ID:      "D6",
			Name:    "chain ID",
			Status:  FAIL,
			Message: fmt.Sprintf("failed to get chain ID: %v", err),
			Fix:     "Check CHAIN_URL",
		}
	}

	name := chainName(chainID)
	return CheckResult{
		Step:    "deploy",
		ID:      "D6",
		Name:    "chain ID",
		Status:  PASS,
		Message: fmt.Sprintf("chain ID %s (%s)", chainID.String(), name),
	}
}

// chainName returns a human-readable name for known chain IDs.
func chainName(id interface{ String() string }) string {
	switch id.String() {
	case "31337":
		return "Hardhat/Anvil (local devnet)"
	case "114":
		return "Coston2 (testnet)"
	case "14":
		return "Flare (mainnet)"
	default:
		return "unknown network"
	}
}

// RegisterServicesChecks adds service-readiness checks (S2, S5, S7, S9, S10) to a Report.
func RegisterServicesChecks(r *Report, extensionEnvPath string) {
	// S2: EXTENSION_ID is set and well-formed.
	if extensionEnvPath == "" {
		r.Add(CheckResult{
			Step:    "services",
			ID:      "S2",
			Name:    "EXTENSION_ID is set and well-formed",
			Status:  SKIP,
			Message: "run pre-build.sh first",
		})
	} else {
		extID, _, _ := parseExtensionEnv(extensionEnvPath)
		if extID == "" {
			r.Add(CheckResult{
				Step:    "services",
				ID:      "S2",
				Name:    "EXTENSION_ID is set and well-formed",
				Status:  FAIL,
				Message: "EXTENSION_ID is empty or not found in extension.env",
				Fix:     "Delete config/extension.env and re-run scripts/pre-build.sh",
			})
		} else if !extensionIDRegex.MatchString(extID) {
			r.Add(CheckResult{
				Step:    "services",
				ID:      "S2",
				Name:    "EXTENSION_ID is set and well-formed",
				Status:  FAIL,
				Message: fmt.Sprintf("EXTENSION_ID has invalid format: %q", extID),
				Fix:     "Delete config/extension.env and re-run scripts/pre-build.sh",
			})
		} else {
			r.Add(CheckResult{
				Step:    "services",
				ID:      "S2",
				Name:    "EXTENSION_ID is set and well-formed",
				Status:  PASS,
				Message: extID,
			})
		}
	}

	// S5: Extension port not in use.
	port := os.Getenv("EXTENSION_PORT")
	if port == "" {
		port = "7702"
	}
	conn, err := net.DialTimeout("tcp", "localhost:"+port, 1*time.Second)
	if err == nil {
		conn.Close()
		r.Add(CheckResult{
			Step:    "services",
			ID:      "S5",
			Name:    "extension port not in use",
			Status:  WARN,
			Message: fmt.Sprintf("port %s is already in use — extension server will fail to bind", port),
			Fix:     "Stop the process using that port or change EXTENSION_PORT",
		})
	} else {
		r.Add(CheckResult{
			Step:    "services",
			ID:      "S5",
			Name:    "extension port not in use",
			Status:  PASS,
			Message: "port available",
		})
	}

	// S7: Port configuration.
	extPort := os.Getenv("EXTENSION_PORT")
	signPort := os.Getenv("SIGN_PORT")
	if extPort != "" || signPort != "" {
		msg := fmt.Sprintf("EXTENSION_PORT=%s, SIGN_PORT=%s", extPort, signPort)
		r.Add(CheckResult{
			Step:    "services",
			ID:      "S7",
			Name:    "port configuration",
			Status:  PASS,
			Message: msg,
		})
	} else {
		r.Add(CheckResult{
			Step:    "services",
			ID:      "S7",
			Name:    "port configuration",
			Status:  PASS,
			Message: "using defaults (standalone: 8080/9090, Docker: 7702/7701)",
		})
	}

	// S9: EXT_PROXY_URL reachable.
	proxyURL := os.Getenv("EXT_PROXY_URL")
	if proxyURL == "" {
		r.Add(CheckResult{
			Step:    "services",
			ID:      "S9",
			Name:    "EXT_PROXY_URL reachable",
			Status:  SKIP,
			Message: "EXT_PROXY_URL not set",
		})
	} else {
		httpClient := &http.Client{Timeout: 5 * time.Second}
		resp, err := httpClient.Get(proxyURL + "/info")
		if err == nil {
			resp.Body.Close()
		}
		if err == nil && resp.StatusCode == http.StatusOK {
			r.Add(CheckResult{
				Step:    "services",
				ID:      "S9",
				Name:    "EXT_PROXY_URL reachable",
				Status:  PASS,
				Message: fmt.Sprintf("proxy reachable at %s", proxyURL),
			})
		} else {
			r.Add(CheckResult{
				Step:    "services",
				ID:      "S9",
				Name:    "EXT_PROXY_URL reachable",
				Status:  WARN,
				Message: fmt.Sprintf("proxy not reachable at %s — is the proxy running?", proxyURL),
				Fix:     "Start the proxy or check EXT_PROXY_URL",
			})
		}
	}

	// S10: PROXY_PRIVATE_KEY set on non-local.
	localMode := os.Getenv("LOCAL_MODE")
	privateKey := os.Getenv("PROXY_PRIVATE_KEY")
	if localMode == "true" || localMode == "" {
		r.Add(CheckResult{
			Step:    "services",
			ID:      "S10",
			Name:    "PROXY_PRIVATE_KEY set on non-local",
			Status:  PASS,
			Message: "local mode, dev key OK",
		})
	} else if localMode == "false" && privateKey == "" {
		r.Add(CheckResult{
			Step:    "services",
			ID:      "S10",
			Name:    "PROXY_PRIVATE_KEY set on non-local",
			Status:  WARN,
			Message: "PROXY_PRIVATE_KEY not set in non-local mode — proxy will use default key",
			Fix:     "Set PROXY_PRIVATE_KEY in .env for non-local deployments",
		})
	} else {
		r.Add(CheckResult{
			Step:    "services",
			ID:      "S10",
			Name:    "PROXY_PRIVATE_KEY set on non-local",
			Status:  PASS,
			Message: "PROXY_PRIVATE_KEY is configured",
		})
	}
}

// RegisterTeeVersionChecks adds TEE version registration checks (V2, V4, V6) to a Report.
func RegisterTeeVersionChecks(r *Report, extensionEnvPath string) {
	// V2: Extension owner key.
	extOwnerKey := os.Getenv("EXTENSION_OWNER_KEY")
	privKey := os.Getenv("DEPLOYMENT_PRIVATE_KEY")
	localMode := os.Getenv("LOCAL_MODE")

	if extOwnerKey != "" {
		r.Add(CheckResult{
			Step:    "tee-version",
			ID:      "V2",
			Name:    "extension owner key",
			Status:  PASS,
			Message: "EXTENSION_OWNER_KEY is configured",
		})
	} else if privKey != "" {
		r.Add(CheckResult{
			Step:    "tee-version",
			ID:      "V2",
			Name:    "extension owner key",
			Status:  PASS,
			Message: "using DEPLOYMENT_PRIVATE_KEY (EXTENSION_OWNER_KEY not set)",
		})
	} else if localMode != "true" && localMode != "" {
		r.Add(CheckResult{
			Step:    "tee-version",
			ID:      "V2",
			Name:    "extension owner key",
			Status:  WARN,
			Message: "neither EXTENSION_OWNER_KEY nor DEPLOYMENT_PRIVATE_KEY set — AddTeeVersion will use dev key which isn't the extension owner on Coston2",
			Fix:     "Set EXTENSION_OWNER_KEY or DEPLOYMENT_PRIVATE_KEY in .env",
		})
	} else {
		r.Add(CheckResult{
			Step:    "tee-version",
			ID:      "V2",
			Name:    "extension owner key",
			Status:  PASS,
			Message: "using dev key (local mode)",
		})
	}

	// V4: Extension ID valid.
	if extensionEnvPath == "" {
		r.Add(CheckResult{
			Step:    "tee-version",
			ID:      "V4",
			Name:    "EXTENSION_ID valid",
			Status:  SKIP,
			Message: "no --config provided",
		})
	} else {
		extID, _, err := parseExtensionEnv(extensionEnvPath)
		if err != nil {
			r.Add(CheckResult{
				Step:    "tee-version",
				ID:      "V4",
				Name:    "EXTENSION_ID valid",
				Status:  FAIL,
				Message: fmt.Sprintf("failed to parse extension.env: %v", err),
			})
		} else if extID == "" {
			// parseExtensionEnv returns empty strings and nil error if file doesn't exist
			if _, statErr := os.Stat(extensionEnvPath); os.IsNotExist(statErr) {
				r.Add(CheckResult{
					Step:    "tee-version",
					ID:      "V4",
					Name:    "EXTENSION_ID valid",
					Status:  SKIP,
					Message: "extension.env not found — run pre-build.sh first",
				})
			} else {
				r.Add(CheckResult{
					Step:    "tee-version",
					ID:      "V4",
					Name:    "EXTENSION_ID valid",
					Status:  FAIL,
					Message: "EXTENSION_ID not set",
					Fix:     "Delete config/extension.env and re-run scripts/pre-build.sh",
				})
			}
		} else if !extensionIDRegex.MatchString(extID) {
			r.Add(CheckResult{
				Step:    "tee-version",
				ID:      "V4",
				Name:    "EXTENSION_ID valid",
				Status:  FAIL,
				Message: "EXTENSION_ID malformed",
				Fix:     "Delete config/extension.env and re-run scripts/pre-build.sh",
			})
		} else {
			r.Add(CheckResult{
				Step:    "tee-version",
				ID:      "V4",
				Name:    "EXTENSION_ID valid",
				Status:  PASS,
				Message: extID,
			})
		}
	}

	// V6: Proxy reachable.
	proxyURL := os.Getenv("EXT_PROXY_URL")
	if proxyURL == "" {
		r.Add(CheckResult{
			Step:    "tee-version",
			ID:      "V6",
			Name:    "proxy reachable",
			Status:  SKIP,
			Message: "EXT_PROXY_URL not set — allow-tee-version uses -p flag",
		})
	} else {
		httpClient := &http.Client{Timeout: 5 * time.Second}
		resp, err := httpClient.Get(proxyURL + "/info")
		if err == nil {
			resp.Body.Close()
		}
		if err == nil && resp.StatusCode == http.StatusOK {
			r.Add(CheckResult{
				Step:    "tee-version",
				ID:      "V6",
				Name:    "proxy reachable",
				Status:  PASS,
				Message: fmt.Sprintf("proxy reachable at %s", proxyURL),
			})
		} else {
			r.Add(CheckResult{
				Step:    "tee-version",
				ID:      "V6",
				Name:    "proxy reachable",
				Status:  WARN,
				Message: fmt.Sprintf("proxy not reachable at %s — start services first", proxyURL),
				Fix:     "Start the proxy or check EXT_PROXY_URL",
			})
		}
	}
}

// RegisterTeeMachineChecks adds TEE machine registration checks (T1, T5, T10) to a Report.
func RegisterTeeMachineChecks(r *Report, extensionEnvPath string) {
	// T1: SIMULATED_TEE mode.
	simulatedTee := os.Getenv("SIMULATED_TEE")
	localMode := os.Getenv("LOCAL_MODE")

	if simulatedTee == "true" && localMode == "false" {
		r.Add(CheckResult{
			Step:    "tee-machine",
			ID:      "T1",
			Name:    "SIMULATED_TEE mode",
			Status:  WARN,
			Message: "SIMULATED_TEE=true on non-local deployment — machine will be registered with test attestation values. Set SIMULATED_TEE=false for real TEE hardware",
			Fix:     "Set SIMULATED_TEE=false in .env when deploying to a real GCP TEE",
		})
	} else if simulatedTee == "true" {
		r.Add(CheckResult{
			Step:    "tee-machine",
			ID:      "T1",
			Name:    "SIMULATED_TEE mode",
			Status:  PASS,
			Message: "simulated TEE mode (local)",
		})
	} else {
		r.Add(CheckResult{
			Step:    "tee-machine",
			ID:      "T1",
			Name:    "SIMULATED_TEE mode",
			Status:  PASS,
			Message: "real attestation mode",
		})
	}

	// T5: NORMAL_PROXY_URL reachable.
	normalProxyURL := os.Getenv("NORMAL_PROXY_URL")
	if normalProxyURL == "" {
		r.Add(CheckResult{
			Step:    "tee-machine",
			ID:      "T5",
			Name:    "NORMAL_PROXY_URL reachable",
			Status:  SKIP,
			Message: "NORMAL_PROXY_URL not set",
		})
	} else {
		httpClient := &http.Client{Timeout: 5 * time.Second}
		resp, err := httpClient.Get(normalProxyURL + "/info")
		if err == nil {
			resp.Body.Close()
		}
		if err == nil && resp.StatusCode == http.StatusOK {
			r.Add(CheckResult{
				Step:    "tee-machine",
				ID:      "T5",
				Name:    "NORMAL_PROXY_URL reachable",
				Status:  PASS,
				Message: fmt.Sprintf("normal proxy reachable at %s", normalProxyURL),
			})
		} else {
			r.Add(CheckResult{
				Step:    "tee-machine",
				ID:      "T5",
				Name:    "NORMAL_PROXY_URL reachable",
				Status:  WARN,
				Message: fmt.Sprintf("normal proxy not reachable at %s — required for FTDC availability check", normalProxyURL),
				Fix:     "Ensure the normal/FTDC proxy is running and NORMAL_PROXY_URL is correct in .env",
			})
		}
	}

	// T10: Host URL reachable.
	extProxyURL := os.Getenv("EXT_PROXY_URL")
	if extProxyURL == "" {
		r.Add(CheckResult{
			Step:    "tee-machine",
			ID:      "T10",
			Name:    "host URL reachable",
			Status:  SKIP,
			Message: "EXT_PROXY_URL not set",
		})
	} else {
		httpClient := &http.Client{Timeout: 5 * time.Second}
		resp, err := httpClient.Get(extProxyURL + "/info")
		if err == nil {
			resp.Body.Close()
		}
		if err == nil && resp.StatusCode == http.StatusOK {
			r.Add(CheckResult{
				Step:    "tee-machine",
				ID:      "T10",
				Name:    "host URL reachable",
				Status:  PASS,
				Message: fmt.Sprintf("host URL reachable at %s", extProxyURL),
			})
		} else {
			r.Add(CheckResult{
				Step:    "tee-machine",
				ID:      "T10",
				Name:    "host URL reachable",
				Status:  WARN,
				Message: fmt.Sprintf("host URL not reachable at %s — machine will be registered with unreachable host. Data providers can't relay instructions", extProxyURL),
				Fix:     "Ensure the host is accessible and EXT_PROXY_URL or -h flag points to the right URL",
			})
		}
	}
}

// RegisterTestChecks adds testing-readiness checks (E1, E3, E6) to a Report.
func RegisterTestChecks(r *Report, extensionEnvPath string) {
	// E1: INSTRUCTION_SENDER valid.
	if extensionEnvPath == "" {
		r.Add(CheckResult{
			Step:    "test",
			ID:      "E1",
			Name:    "INSTRUCTION_SENDER valid",
			Status:  SKIP,
			Message: "no --config provided",
		})
	} else {
		_, instrSender, _ := parseExtensionEnv(extensionEnvPath)
		if _, statErr := os.Stat(extensionEnvPath); os.IsNotExist(statErr) {
			r.Add(CheckResult{
				Step:    "test",
				ID:      "E1",
				Name:    "INSTRUCTION_SENDER valid",
				Status:  SKIP,
				Message: "extension.env not found — run pre-build.sh first",
			})
		} else if instrSender == "" {
			r.Add(CheckResult{
				Step:    "test",
				ID:      "E1",
				Name:    "INSTRUCTION_SENDER valid",
				Status:  FAIL,
				Message: "INSTRUCTION_SENDER not set — run pre-build.sh",
				Fix:     "Run scripts/pre-build.sh to deploy and register the extension",
			})
		} else if !addressRegex.MatchString(instrSender) {
			r.Add(CheckResult{
				Step:    "test",
				ID:      "E1",
				Name:    "INSTRUCTION_SENDER valid",
				Status:  FAIL,
				Message: "INSTRUCTION_SENDER malformed",
				Fix:     "Delete config/extension.env and re-run scripts/pre-build.sh",
			})
		} else {
			r.Add(CheckResult{
				Step:    "test",
				ID:      "E1",
				Name:    "INSTRUCTION_SENDER valid",
				Status:  PASS,
				Message: instrSender,
			})
		}
	}

	// E3: Proxy reachable for test results.
	proxyURL := os.Getenv("EXT_PROXY_URL")
	if proxyURL == "" {
		r.Add(CheckResult{
			Step:    "test",
			ID:      "E3",
			Name:    "proxy reachable for test results",
			Status:  SKIP,
			Message: "EXT_PROXY_URL not set",
		})
	} else {
		httpClient := &http.Client{Timeout: 5 * time.Second}
		resp, err := httpClient.Get(proxyURL + "/info")
		if err == nil {
			resp.Body.Close()
		}
		if err == nil && resp.StatusCode == http.StatusOK {
			r.Add(CheckResult{
				Step:    "test",
				ID:      "E3",
				Name:    "proxy reachable for test results",
				Status:  PASS,
				Message: "proxy reachable for test result polling",
			})
		} else {
			r.Add(CheckResult{
				Step:    "test",
				ID:      "E3",
				Name:    "proxy reachable for test results",
				Status:  WARN,
				Message: "proxy not reachable — test results cannot be polled",
				Fix:     "Start the proxy or check EXT_PROXY_URL",
			})
		}
	}

	// E6: OPType/OPCommand alignment info.
	r.Add(CheckResult{
		Step:    "test",
		ID:      "E6",
		Name:    "OPType/OPCommand alignment info",
		Status:  PASS,
		Message: "OPType/OPCommand hashes are validated at runtime — if a 501 error occurs, check the hash values in the error message",
	})
}

// RegisterStubChecks adds one SKIP result as a placeholder for unimplemented checks.
func RegisterStubChecks(r *Report, step, idRange, name string) {
	r.Add(CheckResult{
		Step:    step,
		ID:      idRange,
		Name:    name,
		Status:  SKIP,
		Message: "not yet implemented",
	})
}
