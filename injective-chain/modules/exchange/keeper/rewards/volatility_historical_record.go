package rewards

import (
	"sort"

	"cosmossdk.io/math"
	"github.com/InjectiveLabs/metrics"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types/v2"
	oracletypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/types"
)

// GetMarketVolatility returns the volatility based on trades in specific market. Returns nil for invalid volatility.
//
//nolint:revive // ok
func (k TradingKeeper) GetMarketVolatility(
	ctx sdk.Context,
	marketID common.Hash,
	options *v2.TradeHistoryOptions,
) (
	vol *math.LegacyDec,
	rawTrades []*v2.TradeRecord,
	meta *oracletypes.MetadataStatistics,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	maxAge, groupingSec, includeRawHistory, includeMetadata := k.getHistoricalTradeRecordsSearchParams(ctx, options)

	tradeRecords, _ := k.GetHistoricalTradeRecords(ctx, marketID, maxAge)
	trades := tradeRecords.LatestTradeRecords

	if len(trades) == 0 {
		return
	}

	tradesGrouped := GetRecordsGroupedBy(trades, groupingSec)

	vol = GetStandardDeviationForTradeRecords(tradesGrouped)

	if includeRawHistory {
		rawTrades = trades
	}
	if includeMetadata {
		meta = calculateStatistics(trades, tradesGrouped)
	}

	return
}

//revive:disable:function-result-limit // we need to return 4 values
func (TradingKeeper) getHistoricalTradeRecordsSearchParams(
	ctx sdk.Context,
	options *v2.TradeHistoryOptions,
) (maxAge, groupingSec int64, includeRawHistory, includeMetadata bool) {
	maxAge = int64(0)
	groupingSec = int64(GROUPING_SECONDS_DEFAULT)
	includeRawHistory = false
	includeMetadata = false

	if options != nil {
		includeRawHistory = options.IncludeRawHistory
		includeMetadata = options.IncludeMetadata
		if options.MaxAge > 0 {
			maxAge = ctx.BlockTime().Unix() - int64(options.MaxAge)
		}
		if options.TradeGroupingSec > 0 {
			groupingSec = int64(options.TradeGroupingSec)
		}
	}
	return maxAge, groupingSec, includeRawHistory, includeMetadata
}

func (k TradingKeeper) AppendTradeRecord(ctx sdk.Context, marketID common.Hash, tradeRecord *v2.TradeRecord) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	existingOrEmptyRecord, _ := k.GetHistoricalTradeRecords(ctx, marketID, tradeRecord.Timestamp-types.MaxHistoricalTradeRecordAge)
	existingOrEmptyRecord.LatestTradeRecords = append(existingOrEmptyRecord.LatestTradeRecords, tradeRecord)

	k.SetHistoricalTradeRecords(ctx, marketID, existingOrEmptyRecord)
}

func (k TradingKeeper) CleanupHistoricalTradeRecords(ctx sdk.Context) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	before := ctx.BlockTime().Unix() - types.MaxHistoricalTradeRecordAge
	onlyEnabled := true

	k.IterateSpotMarkets(ctx, &onlyEnabled, func(m *v2.SpotMarket) (stop bool) {
		k.cleanupMarketHistoricalTradeRecords(ctx, m.MarketID(), before)
		return false
	})

	k.IterateDerivativeMarkets(ctx, &onlyEnabled, func(m *v2.DerivativeMarket) (stop bool) {
		k.cleanupMarketHistoricalTradeRecords(ctx, m.MarketID(), before)
		return false
	})
}

func (k TradingKeeper) cleanupMarketHistoricalTradeRecords(ctx sdk.Context, marketID common.Hash, before int64) {
	needsSave := false
	existingOrEmptyRecord, omitted := k.GetHistoricalTradeRecords(ctx, marketID, before)

	if len(existingOrEmptyRecord.LatestTradeRecords) == 0 {
		if omitted {
			// some records older than 'before' have been omitted, need to overwrite with empty entry
			needsSave = true
		} else {
			// empty records - no need to cleanup the entry
			return
		}
	} else if omitted {
		// non-empty records and something has been omitted, need to save new entry
		needsSave = true
	}

	if needsSave {
		k.SetHistoricalTradeRecords(ctx, marketID, existingOrEmptyRecord)
	}
}

