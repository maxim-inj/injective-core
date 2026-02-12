package v1dot18dot0

import (
	"cosmossdk.io/log"
	"cosmossdk.io/store/prefix"
	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/InjectiveLabs/injective-core/injective-chain/app/upgrades"
	oracletypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/types"
)

const UpgradeName = "v1.18.0-beta"

func StoreUpgrades() storetypes.StoreUpgrades {
	return storetypes.StoreUpgrades{}
}

func UpgradeSteps() []*upgrades.UpgradeHandlerStep {
	return []*upgrades.UpgradeHandlerStep{
		upgrades.NewUpgradeHandlerStep(
			"CLEAN UP BAND ORACLE",
			UpgradeName,
			upgrades.MainnetChainID,
			CleanupBandOracle,
		),
		upgrades.NewUpgradeHandlerStep(
			"CLEAN UP BAND ORACLE",
			UpgradeName,
			upgrades.TestnetChainID,
			CleanupBandOracle,
		),
		upgrades.NewUpgradeHandlerStep(
			"CLEAN UP BAND ORACLE",
			UpgradeName,
			upgrades.DevnetChainID,
			CleanupBandOracle,
		),
		upgrades.NewUpgradeHandlerStep(
			"SET CHAINLINK ORACLE PARAMS",
			UpgradeName,
			upgrades.MainnetChainID,
			SetChainlinkOracleParams,
		),
		upgrades.NewUpgradeHandlerStep(
			"SET CHAINLINK ORACLE PARAMS",
			UpgradeName,
			upgrades.TestnetChainID,
			SetChainlinkOracleParams,
		),
	}
}

// SetChainlinkOracleParams sets the Chainlink Data Streams oracle parameters based on the chain ID.
func SetChainlinkOracleParams(ctx sdk.Context, app upgrades.InjectiveApplication, logger log.Logger) error {
	oracleKeeper := app.GetOracleKeeper()
	params := oracleKeeper.GetParams(ctx)

	switch ctx.ChainID() {
	case upgrades.MainnetChainID:
		params.ChainlinkVerifierProxyContract = "0x60fAa7faC949aF392DFc858F5d97E3EEfa07E9EB"
		params.AcceptUnverifiedChainlinkDataStreamsReports = false
	case upgrades.TestnetChainID:
		params.ChainlinkVerifierProxyContract = ""
		params.AcceptUnverifiedChainlinkDataStreamsReports = true
	}
	params.ChainlinkDataStreamsVerificationGasLimit = 500_000

	oracleKeeper.SetParams(ctx, params)

	logger.Info("Chainlink oracle params set",
		"chain_id", ctx.ChainID(),
		"chainlink_verifier_proxy_contract", params.ChainlinkVerifierProxyContract,
		"accept_unverified_reports", params.AcceptUnverifiedChainlinkDataStreamsReports,
		"verification_gas_limit", params.ChainlinkDataStreamsVerificationGasLimit,
	)

	return nil
}

// CleanupBandOracle deletes all BandIBCCallDataRecord entries and set Band IBC flag to false
func CleanupBandOracle(ctx sdk.Context, app upgrades.InjectiveApplication, _ log.Logger) error {
	k := app.GetOracleKeeper()

	// remove old records
	for _, record := range k.GetAllBandCalldataRecords(ctx) {
		k.DeleteBandIBCCallDataRecord(ctx, record.ClientId)
	}

	// remove requests
	for _, req := range k.GetAllBandIBCOracleRequests(ctx) {
		k.DeleteBandIBCOracleRequest(ctx, req.RequestId)
	}

	// remove price states (no keeper method for this one)

	var (
		keys              = make([][]byte, 0)
		store             = ctx.KVStore(app.GetKey(oracletypes.StoreKey))
		bandIBCPriceStore = prefix.NewStore(store, oracletypes.BandIBCPriceKey)
	)

	func() {
		iterator := bandIBCPriceStore.Iterator(nil, nil)
		defer iterator.Close()

		for ; iterator.Valid(); iterator.Next() {
			keys = append(keys, iterator.Key())
		}
	}()

	for _, key := range keys {
		bandIBCPriceStore.Delete(key)
	}

	// overwrite band ibc params to default
	var emptyParams oracletypes.BandIBCParams //nolint:staticcheck // ok
	k.SetBandIBCParams(ctx, emptyParams)

	k.SetBandIBCLatestClientID(ctx, 0)
	k.SetBandIBCLatestRequestID(ctx, 0)

	return nil
}
