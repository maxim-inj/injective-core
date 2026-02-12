package helpers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"testing"
	"time"

	"cosmossdk.io/math"
	exchangev2 "github.com/InjectiveLabs/sdk-go/chain/exchange/types/v2"
	insurancetypes "github.com/InjectiveLabs/sdk-go/chain/insurance/types"
	oracletypes "github.com/InjectiveLabs/sdk-go/chain/oracle/types"
	tftypes "github.com/InjectiveLabs/sdk-go/chain/tokenfactory/types"
	abciv1 "github.com/cometbft/cometbft/api/cometbft/abci/v1"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	govv1beta1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	proposalStatusTimeout   = 45 * time.Second
	proposalStatusPollEvery = 500 * time.Millisecond
)

type ExchangeSetupSuite struct {
	MarketID string
}

type Market struct {
	MarketID string `json:"market_id"`
}

type Response struct {
	Markets []Market `json:"markets"`
}

func extractMarketID(stdout []byte) (string, error) {
	var resp Response
	if err := json.Unmarshal(stdout, &resp); err != nil {
		return "", fmt.Errorf("error unmarshaling JSON: %w", err)
	}
	if len(resp.Markets) == 0 {
		return "", fmt.Errorf("no markets found")
	}
	return resp.Markets[0].MarketID, nil
}

