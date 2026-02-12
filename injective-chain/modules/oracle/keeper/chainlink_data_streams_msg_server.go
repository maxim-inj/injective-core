package keeper

import (
	"context"
	"fmt"
	"math/big"

	"cosmossdk.io/errors"
	"cosmossdk.io/math"
	"github.com/InjectiveLabs/metrics"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/smartcontractkit/data-streams-sdk/go/feed"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/types"
)

type ChainlinkDataStreamsMsgServer struct {
	Keeper
	svcTags metrics.Tags
}

// NewChainlinkDataStreamsMsgServerImpl returns an implementation of the Chainlink Data Streams MsgServer interface.
func NewChainlinkDataStreamsMsgServerImpl(keeper Keeper) ChainlinkDataStreamsMsgServer {
	return ChainlinkDataStreamsMsgServer{
		Keeper: keeper,
		svcTags: metrics.Tags{
			"svc": "chainlink_data_stream_msg_h",
		},
	}
}

// decodedReportData holds the extracted data from a decoded Chainlink report.
type decodedReportData struct {
	feedIDStr             string
	price                 *big.Int
	validFromTimestamp    uint32
	observationsTimestamp uint32
}

var (
	reportContextType      abi.Type
	reportBytesType        abi.Type
	reportBytes32SliceType abi.Type
	reportBytes32Type      abi.Type
	reportUint32Type       abi.Type
	reportUint64Type       abi.Type
	reportUint192Type      abi.Type
	reportInt192Type       abi.Type

	fullReportArgs abi.Arguments
	v3ReportArgs   abi.Arguments
	v8ReportArgs   abi.Arguments
)

func init() {
	var err error

	reportContextType, err = abi.NewType("bytes32[3]", "bytes32[3]", nil)
	if err != nil {
		panic("failed to create reportContext ABI type: " + err.Error())
	}
	reportBytesType, err = abi.NewType("bytes", "bytes", nil)
	if err != nil {
		panic("failed to create bytes ABI type: " + err.Error())
	}
	reportBytes32SliceType, err = abi.NewType("bytes32[]", "bytes32[]", nil)
	if err != nil {
		panic("failed to create bytes32[] ABI type: " + err.Error())
	}
	reportBytes32Type, err = abi.NewType("bytes32", "bytes32", nil)
	if err != nil {
		panic("failed to create bytes32 ABI type: " + err.Error())
	}
	reportUint32Type, err = abi.NewType("uint32", "uint32", nil)
	if err != nil {
		panic("failed to create uint32 ABI type: " + err.Error())
	}
	reportUint64Type, err = abi.NewType("uint64", "uint64", nil)
	if err != nil {
		panic("failed to create uint64 ABI type: " + err.Error())
	}
	reportUint192Type, err = abi.NewType("uint192", "uint192", nil)
	if err != nil {
		panic("failed to create uint192 ABI type: " + err.Error())
	}
	reportInt192Type, err = abi.NewType("int192", "int192", nil)
	if err != nil {
		panic("failed to create int192 ABI type: " + err.Error())
	}

	fullReportArgs = abi.Arguments{
		{Name: "reportContext", Type: reportContextType},
		{Name: "reportData", Type: reportBytesType},
		{Name: "rawRs", Type: reportBytes32SliceType},
		{Name: "rawSs", Type: reportBytes32SliceType},
		{Name: "rawVs", Type: reportBytes32Type},
	}

	v3ReportArgs = abi.Arguments{
		{Name: "feedId", Type: reportBytes32Type},
		{Name: "validFromTimestamp", Type: reportUint32Type},
		{Name: "observationsTimestamp", Type: reportUint32Type},
		{Name: "nativeFee", Type: reportUint192Type},
		{Name: "linkFee", Type: reportUint192Type},
		{Name: "expiresAt", Type: reportUint32Type},
		{Name: "price", Type: reportInt192Type},
		{Name: "bid", Type: reportInt192Type},
		{Name: "ask", Type: reportInt192Type},
	}

	v8ReportArgs = abi.Arguments{
		{Name: "feedId", Type: reportBytes32Type},
		{Name: "validFromTimestamp", Type: reportUint32Type},
		{Name: "observationsTimestamp", Type: reportUint32Type},
		{Name: "nativeFee", Type: reportUint192Type},
		{Name: "linkFee", Type: reportUint192Type},
		{Name: "expiresAt", Type: reportUint32Type},
		{Name: "lastUpdateTimestamp", Type: reportUint64Type},
		{Name: "midPrice", Type: reportInt192Type},
		{Name: "marketStatus", Type: reportUint32Type},
	}
}

