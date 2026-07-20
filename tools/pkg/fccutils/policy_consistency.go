package fccutils

import (
	"context"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/flare-foundation/go-flare-common/pkg/logger"

	"extension-scaffold/tools/pkg/support"
)

// ErrPolicyOutOfSync indicates the FTDC proxy's latest signing policy does not
// match the on-chain current reward epoch. Callers can test for it with errors.Is.
var ErrPolicyOutOfSync = errors.New("FTDC proxy signing policy out of sync with on-chain reward epoch")

// policyInSync reports whether the proxy's last signing policy id is acceptable
// for the given on-chain current reward epoch. The proxy is legitimately one
// epoch ahead during the ~2h window after a new signing policy is initialized
// but before its reward epoch starts (same tolerance as tee-node walletutils).
func policyInSync(onchainEpoch, proxyPolicyID uint64) bool {
	return proxyPolicyID == onchainEpoch || proxyPolicyID == onchainEpoch+1
}

// outOfSyncError builds an actionable error explaining the mismatch and its
// consequence (the 404 "no round" rejection from data providers).
func outOfSyncError(onchainEpoch, proxyPolicyID uint64) error {
	var rel string
	if proxyPolicyID < onchainEpoch {
		rel = fmt.Sprintf("proxy is %d epoch(s) behind", onchainEpoch-proxyPolicyID)
	} else {
		rel = fmt.Sprintf("proxy is %d epoch(s) ahead", proxyPolicyID-onchainEpoch)
	}
	return fmt.Errorf("%w: proxy signing policy %d vs on-chain current reward epoch %d (%s); "+
		"data providers will reject the availability check with 404 \"no round\"; "+
		"wait for the proxy to catch up to the current signing policy, or check the proxy's "+
		"initial_signing_policy_offset and C-chain indexer sync",
		ErrPolicyOutOfSync, proxyPolicyID, onchainEpoch, rel)
}

// CheckFTDCProxyPolicyConsistency compares the on-chain current reward epoch
// (FlareSystemsManager.getCurrentRewardEpochId) against the signing policy id
// reported by the FTDC proxy at ftdcProxyURL (TeeInfo.LastSigningPolicyID).
//
// It returns a wrapped ErrPolicyOutOfSync only on a CONFIRMED mismatch. If
// either lookup fails (RPC down, proxy not up yet) it logs a warning and
// returns nil, so the gate never turns a transient into a blocked registration.
func CheckFTDCProxyPolicyConsistency(s *support.Support, ftdcProxyURL string) error {
	callOpts := &bind.CallOpts{Context: context.Background()}
	onchain, err := s.FlareSystemManager.GetCurrentRewardEpochId(callOpts)
	if err != nil {
		logger.Warnf("policy consistency check skipped: could not query on-chain reward epoch: %v", err)
		return nil
	}
	if onchain == nil {
		logger.Warnf("policy consistency check skipped: on-chain reward epoch is nil")
		return nil
	}

	info, err := TeeInfo(ftdcProxyURL)
	if err != nil {
		logger.Warnf("policy consistency check skipped: could not fetch FTDC proxy info from %s: %v", ftdcProxyURL, err)
		return nil
	}

	onchainEpoch := onchain.Uint64()
	proxyPolicyID := uint64(info.TeeInfo.LastSigningPolicyID)
	if !policyInSync(onchainEpoch, proxyPolicyID) {
		return outOfSyncError(onchainEpoch, proxyPolicyID)
	}

	logger.Infof("policy consistency OK: FTDC proxy signing policy %d matches on-chain reward epoch %d (tolerance +1)", proxyPolicyID, onchainEpoch)
	return nil
}
