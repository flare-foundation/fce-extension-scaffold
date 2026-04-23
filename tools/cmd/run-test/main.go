package main

import (
	"encoding/json"
	"flag"
	"strings"
	"time"

	"extension-scaffold/tools/pkg/configs"
	"extension-scaffold/tools/pkg/fccutils"
	"extension-scaffold/tools/pkg/support"
	instrutils "extension-scaffold/tools/pkg/utils"

	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-flare-common/pkg/logger"
	"github.com/pkg/errors"
)

type SayHelloResponse struct {
	Greeting       string `json:"greeting"`
	GreetingNumber int    `json:"greetingNumber"`
}

type SayGoodbyeResponse struct {
	Farewell       string `json:"farewell"`
	FarewellNumber int    `json:"farewellNumber"`
}

func main() {
	af := flag.String("a", configs.AddressesFile, "file with deployed addresses")
	cf := flag.String("c", configs.ChainNodeURL, "chain node url")
	pf := flag.String("p", configs.ExtensionProxyURL, "extension proxy url")
	instructionSenderF := flag.String("instructionSender", "", "instructionSender address")
	flag.Parse()

	instructionSenderAddress := common.HexToAddress(*instructionSenderF)

	testSupport, err := support.DefaultSupport(*af, *cf)
	if err != nil {
		fccutils.FatalWithCause(err)
	}

	// --- Generic: configure contract -----------------------------------------
	logger.Infof("Setting extension ID on instruction sender...")
	err = instrutils.SetExtensionId(testSupport, instructionSenderAddress)
	if err != nil {
		if strings.Contains(err.Error(), "already set") || strings.Contains(err.Error(), "Extension ID already set") {
			logger.Infof("Extension ID already set on contract, continuing")
		} else {
			logger.Errorf("setExtensionId failed: %s", err)
			fccutils.FatalWithCause(errors.Errorf(
				"setExtensionId failed — is the extension registered? Check that pre-build.sh completed successfully. Error: %s", err))
		}
	}

	// --- Test case 1: Send a SAY_HELLO instruction ---
	logger.Infof("Sending SAY_HELLO instruction...")

	payload, err := json.Marshal(map[string]interface{}{
		"name": "World",
	})
	if err != nil {
		fccutils.FatalWithCause(err)
	}

	instructionId, _, err := instrutils.SendSayHello(testSupport, instructionSenderAddress, payload)
	if err != nil {
		fccutils.FatalWithCause(err)
	}
	logger.Infof("Instruction sent. ID: %s", instructionId.Hex())

	time.Sleep(5 * time.Second)

	err = verifyHelloResult(*pf, instructionId)
	if err != nil {
		fccutils.FatalWithCause(err)
	}
	logger.Infof("Test passed: SAY_HELLO instruction processed successfully")

	// --- Test case 2: Send a SAY_GOODBYE instruction ---
	logger.Infof("Sending SAY_GOODBYE instruction...")

	goodbyeInstructionId, _, err := instrutils.SendSayGoodbye(testSupport, instructionSenderAddress, "World", "heading out")
	if err != nil {
		fccutils.FatalWithCause(err)
	}
	logger.Infof("Instruction sent. ID: %s", goodbyeInstructionId.Hex())

	time.Sleep(5 * time.Second)

	err = verifyGoodbyeResult(*pf, goodbyeInstructionId)
	if err != nil {
		fccutils.FatalWithCause(err)
	}
	logger.Infof("Test passed: SAY_GOODBYE instruction processed successfully")

	logger.Infof("All tests passed.")
}

func verifyHelloResult(proxyURL string, instructionId common.Hash) error {
	// --- Generic: poll proxy for result (do not modify) ---
	actionResponse, err := fccutils.ActionResult(proxyURL, instructionId)
	if err != nil {
		return err
	}
	actionResult := actionResponse.Result

	if actionResult.Status == 0 {
		return errors.Errorf("instruction processing failed: %s", actionResult.Log)
	}
	if actionResult.Status == 2 {
		return errors.New("instruction still pending after polling, expected completed")
	}

	if len(actionResult.Data) == 0 {
		return errors.New("expected response data but got none")
	}

	var resp SayHelloResponse
	err = json.Unmarshal(actionResult.Data, &resp)
	if err != nil {
		return errors.Errorf("failed to unmarshal response: %s", err)
	}

	if resp.Greeting == "" {
		return errors.New("expected non-empty Greeting")
	}
	if resp.GreetingNumber < 1 {
		return errors.Errorf("expected GreetingNumber >= 1, got %d", resp.GreetingNumber)
	}

	logger.Infof("Response data: %+v", resp)

	return nil
}

func verifyGoodbyeResult(proxyURL string, instructionId common.Hash) error {
	actionResponse, err := fccutils.ActionResult(proxyURL, instructionId)
	if err != nil {
		return err
	}
	actionResult := actionResponse.Result

	if actionResult.Status == 0 {
		return errors.Errorf("instruction processing failed: %s", actionResult.Log)
	}
	if actionResult.Status == 2 {
		return errors.New("instruction still pending after polling, expected completed")
	}

	if len(actionResult.Data) == 0 {
		return errors.New("expected response data but got none")
	}

	var resp SayGoodbyeResponse
	err = json.Unmarshal(actionResult.Data, &resp)
	if err != nil {
		return errors.Errorf("failed to unmarshal response: %s", err)
	}

	if resp.Farewell == "" {
		return errors.New("expected non-empty Farewell")
	}
	if resp.FarewellNumber < 1 {
		return errors.Errorf("expected FarewellNumber >= 1, got %d", resp.FarewellNumber)
	}

	logger.Infof("Response data: %+v", resp)

	return nil
}
