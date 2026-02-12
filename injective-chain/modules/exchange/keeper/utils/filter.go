package utils //nolint:revive // meaningless linter suggestions

import (
	"github.com/ethereum/go-ethereum/common"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types/v2"
)

type FilterFn[T any] func(T) bool

func ChainFilters[T any](filters ...FilterFn[T]) FilterFn[T] {
	return func(t T) bool {
		for _, filter := range filters {
			if !filter(t) {
				return false
			}
		}

		return true
	}
}

func MarketStatusFilter(status ...v2.MarketStatus) FilterFn[v2.MarketI] {
	m := make(map[v2.MarketStatus]struct{}, len(status))
	for _, s := range status {
		m[s] = struct{}{}
	}

	return func(market v2.MarketI) bool {
		_, found := m[market.GetMarketStatus()]
		return found
	}
}

func MarketIDFilter(ids ...string) FilterFn[v2.MarketI] {
	m := make(map[common.Hash]struct{}, len(ids))
	for _, id := range ids {
		m[common.HexToHash(id)] = struct{}{}
	}

	return func(market v2.MarketI) bool {
		_, found := m[market.MarketID()]
		return found
	}
}

func AllMarketsFilter() FilterFn[v2.MarketI] {
	return func(_ v2.MarketI) bool {
		return true
	}
}