// RelayChainlinkPrices handles the MsgRelayChainlinkPrices message.
func (k ChainlinkDataStreamsMsgServer) RelayChainlinkPrices(c context.Context, msg *types.MsgRelayChainlinkPrices) (*types.MsgRelayChainlinkPricesResponse, error) {
	defer metrics.ReportFuncCallAndTiming(k.svcTags)()

	ctx := sdk.UnwrapSDKContext(c)

	if len(msg.Reports) == 0 {
		return &types.MsgRelayChainlinkPricesResponse{}, nil
	}

	var (
		processedCount int
		lastErr        error
	)

	// Process each report in the message
	for _, chainlinkReport := range msg.Reports {
		if err := k.processChainlinkReport(ctx, chainlinkReport); err != nil {
			lastErr = err
			continue
		}
		processedCount++
	}

	if processedCount == 0 && lastErr != nil {
		return nil, lastErr
	}

	return &types.MsgRelayChainlinkPricesResponse{}, nil
}

// decodeFullReportData extracts the report data bytes from a full report payload.
func decodeFullReportData(fullReport []byte) ([]byte, error) {
	decoded, err := fullReportArgs.Unpack(fullReport)
	if err != nil {
		return nil, fmt.Errorf("failed to decode full report: %w", err)
	}
	if len(decoded) < 2 {
		return nil, fmt.Errorf("unexpected full report output size")
	}
	reportData, ok := decoded[1].([]byte)
	if !ok {
		return nil, fmt.Errorf("unexpected report data type: %T", decoded[1])
	}
	if len(reportData) == 0 {
		return nil, fmt.Errorf("empty report data")
	}
	return reportData, nil
}

// decodeVerifiedReport decodes the verified report bytes into the fields we store.
func decodeVerifiedReport(feedID feed.ID, reportData []byte) (*decodedReportData, error) {
	feedVersion := feedID.Version()

	var (
		args       abi.Arguments
		priceIndex int
	)

	switch feedVersion {
	case feed.FeedVersion3:
		args = v3ReportArgs
		priceIndex = 6
	case feed.FeedVersion8:
		args = v8ReportArgs
		priceIndex = 7
	default:
		return nil, fmt.Errorf("unsupported Chainlink Data Stream schema version: %d", feedVersion)
	}

	decoded, err := args.Unpack(reportData)
	if err != nil {
		return nil, fmt.Errorf("failed to decode verified report: %w", err)
	}
	if len(decoded) <= priceIndex {
		return nil, fmt.Errorf("unexpected verified report output size")
	}

	feedIDBytes, err := bytes32ToBytes(decoded[0])
	if err != nil {
		return nil, err
	}
	validFromTimestamp, err := toUint32(decoded[1])
	if err != nil {
		return nil, err
	}
	observationsTimestamp, err := toUint32(decoded[2])
	if err != nil {
		return nil, err
	}
	price, err := toBigInt(decoded[priceIndex])
	if err != nil {
		return nil, err
	}

	var decodedFeedID feed.ID
	copy(decodedFeedID[:], feedIDBytes)

	return &decodedReportData{
		feedIDStr:             decodedFeedID.String(),
		price:                 price,
		validFromTimestamp:    validFromTimestamp,
		observationsTimestamp: observationsTimestamp,
	}, nil
}

func bytes32ToBytes(value any) ([]byte, error) {
	switch v := value.(type) {
	case [32]byte:
		b := make([]byte, 32)
		copy(b, v[:])
		return b, nil
	case []byte:
		if len(v) != 32 {
			return nil, fmt.Errorf("unexpected bytes32 length: %d", len(v))
		}
		b := make([]byte, 32)
		copy(b, v)
		return b, nil
	default:
		return nil, fmt.Errorf("unexpected bytes32 type: %T", value)
	}
}

