package types

import (
	"cosmossdk.io/math"
)

// NewChainlinkDataStreamsPriceState creates a new ChainlinkDataStreamsPriceState instance.
func NewChainlinkDataStreamsPriceState(
	feedID string,
	reportPrice math.Int,
	validFromTimestamp uint64,
	observationsTimestamp uint64,
	price math.LegacyDec,
	blockTime int64,
) *ChainlinkDataStreamsPriceState {
	return &ChainlinkDataStreamsPriceState{
		FeedId:                feedID,
		ReportPrice:           reportPrice,
		ValidFromTimestamp:    validFromTimestamp,
		ObservationsTimestamp: observationsTimestamp,
		PriceState:            *NewPriceState(price, blockTime),
	}
}

// Update updates the ChainlinkDataStreamsPriceState with new values.
func (c *ChainlinkDataStreamsPriceState) Update(
	reportPrice math.Int,
	validFromTimestamp uint64,
	observationsTimestamp uint64,
	price math.LegacyDec,
	blockTime int64,
) {
	c.ReportPrice = reportPrice
	c.ValidFromTimestamp = validFromTimestamp
	c.ObservationsTimestamp = observationsTimestamp
	c.PriceState.UpdatePrice(price, blockTime)
}