func (k TradingKeeper) PersistVwapInfo(ctx sdk.Context, spotVwapInfo *v2.SpotVwapInfo, derivativeVwapInfo *v2.DerivativeVwapInfo) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	blockTime := ctx.BlockTime()

	if spotVwapInfo != nil {
		spotMarketIDs := spotVwapInfo.GetSortedSpotMarketIDs()
		for _, spotMarketID := range spotMarketIDs {
			k.AppendTradeRecord(ctx, spotMarketID, &v2.TradeRecord{
				Timestamp: blockTime.Unix(),
				Price:     (*spotVwapInfo)[spotMarketID].Price,
				Quantity:  (*spotVwapInfo)[spotMarketID].Quantity,
			})
		}
	}

	if derivativeVwapInfo != nil {
		perpetualMarketIDs := derivativeVwapInfo.GetSortedPerpetualMarketIDs()
		for _, perpetualMarketID := range perpetualMarketIDs {
			k.AppendTradeRecord(ctx, perpetualMarketID, &v2.TradeRecord{
				Timestamp: blockTime.Unix(),
				Price:     derivativeVwapInfo.PerpetualVwapInfo[perpetualMarketID].VwapData.Price,
				Quantity:  derivativeVwapInfo.PerpetualVwapInfo[perpetualMarketID].VwapData.Quantity,
			})
		}

		expiryMarketIDs := derivativeVwapInfo.GetSortedExpiryFutureMarketIDs()
		for _, expiryMarketID := range expiryMarketIDs {
			k.AppendTradeRecord(ctx, expiryMarketID, &v2.TradeRecord{
				Timestamp: blockTime.Unix(),
				Price:     derivativeVwapInfo.ExpiryVwapInfo[expiryMarketID].VwapData.Price,
				Quantity:  derivativeVwapInfo.ExpiryVwapInfo[expiryMarketID].VwapData.Quantity,
			})
		}

		binaryOptionsMarketIDs := derivativeVwapInfo.GetSortedBinaryOptionsMarketIDs()
		for _, binaryOptionMarketID := range binaryOptionsMarketIDs {
			k.AppendTradeRecord(ctx, binaryOptionMarketID, &v2.TradeRecord{
				Timestamp: blockTime.Unix(),
				Price:     derivativeVwapInfo.BinaryOptionsVwapInfo[binaryOptionMarketID].VwapData.Price,
				Quantity:  derivativeVwapInfo.BinaryOptionsVwapInfo[binaryOptionMarketID].VwapData.Quantity,
			})
		}
	}
}

// calculateStatistics returns statistics metadata over given trade records and grouped trade records.
// Mean is VWAP over grouped trade records, Twap is calculated over the grouped prices.
func calculateStatistics(tradeRecords, groupedTradeRecords []*v2.TradeRecord) *oracletypes.MetadataStatistics {
	var (
		sum     = math.LegacyZeroDec()
		qSum    = math.LegacyZeroDec()
		twapSum = math.LegacyZeroDec()
		count   = uint32(len(tradeRecords))
	)

	if count == 0 {
		return nil
	}

	for i, r := range groupedTradeRecords {
		sum = sum.Add(r.Price.Mul(r.Quantity))
		qSum = qSum.Add(r.Quantity)
		if i > 0 {
			// twapSum += p * ∆t
			twapSum = twapSum.Add(r.Price.Mul(math.LegacyNewDec(r.Timestamp - groupedTradeRecords[i-1].Timestamp)))
		}
	}

	// compute median on copy so the slice sorting doesn't mess up the indexes above
	recordsCopy := make([]*v2.TradeRecord, 0, count)
	recordsCopy = append(recordsCopy, tradeRecords...)
	sort.SliceStable(recordsCopy, func(i, j int) bool {
		return recordsCopy[i].Price.LT(recordsCopy[j].Price)
	})

	median := recordsCopy[count/2].Price
	if count%2 == 0 {
		median = median.Add(recordsCopy[count/2-1].Price).Quo(math.LegacyNewDec(2))
	}

	meta := &oracletypes.MetadataStatistics{
		Mean:              sum.Quo(qSum),
		MinPrice:          recordsCopy[0].Price,
		MaxPrice:          recordsCopy[count-1].Price,
		MedianPrice:       median,
		FirstTimestamp:    tradeRecords[0].Timestamp,
		LastTimestamp:     tradeRecords[count-1].Timestamp,
		GroupCount:        uint32(len(groupedTradeRecords)),
		RecordsSampleSize: count,
		Twap:              math.LegacyZeroDec(),
	}
	if count > 1 {
		meta.Twap = twapSum.Quo(math.LegacyNewDec(meta.LastTimestamp - meta.FirstTimestamp))
	}

	return meta
}