func toUint32(value any) (uint32, error) {
	switch v := value.(type) {
	case uint32:
		return v, nil
	case uint64:
		if v > uint64(^uint32(0)) {
			return 0, fmt.Errorf("uint32 overflow: %d", v)
		}
		return uint32(v), nil
	case *big.Int:
		if v.Sign() < 0 || v.BitLen() > 32 {
			return 0, fmt.Errorf("invalid uint32 value: %s", v.String())
		}
		return uint32(v.Uint64()), nil
	default:
		return 0, fmt.Errorf("unexpected uint32 type: %T", value)
	}
}

func toBigInt(value any) (*big.Int, error) {
	switch v := value.(type) {
	case *big.Int:
		return v, nil
	case big.Int:
		return &v, nil
	case int64:
		return big.NewInt(v), nil
	case uint64:
		return new(big.Int).SetUint64(v), nil
	case uint32:
		return new(big.Int).SetUint64(uint64(v)), nil
	default:
		return nil, fmt.Errorf("unexpected integer type: %T", value)
	}
}

// processChainlinkReport processes a single Chainlink report.
func (k ChainlinkDataStreamsMsgServer) processChainlinkReport(ctx sdk.Context, chainlinkReport *types.ChainlinkReport) error {
	// Validate inputs
	if chainlinkReport == nil || len(chainlinkReport.FeedId) == 0 || len(chainlinkReport.FullReport) == 0 {
		return errors.Wrap(types.ErrInvalidOracleRequest, "empty chainlink report")
	}

	// Convert feed ID bytes to feed.ID type to determine version
	var feedID feed.ID
	if len(chainlinkReport.FeedId) != 32 {
		k.Logger(ctx).Error("invalid feed ID length", "expected", 32, "got", len(chainlinkReport.FeedId))
		return errors.Wrap(types.ErrInvalidOracleRequest, "invalid feed ID length")
	}
	copy(feedID[:], chainlinkReport.FeedId)

	params := k.GetParams(ctx)
	var err error
	var reportData []byte
	if params.AcceptUnverifiedChainlinkDataStreamsReports {
		reportData, err = decodeFullReportData(chainlinkReport.FullReport)
		if err != nil {
			k.Logger(ctx).Error("Chainlink report decode failed", "error", err)
			return errors.Wrap(types.ErrInvalidOracleRequest, err.Error())
		}
	} else {
		reportData, err = k.verifyChainlinkReport(ctx, chainlinkReport.FullReport)
		if err != nil {
			k.Logger(ctx).Error("Chainlink report verification failed", "error", err)
			return err
		}
	}

	// Decode the verified report based on version
	decoded, err := decodeVerifiedReport(feedID, reportData)
	if err != nil {
		k.Logger(ctx).Error("Chainlink report decode failed", "error", err)
		return errors.Wrap(types.ErrInvalidOracleRequest, err.Error())
	}

	expectedFeedID := feedID.String()
	if decoded.feedIDStr != expectedFeedID {
		k.Logger(ctx).Error("Chainlink report feed ID mismatch", "expected", expectedFeedID, "got", decoded.feedIDStr)
		return errors.Wrap(types.ErrInvalidOracleRequest, "feed ID mismatch")
	}

	if decoded.price == nil {
		k.Logger(ctx).Error("price is nil in decoded report")
		return errors.Wrap(types.ErrInvalidOracleRequest, "price is nil in decoded report")
	}

	// Convert big.Int price to math.LegacyDec
	// Chainlink prices are expressed as integers with 18 decimal places
	priceDecimal := math.LegacyNewDecFromBigIntWithPrec(decoded.price, 18)

	// Convert the raw price to math.Int for storage
	reportPriceInt := math.NewIntFromBigInt(decoded.price)

	// Process the report
	k.ProcessChainlinkDataStreamsReport(
		ctx,
		decoded.feedIDStr,
		reportPriceInt,
		uint64(decoded.validFromTimestamp),
		uint64(decoded.observationsTimestamp),
		priceDecimal,
	)

	return nil
}
