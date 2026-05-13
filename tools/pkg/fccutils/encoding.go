package fccutils

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-flare-common/pkg/logger"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/fdc2"
	"github.com/pkg/errors"
)

var (
	PlatformIntel   common.Hash = common.HexToHash("4743505f494e54454c5f54445800000000000000000000000000000000000000") // GCP_INTEL_TDX
	PlatformAMD     common.Hash = common.HexToHash("4743505f414d445f534556000000000000000000000000000000000000000000") // GCP_AMD_SEV
	PlatformAMDESEV common.Hash = common.HexToHash("4743505f414d445f5345565f4553000000000000000000000000000000000000") // GCP_AMD_SEV_ES
	TestPlatform    common.Hash = common.HexToHash("544553545f504c4154464f524d00000000000000000000000000000000000000") // TEST_PLATFORM
	TeeCodeHash     common.Hash = common.HexToHash("194844cf417dde867073e5ab7199fa4d21fd82b5dbe2bdea8b3d7fc18d10fdc2")
)

type StackTracer interface {
	StackTrace() errors.StackTrace
}

func FatalWithCause(err error) {
	errCause, ok := err.(StackTracer)
	if ok {
		st := errCause.StackTrace()
		logger.Fatalf("Error: %v %+v", err, st)
	} else {
		logger.Fatalf("Error: %v", err)
	}
}

func EncodeFTDCTeeAvailabilityCheckRequest(data fdc2.ITeeAvailabilityCheckRequestBody) ([]byte, error) {
	return structs.Encode(fdc2.AttestationTypeArguments[fdc2.AvailabilityCheck].Request, data)
}

func DecodeFTDCTeeAvailabilityCheckRequest(data []byte) (fdc2.ITeeAvailabilityCheckRequestBody, error) {
	var request fdc2.ITeeAvailabilityCheckRequestBody
	err := structs.DecodeTo(fdc2.AttestationTypeArguments[fdc2.AvailabilityCheck].Request, data, &request)
	if err != nil {
		return fdc2.ITeeAvailabilityCheckRequestBody{}, errors.Errorf("%s", err)
	}
	return request, nil
}

func EncodeFTDCTeeAvailabilityCheckResponse(data fdc2.ITeeAvailabilityCheckResponseBody) ([]byte, error) {
	return structs.Encode(fdc2.AttestationTypeArguments[fdc2.AvailabilityCheck].Response, data)
}

func DecodeFTDCTeeAvailabilityCheckResponse(data []byte) (fdc2.ITeeAvailabilityCheckResponseBody, error) {
	var request fdc2.ITeeAvailabilityCheckResponseBody
	err := structs.DecodeTo(fdc2.AttestationTypeArguments[fdc2.AvailabilityCheck].Response, data, &request)
	if err != nil {
		return fdc2.ITeeAvailabilityCheckResponseBody{}, errors.Errorf("%s", err)
	}

	return request, nil
}
