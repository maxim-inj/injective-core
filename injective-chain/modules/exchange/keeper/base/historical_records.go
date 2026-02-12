package base

import (
	"cosmossdk.io/store/prefix"
	"github.com/InjectiveLabs/metrics"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types/v2"
)

func (k *BaseKeeper) GetAllHistoricalTradeRecords(ctx sdk.Context) []*v2.TradeRecords {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	allTradeRecords := make([]*v2.TradeRecords, 0)
	store := ctx.KVStore(k.storeKey)
	historicalTradeRecordsStore := prefix.NewStore(store, types.MarketHistoricalTradeRecordsPrefix)

	iterateSafe(historicalTradeRecordsStore.Iterator(nil, nil), func(_, v []byte) bool {
		var tradeRecords v2.TradeRecords
		k.cdc.MustUnmarshal(v, &tradeRecords)

		allTradeRecords = append(allTradeRecords, &tradeRecords)
		return false
	})

	return allTradeRecords
}

func (k *BaseKeeper) SetHistoricalTradeRecords(ctx sdk.Context, marketID common.Hash, entry *v2.TradeRecords) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)

	bz := k.cdc.MustMarshal(entry)
	store.Set(types.GetMarketHistoricalTradeRecordsKey(marketID), bz)
}

// GetHistoricalTradeRecords returns the historical trade records for a market starting from the `from` time.
func (k *BaseKeeper) GetHistoricalTradeRecords(ctx sdk.Context, marketID common.Hash, from int64) (entry *v2.TradeRecords, omitted bool) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	entry = &v2.TradeRecords{MarketId: marketID.Hex()}

	store := k.getStore(ctx)
	bz := store.Get(types.GetMarketHistoricalTradeRecordsKey(marketID))
	if bz == nil {
		return entry, false
	}

	var tradeEntry v2.TradeRecords
	k.cdc.MustUnmarshal(bz, &tradeEntry)

	entry.LatestTradeRecords, omitted = filterHistoricalTradeRecords(tradeEntry.LatestTradeRecords, from)

	return entry, omitted
}

func filterHistoricalTradeRecords(records []*v2.TradeRecord, from int64) (filteredRecords []*v2.TradeRecord, omitted bool) {
	offsetIdx := -1

	for idx, tradeRecord := range records {
		if tradeRecord.Timestamp < from {
			omitted = true
			continue
		}

		offsetIdx = idx
		break
	}

	if offsetIdx < 0 {
		return nil, omitted
	}

	return records[offsetIdx:], omitted
}