// SetupMarketsAndUsers sets up both spot and derivative markets with user wallets funded with quote denom.
// Returns the launched derivative and spot markets.
func SetupMarketsAndUsers(
	t *testing.T,
	ctx context.Context,
	chain *cosmos.CosmosChain,
	marketLauncher ibc.Wallet,
	userWallets []ibc.Wallet,
) (*exchangev2.FullDerivativeMarket, *exchangev2.SpotMarket) {
	t.Helper()

	quoteDenom := fmt.Sprintf("factory/%s/usdt", marketLauncher.FormattedAddress())
	proposalInitialDeposit := math.NewIntWithDecimal(1000, 18)
	depositCoin := sdk.NewCoin(chain.Config().Denom, proposalInitialDeposit)

	// *** Batch 1: Denom, Mints, Setup Proposals ***
	prepareAndBroadcastSetupBatch(t, ctx, chain, marketLauncher, userWallets, quoteDenom, depositCoin)

	// *** Batch 2: Relay, Insurance, Spot Launch, Perp Launch ***
	prepareAndBroadcastLaunchBatch(t, ctx, chain, marketLauncher, quoteDenom, depositCoin)

	// *** Query Markets ***
	// Create gRPC connection and query client
	conn, err := grpc.NewClient(chain.GetHostGRPCAddress(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()

	queryClient := exchangev2.NewQueryClient(conn)

	// Spot Market (from MsgInstantSpotMarketLaunch - available immediately after tx)
	spotResp, err := QueryRPC(ctx, queryClient.SpotMarkets, &exchangev2.QuerySpotMarketsRequest{})
	require.NoError(t, err, "error querying spot markets")
	require.NotEmpty(t, spotResp.Markets)

	// Derivative Market (from Proposal - available after pass)
	derivResp, err := QueryRPC(ctx, queryClient.DerivativeMarkets, &exchangev2.QueryDerivativeMarketsRequest{})
	require.NoError(t, err, "error querying derivative markets")
	require.NotEmpty(t, derivResp.Markets)

	return derivResp.Markets[0], spotResp.Markets[0]
}

// QueryExchangeParams queries exchange params using gRPC
func QueryExchangeParams(ctx context.Context, chain *cosmos.CosmosChain) (*exchangev2.Params, error) {
	conn, err := grpc.NewClient(chain.GetHostGRPCAddress(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	queryClient := exchangev2.NewQueryClient(conn)

	res, err := QueryRPC(ctx, queryClient.QueryExchangeParams, &exchangev2.QueryExchangeParamsRequest{})
	if err != nil {
		return nil, err
	}

	return &res.Params, nil
}

func prepareAndBroadcastSetupBatch(
	t *testing.T,
	ctx context.Context,
	chain *cosmos.CosmosChain,
	marketLauncher ibc.Wallet,
	userWallets []ibc.Wallet,
	quoteDenom string,
	depositCoin sdk.Coin,
) {
	t.Helper()

	txOpts := []cosmos.FactoryOpt{WithGas(2_000_000)}

	msgs := []sdk.Msg{}

	// 1. Create Quote Denom
	createDenom := &tftypes.MsgCreateDenom{
		Sender:         marketLauncher.FormattedAddress(),
		Subdenom:       "usdt",
		Name:           "Tether USD",
		Symbol:         "USDT",
		Decimals:       6,
		AllowAdminBurn: false,
	}
	msgs = append(msgs, createDenom)

	// 2. Mint to Launcher
	mintLauncher := &tftypes.MsgMint{
		Sender:   marketLauncher.FormattedAddress(),
		Amount:   sdk.NewCoin(quoteDenom, math.NewInt(1000000000000000000)),
		Receiver: marketLauncher.FormattedAddress(),
	}
	msgs = append(msgs, mintLauncher)

	// 3. Mint to all user wallets
	for _, wallet := range userWallets {
		mintUser := &tftypes.MsgMint{
			Sender:   marketLauncher.FormattedAddress(),
			Amount:   sdk.NewCoin(quoteDenom, math.NewInt(1000000000000000000)),
			Receiver: wallet.FormattedAddress(),
		}
		msgs = append(msgs, mintUser)
	}

	// 4. Proposal: Update Denom Min Notional
	propMinNotional := &exchangev2.BatchExchangeModificationProposal{
		Title:       "Update Denom Min Notional",
		Description: "Set min notional for USDT",
		DenomMinNotionalProposal: &exchangev2.DenomMinNotionalProposal{
			Title:       ".",
			Description: ".",
			DenomMinNotionals: []*exchangev2.DenomMinNotional{
				{
					Denom:       quoteDenom,
					MinNotional: math.LegacyNewDec(1),
				},
			},
		},
	}
	msgPropMinNotional := &exchangev2.MsgBatchExchangeModification{
		Sender:   authtypes.NewModuleAddress(govtypes.ModuleName).String(),
		Proposal: propMinNotional,
	}
	propMinNotionalEncoded, err := codectypes.NewAnyWithValue(msgPropMinNotional)
	require.NoError(t, err)

	submitPropMinNotional := &govv1.MsgSubmitProposal{
		InitialDeposit: []sdk.Coin{depositCoin},
		Proposer:       marketLauncher.FormattedAddress(),
		Title:          "Update Denom Min Notional",
		Summary:        "Set min notional for USDT",
		Messages:       []*codectypes.Any{propMinNotionalEncoded},
	}
	msgs = append(msgs, submitPropMinNotional)

	// 5. Proposal: Grant Price Feeder Privilege
	propPriceFeeder := &oracletypes.GrantPriceFeederPrivilegeProposal{
		Title:       "Market launcher can relay price",
		Description: "Market launcher can relay price",
		Base:        "oracle_base_inj",
		Quote:       "oracle_quote_usdt",
		Relayers:    []string{marketLauncher.FormattedAddress()},
	}
	submitPropPriceFeeder := &govv1beta1.MsgSubmitProposal{
		InitialDeposit: sdk.NewCoins(depositCoin),
		Proposer:       marketLauncher.FormattedAddress(),
	}
	require.NoError(t, submitPropPriceFeeder.SetContent(propPriceFeeder))
	msgs = append(msgs, submitPropPriceFeeder)

	// Broadcast Batch 1
	txResp := BroadcastTxBlock(t, ctx, chain, marketLauncher, txOpts, msgs...)
	require.Equal(t, uint32(0), txResp.Code, "failed setup batch: %s", txResp.RawLog)

	// Vote on Batch 1 Proposals
	propIDs, err := getProposalIDs(txResp.Events)
	require.NoError(t, err)
	require.Len(t, propIDs, 2, "expected 2 proposals in setup batch")

	for _, id := range propIDs {
		txHashes, err := VoteOnProposalAllValidatorsRPC(t, ctx, chain, id, govv1.VoteOption_VOTE_OPTION_YES)
		require.NoError(t, err)
		require.NotEmpty(t, txHashes, "expected at least one vote tx hash")

		// wait for the last vote tx to be included in a block
		// this is patch required because the VoteOnProposalAllValidatorsRPC function create a new tx factory every time
		// the proper solution would be to create a new struct representing the votes broadcaster, that creates the factory once
		// but this is the only logic so far sending more than one gov proposal in a TX, there is not need to create the broadcaster yet
		lastTxHash := txHashes[len(txHashes)-1]
		_, err = getTxResponseRPC(ctx, chain.Nodes()[0], lastTxHash)
		require.NoError(t, err, "failed waiting for vote tx %s", lastTxHash)
	}

	// Wait for pass
	for _, id := range propIDs {
		_, err = WaitForProposalStatusByTime(ctx, chain, id, govv1beta1.StatusPassed, proposalStatusTimeout, proposalStatusPollEvery)
		require.NoError(t, err)
	}
}

func prepareAndBroadcastLaunchBatch(
	t *testing.T,
	ctx context.Context,
	chain *cosmos.CosmosChain,
	marketLauncher ibc.Wallet,
	quoteDenom string,
	depositCoin sdk.Coin,
) {
	t.Helper()

	txOpts := []cosmos.FactoryOpt{WithGas(2_000_000)}

	msgs := []sdk.Msg{}

	// 1. Relay Price
	relayPrice := &oracletypes.MsgRelayPriceFeedPrice{
		Sender: marketLauncher.FormattedAddress(),
		Base:   []string{"oracle_base_inj"},
		Quote:  []string{"oracle_quote_usdt"},
		Price:  []math.LegacyDec{math.LegacyMustNewDecFromStr("10.00")},
	}
	msgs = append(msgs, relayPrice)

	// 2. Create Insurance Fund
	createInsurance := &insurancetypes.MsgCreateInsuranceFund{
		Sender:         marketLauncher.FormattedAddress(),
		Ticker:         "inj / usdt",
		QuoteDenom:     quoteDenom,
		OracleBase:     "oracle_base_inj",
		OracleQuote:    "oracle_quote_usdt",
		OracleType:     oracletypes.OracleType_PriceFeed,
		Expiry:         -1,
		InitialDeposit: sdk.NewCoin(quoteDenom, math.NewInt(1000000000)),
	}
	msgs = append(msgs, createInsurance)

	// 3. Instant Spot Market Launch
	spotLaunch := &exchangev2.MsgInstantSpotMarketLaunch{
		Sender:              marketLauncher.FormattedAddress(),
		Ticker:              "INJ / USDT",
		BaseDenom:           "inj",
		QuoteDenom:          quoteDenom,
		MinPriceTickSize:    math.LegacyNewDecWithPrec(1, 4),
		MinQuantityTickSize: math.LegacyNewDecWithPrec(1, 4),
		MinNotional:         math.LegacyNewDec(1),
		BaseDecimals:        18,
		QuoteDecimals:       6,
	}
	msgs = append(msgs, spotLaunch)

	// 4. Proposal: Launch Perp Market
	propPerpLaunch := &exchangev2.PerpetualMarketLaunchProposal{
		Title:                  "Launch a perp market",
		Description:            "Launch a pert market",
		Ticker:                 "inj / usdt",
		QuoteDenom:             quoteDenom,
		OracleBase:             "oracle_base_inj",
		OracleQuote:            "oracle_quote_usdt",
		OracleScaleFactor:      0,
		OracleType:             oracletypes.OracleType_PriceFeed,
		InitialMarginRatio:     math.LegacyNewDecWithPrec(5, 2),
		MaintenanceMarginRatio: math.LegacyNewDecWithPrec(2, 2),
		ReduceMarginRatio:      math.LegacyNewDecWithPrec(8, 2),
		MakerFeeRate:           math.LegacyNewDecWithPrec(1, 3),
		TakerFeeRate:           math.LegacyNewDecWithPrec(3, 3),
		MinPriceTickSize:       math.LegacyNewDecWithPrec(1, 4),
		MinQuantityTickSize:    math.LegacyNewDecWithPrec(1, 4),
		MinNotional:            math.LegacyOneDec(),
		OpenNotionalCap: exchangev2.OpenNotionalCap{
			Cap: &exchangev2.OpenNotionalCap_Uncapped{
				Uncapped: &exchangev2.OpenNotionalCapUncapped{},
			},
		},
		AdminInfo: &exchangev2.AdminInfo{
			Admin:            marketLauncher.FormattedAddress(),
			AdminPermissions: 63, // max perms
		},
	}
	submitPropPerpLaunch := &govv1beta1.MsgSubmitProposal{
		InitialDeposit: sdk.NewCoins(depositCoin),
		Proposer:       marketLauncher.FormattedAddress(),
	}
	require.NoError(t, submitPropPerpLaunch.SetContent(propPerpLaunch))
	msgs = append(msgs, submitPropPerpLaunch)

	// Broadcast Batch 2
	txResp := BroadcastTxBlock(t, ctx, chain, marketLauncher, txOpts, msgs...)
	require.Equal(t, uint32(0), txResp.Code, "failed launch batch: %s", txResp.RawLog)

	// Vote on Batch 2 Proposal
	propIDs, err := getProposalIDs(txResp.Events)
	require.NoError(t, err)
	require.Len(t, propIDs, 1, "expected 1 proposal in launch batch")

	txHashes, err := VoteOnProposalAllValidatorsRPC(t, ctx, chain, propIDs[0], govv1.VoteOption_VOTE_OPTION_YES)
	require.NoError(t, err)
	require.NotEmpty(t, txHashes, "expected at least one vote tx hash")

	lastTxHash := txHashes[len(txHashes)-1]
	_, err = getTxResponseRPC(ctx, chain.Nodes()[0], lastTxHash)
	require.NoError(t, err, "failed waiting for vote tx %s", lastTxHash)

	// Wait for pass
	_, err = WaitForProposalStatusByTime(ctx, chain, propIDs[0], govv1beta1.StatusPassed, proposalStatusTimeout, proposalStatusPollEvery)
	require.NoError(t, err)
}

func getProposalIDs(events []abciv1.Event) ([]uint64, error) {
	var ids []uint64
	for _, event := range events {
		if event.Type != "submit_proposal" {
			continue
		}
		for _, attr := range event.Attributes {
			keyStr := attr.Key
			valStr := attr.Value

			// tendermint < v0.37-alpha returns base64 encoded strings in events.
			if keyStr == "proposal_id" {
				// value is already plain string
			} else {
				kb, err := base64.StdEncoding.DecodeString(keyStr)
				if err == nil && string(kb) == "proposal_id" {
					vb, err := base64.StdEncoding.DecodeString(valStr)
					if err == nil {
						valStr = string(vb)
					}
				} else {
					continue
				}
			}

			id, err := strconv.ParseUint(valStr, 10, 64)
			if err != nil {
				return nil, err
			}
			ids = append(ids, id)
		}
	}
	return ids, nil
}
