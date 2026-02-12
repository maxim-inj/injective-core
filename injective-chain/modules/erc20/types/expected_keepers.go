package types

import (
	"context"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/evm/statedb"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"

	evmtypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/evm/types"
	permissionstypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/permissions/types"
	tftypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/tokenfactory/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

type BankKeeper interface {
	HasSupply(ctx context.Context, denom string) bool
	GetDenomMetaData(ctx context.Context, denom string) (banktypes.Metadata, bool)
	SendCoinsFromModuleToAccount(ctx context.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error
	SendCoinsFromAccountToModule(ctx context.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error
	MintCoins(ctx context.Context, moduleName string, amt sdk.Coins) error
	BurnCoins(ctx context.Context, moduleName string, amt sdk.Coins) error
}

type AccountKeeper interface {
	GetSequence(ctx context.Context, addr sdk.AccAddress) (uint64, error)
}

type EVMKeeper interface {
	GetAccount(ctx sdk.Context, addr common.Address) *statedb.Account
	EthCall(c context.Context, req *evmtypes.EthCallRequest) (*evmtypes.MsgEthereumTxResponse, error)
	ApplyTransaction(ctx sdk.Context, msg *evmtypes.MsgEthereumTx) (*evmtypes.MsgEthereumTxResponse, error)
}

type TokenFactoryKeeper interface {
	GetAuthorityMetadata(ctx sdk.Context, denom string) (tftypes.DenomAuthorityMetadata, error)
}

type CommunityPoolKeeper interface {
	FundCommunityPool(ctx context.Context, amount sdk.Coins, sender sdk.AccAddress) error
}

type PermissionsKeeper interface {
	HasNamespace(ctx sdk.Context, denom string) bool
	HasPermissionsForAction(ctx sdk.Context, denom string, actor sdk.AccAddress, action permissionstypes.Action) bool
	IsActionDisabledByPolicy(ctx sdk.Context, denom string, action permissionstypes.Action) bool
}
