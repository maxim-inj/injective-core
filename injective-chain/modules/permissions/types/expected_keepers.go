package types

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	evmtypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/evm/types"
)

type BankKeeper interface {
	AppendSendRestriction(restriction banktypes.SendRestrictionFn)
	PrependSendRestriction(restriction banktypes.SendRestrictionFn)
	ClearSendRestriction()
	SendCoinsFromModuleToAccount(ctx context.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error
}

type TokenFactoryKeeper interface {
	GetDenomAdmin(ctx sdk.Context, denom string) (sdk.AccAddress, error)
}

type WasmKeeper interface {
	HasContractInfo(ctx context.Context, contractAddress sdk.AccAddress) bool
	QuerySmart(ctx context.Context, contractAddr sdk.AccAddress, req []byte) ([]byte, error)
}

type EvmKeeper interface {
	EthCall(c context.Context, req *evmtypes.EthCallRequest) (*evmtypes.MsgEthereumTxResponse, error)
}
