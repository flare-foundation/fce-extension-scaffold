package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"extension-scaffold/tools/pkg/configs"
	"extension-scaffold/tools/pkg/support"
	"extension-scaffold/tools/pkg/validate"

	"github.com/ethereum/go-ethereum/common"
)

// stepsNeedingChain are the step values that require a live chain client.
// services, tee-version, tee-machine, and test only read env vars / hit HTTP endpoints.
var stepsNeedingChain = map[string]bool{
	"deploy":   true,
	"register": true,
	"all":      true,
}

func main() {
	af := flag.String("a", configs.AddressesFile, "file with deployed addresses")
	cf := flag.String("c", configs.ChainNodeURL, "chain node url")
	configPath := flag.String("config", "", "path to extension.env for post-deploy validation")
	step := flag.String("step", "all", "which step to check: deploy, register, services, tee-version, tee-machine, test, or all")
	checksFilter := flag.String("checks", "", "comma-separated check IDs to include (e.g. D5,R2); all others are hidden")
	jsonOut := flag.Bool("json", false, "output as JSON instead of colored terminal")
	flag.Parse()

	report := &validate.Report{}

	switch *step {
	case "deploy", "register", "all":
		s, err := support.DefaultSupport(*af, *cf)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error initializing support: %v\n", err)
			fmt.Fprintf(os.Stderr, "\nHints:\n")
			fmt.Fprintf(os.Stderr, "  - Check that the addresses file exists: %s\n", *af)
			fmt.Fprintf(os.Stderr, "  - Check that the chain node is running: %s\n", *cf)
			fmt.Fprintf(os.Stderr, "  - If using a custom key, ensure DEPLOYMENT_PRIVATE_KEY is set in .env\n")
			os.Exit(1)
		}

		addresses := map[string]common.Address{
			"TeeExtensionRegistry": s.Addresses.TeeExtensionRegistry,
			"TeeMachineRegistry":   s.Addresses.TeeMachineRegistry,
		}

		switch *step {
		case "deploy":
			validate.RegisterDeployChecks(report, s.ChainClient, s.Prv, addresses, *configPath)
		case "register":
			validate.RegisterRegistrationChecks(report, s.ChainClient, s.Prv, s.TeeExtensionRegistry, s.TeeOwnerAllowlist, *configPath)
		case "all":
			validate.RegisterDeployChecks(report, s.ChainClient, s.Prv, addresses, *configPath)
			validate.RegisterRegistrationChecks(report, s.ChainClient, s.Prv, s.TeeExtensionRegistry, s.TeeOwnerAllowlist, *configPath)
			validate.RegisterServicesChecks(report, *configPath)
			validate.RegisterTeeVersionChecks(report, *configPath)
			validate.RegisterTeeMachineChecks(report, *configPath)
			validate.RegisterTestChecks(report, *configPath)
		}

	case "services":
		validate.RegisterServicesChecks(report, *configPath)
	case "tee-version":
		validate.RegisterTeeVersionChecks(report, *configPath)
	case "tee-machine":
		validate.RegisterTeeMachineChecks(report, *configPath)
	case "test":
		validate.RegisterTestChecks(report, *configPath)
	default:
		fmt.Fprintf(os.Stderr, "Unknown step: %q\n", *step)
		fmt.Fprintf(os.Stderr, "Valid steps: deploy, register, services, tee-version, tee-machine, test, all\n")
		os.Exit(1)
	}

	// If --checks is set, filter results to only the requested IDs.
	if *checksFilter != "" {
		allowed := make(map[string]bool)
		for _, id := range strings.Split(*checksFilter, ",") {
			if t := strings.TrimSpace(id); t != "" {
				allowed[t] = true
			}
		}
		filtered := report.Results[:0]
		for _, res := range report.Results {
			if allowed[res.ID] {
				filtered = append(filtered, res)
			}
		}
		report.Results = filtered
	}

	if *jsonOut {
		report.PrintJSON()
	} else {
		report.Print()
	}

	if report.HasFailures() {
		os.Exit(1)
	}
}