func GetRecordsGroupedBy(tradeRecords []*v2.TradeRecord, seconds int64) []*v2.TradeRecord {
	groupedTradeRecords := make([]*v2.TradeRecord, 0)

	for _, tradeRecord := range tradeRecords {
		// Don't use midPrice, it's manipulable.
		//
		// latestGroupTimestamp := from.Add(time.Duration(len(groupedTradeRecords)+1) * seconds)
		// for tradeRecord.Timestamp.After(latestGroupTimestamp) {
		// 	groupedTradeRecords = append(groupedTradeRecords, &v2.TradeRecord{
		// 		Timestamp: latestGroupTimestamp,
		// 		Price:     midPrice,
		// 		Quantity:  math.LegacyZeroDec(),
		// 	})
		// 	latestGroupTimestamp = latestGroupTimestamp.Add(seconds)
		// }

		if len(groupedTradeRecords) == 0 {
			groupedTradeRecords = append(groupedTradeRecords, tradeRecord)
			continue
		}

		lastTradeRecord := groupedTradeRecords[len(groupedTradeRecords)-1]
		if tradeRecord.Timestamp-lastTradeRecord.Timestamp < seconds {
			groupedQuantity := lastTradeRecord.Quantity.Add(tradeRecord.Quantity)
			groupedTradeRecords[len(groupedTradeRecords)-1] = &v2.TradeRecord{
				Timestamp: lastTradeRecord.Timestamp,
				// nolint:all
				// price = (p0 * q0 + p1 * q1) / (q0 + q1)
				Price:    lastTradeRecord.Price.Mul(lastTradeRecord.Quantity).Add(tradeRecord.Price.Mul(tradeRecord.Quantity)).Quo(groupedQuantity),
				Quantity: groupedQuantity,
			}
		} else {
			groupedTradeRecords = append(groupedTradeRecords, tradeRecord)
		}
	}

	return groupedTradeRecords
}

// GetStandardDeviationForTradeRecords returns the volume-weighted arithmetic mean for the trade records.
func GetStandardDeviationForTradeRecords(tradeRecords []*v2.TradeRecord) *math.LegacyDec {
	if len(tradeRecords) == 1 {
		standardDeviationValue := math.LegacyZeroDec()
		return &standardDeviationValue
	}

	// x̄ = ∑(p * q) / ∑q
	mean := GetMeanForTradeRecords(tradeRecords)

	scaledSum, aggregateQuantity := math.LegacyZeroDec(), math.LegacyZeroDec()

	for _, tradeRecord := range tradeRecords {
		scaledDeviation := tradeRecord.Price.Sub(mean)
		scaledSum = scaledSum.Add(tradeRecord.Quantity.Mul(scaledDeviation.Mul(scaledDeviation)))
		aggregateQuantity = aggregateQuantity.Add(tradeRecord.Quantity)
	}
	// x̄ = ∑(p * q) / ∑q

	// σ² = ∑(q * (p - x̄)²) / ∑q
	variance := scaledSum.Quo(aggregateQuantity)
	// σ = √σ²
	scaledStandardDeviationValue, err := variance.ApproxSqrt()
	if err != nil {
		return nil
	}

	return &scaledStandardDeviationValue
}

// GetMeanForTradeRecords returns the volume-weighted arithmetic mean for the trade records.
// x̄ = ∑(p * q) / ∑q
func GetMeanForTradeRecords(tradeRecords []*v2.TradeRecord) (mean math.LegacyDec) {
	if len(tradeRecords) == 0 {
		return math.LegacyZeroDec()
	}

	sum, aggregateQuantity := math.LegacyZeroDec(), math.LegacyZeroDec()
	for _, tradeRecord := range tradeRecords {
		sum = sum.Add(tradeRecord.Price.Mul(tradeRecord.Quantity))
		aggregateQuantity = aggregateQuantity.Add(tradeRecord.Quantity)
	}

	return sum.Quo(aggregateQuantity)
}
