package fccutils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/flare-foundation/go-flare-common/pkg/logger"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/flare-foundation/go-flare-common/pkg/tee/attestation/googlecloud"
	"github.com/flare-foundation/tee-node/pkg/attestation"
	"github.com/flare-foundation/tee-node/pkg/types"
	"github.com/pkg/errors"
)

const repeats = 15

func TeeInfo(nodeURL string) (*types.SignedTeeInfoResponse, error) {
	result, err := http.Get(nodeURL + "/info")
	if err != nil {
		return nil, errors.Errorf("%s", err)
	}
	defer result.Body.Close()

	var teeInfo types.SignedTeeInfoResponse
	err = json.NewDecoder(result.Body).Decode(&teeInfo)
	if err != nil {
		return nil, errors.Errorf("%s", err)
	}

	return &teeInfo, nil
}

func CodeHashAndPlatform(attestationString string) (common.Hash, common.Hash, error) {
	claims := attestation.NeededClaims{}
	_, _, err := googlecloud.ParsePKITokenUnverifiedClaims(attestationString, &claims)
	if err != nil {
		return common.Hash{}, common.Hash{}, errors.Errorf("%s", err)
	}

	codeHash, err := claims.CodeHash()
	if err != nil {
		return common.Hash{}, common.Hash{}, errors.Errorf("%s", err)
	}
	platform, err := claims.Platform()
	if err != nil {
		return common.Hash{}, common.Hash{}, errors.Errorf("%s", err)
	}

	return codeHash, platform, nil
}

func TeeProxyId(teeInfo *types.SignedTeeInfoResponse) (common.Address, common.Address, error) {
	pubKey, err := types.ParsePubKey(teeInfo.TeeInfo.PublicKey)
	if err != nil {
		return common.Address{}, common.Address{}, errors.Errorf("%s", err)
	}

	teeID := crypto.PubkeyToAddress(*pubKey)

	hash, err := teeInfo.TeeInfo.Hash()
	if err != nil {
		return common.Address{}, common.Address{}, errors.Errorf("%s", err)
	}
	proxyPubKey, err := crypto.SigToPub(accounts.TextHash(hash), teeInfo.ProxySignature)
	if err != nil {
		return common.Address{}, common.Address{}, errors.Errorf("%s", err)
	}
	proxyID := crypto.PubkeyToAddress(*proxyPubKey)

	return teeID, proxyID, nil
}

func ActionResult(nodeURL string, actionID common.Hash) (*types.ActionResponse, error) {
	var result *http.Response
	var err error
	for range repeats {
		result, err = http.Get(nodeURL + "/action/result/" + actionID.Hex())
		if err == nil && result.StatusCode == http.StatusOK {
			break
		}
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		return nil, errors.Errorf("%s", err)
	}
	if result.StatusCode != http.StatusOK {
		logger.Warnf("action result status not ok: got: %d for %s, %s", result.StatusCode, actionID.Hex(), nodeURL)
		return nil, errors.Errorf("action result status not ok, got: %d", result.StatusCode)
	}
	defer result.Body.Close()

	var response types.ActionResponse
	err = json.NewDecoder(result.Body).Decode(&response)
	if err != nil {
		return nil, errors.Errorf("%s", err)
	}

	return &response, nil
}

func SetProxyUrl(configurationPort int, proxyPort int) error {
	url := fmt.Sprintf("http://localhost:%d", proxyPort)
	request := types.ConfigureProxyURLRequest{
		URL: &url,
	}

	body, err := json.Marshal(request)
	if err != nil {
		return err
	}

	url = fmt.Sprintf("http://localhost:%d/proxy", configurationPort)
	logger.Infof("Setting proxy url on tee: %s", url)
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

func SetInitialOwner(configurationPort int, ownerAddress common.Address) error {
	request := types.ConfigureInitialOwnerRequest{
		Owner: &ownerAddress,
	}

	body, err := json.Marshal(request)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("http://localhost:%d/initial-owner", configurationPort)
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

func SetExtensionID(configurationPort int, extensionID common.Hash) error {
	request := types.ConfigureExtensionIDRequest{
		ExtensionID: &extensionID,
	}

	body, err := json.Marshal(request)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("http://localhost:%d/extension-id", configurationPort)
	logger.Infof("Setting extension id on tee: %s", url)
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}
