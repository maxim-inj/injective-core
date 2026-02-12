package base

import (
	"cosmossdk.io/math"
	"cosmossdk.io/store/prefix"
	"github.com/InjectiveLabs/metrics"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types/v2"
)

func (k *BaseKeeper) ExistsGrantAuthorization(
	ctx sdk.Context,
	granter sdk.AccAddress,
	grantee sdk.AccAddress,
) bool {
	defer metrics.ReportFuncCallAndTiming(k.svcTags)()
	key := types.GetGrantAuthorizationKey(granter, grantee)
	return k.getStore(ctx).Has(key)
}

func (k *BaseKeeper) GetGrantAuthorization(
	ctx sdk.Context,
	granter sdk.AccAddress,
	grantee sdk.AccAddress,
) math.Int {
	defer metrics.ReportFuncCallAndTiming(k.svcTags)()
	key := types.GetGrantAuthorizationKey(granter, grantee)

	bz := k.getStore(ctx).Get(key)
	if bz == nil {
		return math.ZeroInt()
	}
	return types.IntBytesToInt(bz)
}

func (k *BaseKeeper) GetAllGranterAuthorizations(ctx sdk.Context, granter sdk.AccAddress) []*v2.GrantAuthorization {
	defer metrics.ReportFuncCallAndTiming(k.svcTags)()

	authorizationsPrefix := types.GetGrantAuthorizationIteratorPrefix(granter)
	authorizationsStore := prefix.NewStore(k.getStore(ctx), authorizationsPrefix)

	authorizations := make([]*v2.GrantAuthorization, 0)

	iterateSafe(authorizationsStore.Iterator(nil, nil), func(key, value []byte) bool {
		grantee := sdk.AccAddress(key)
		amount := types.IntBytesToInt(value)
		authorizations = append(authorizations, &v2.GrantAuthorization{
			Grantee: grantee.String(),
			Amount:  amount,
		})
		return false
	})

	return authorizations
}

func (k *BaseKeeper) GetAllGrantAuthorizations(ctx sdk.Context) []*v2.FullGrantAuthorizations {
	defer metrics.ReportFuncCallAndTiming(k.svcTags)()

	authorizationsStore := prefix.NewStore(k.getStore(ctx), types.GrantAuthorizationsPrefix)

	fullAuthorizations := make([]*v2.FullGrantAuthorizations, 0)
	granters := make([]sdk.AccAddress, 0)
	authorizations := make(map[string][]*v2.GrantAuthorization, 0)

	iterateSafe(authorizationsStore.Iterator(nil, nil), func(key, value []byte) bool {
		granter := sdk.AccAddress(key[:common.AddressLength])

		if _, ok := authorizations[granter.String()]; !ok {
			granters = append(granters, granter)
			authorizations[granter.String()] = make([]*v2.GrantAuthorization, 0)
		}

		grantee := sdk.AccAddress(key[common.AddressLength:])
		amount := types.IntBytesToInt(value)

		authorizations[granter.String()] = append(authorizations[granter.String()], &v2.GrantAuthorization{
			Grantee: grantee.String(),
			Amount:  amount,
		})
		return false
	})

	for _, granter := range granters {
		fullAuthorizations = append(fullAuthorizations, &v2.FullGrantAuthorizations{
			Granter:                    granter.String(),
			TotalGrantAmount:           k.GetTotalGrantAmount(ctx, granter),
			LastDelegationsCheckedTime: k.GetLastValidGrantDelegationCheckTime(ctx, granter),
			Grants:                     authorizations[granter.String()],
		})
	}

	return fullAuthorizations
}

func (k *BaseKeeper) SetGrantAuthorization(
	ctx sdk.Context,
	granter sdk.AccAddress,
	grantee sdk.AccAddress,
	amount math.Int,
) {
	defer metrics.ReportFuncCallAndTiming(k.svcTags)()
	key := types.GetGrantAuthorizationKey(granter, grantee)

	if amount.IsZero() {
		k.deleteGrantAuthorization(ctx, granter, grantee)
		return
	}

	k.getStore(ctx).Set(key, types.IntToIntBytes(amount))
}

func (k *BaseKeeper) deleteGrantAuthorization(
	ctx sdk.Context,
	granter sdk.AccAddress,
	grantee sdk.AccAddress,
) {
	defer metrics.ReportFuncCallAndTiming(k.svcTags)()
	key := types.GetGrantAuthorizationKey(granter, grantee)

	k.getStore(ctx).Delete(key)
}

func (k *BaseKeeper) GetTotalGrantAmount(ctx sdk.Context, granter sdk.AccAddress) math.Int {
	defer metrics.ReportFuncCallAndTiming(k.svcTags)()

	bz := k.getStore(ctx).Get(types.GetTotalGrantAmountKey(granter))
	if bz == nil {
		return math.ZeroInt()
	}

	return types.IntBytesToInt(bz)
}

func (k *BaseKeeper) SetTotalGrantAmount(
	ctx sdk.Context,
	granter sdk.AccAddress,
	amount math.Int,
) {
	defer metrics.ReportFuncCallAndTiming(k.svcTags)()

	if amount.IsZero() {
		k.deleteTotalGrantAmount(ctx, granter)
		return
	}

	key := types.GetTotalGrantAmountKey(granter)
	k.getStore(ctx).Set(key, types.IntToIntBytes(amount))
}

func (k *BaseKeeper) deleteTotalGrantAmount(
	ctx sdk.Context,
	granter sdk.AccAddress,
) {
	defer metrics.ReportFuncCallAndTiming(k.svcTags)()
	key := types.GetTotalGrantAmountKey(granter)

	k.getStore(ctx).Delete(key)
}

func (k *BaseKeeper) GetActiveGrant(ctx sdk.Context, grantee sdk.AccAddress) *v2.ActiveGrant {
	defer metrics.ReportFuncCallAndTiming(k.svcTags)()
	key := types.GetActiveGrantKey(grantee)

	bz := k.getStore(ctx).Get(key)
	if bz == nil {
		return nil
	}

	var grant v2.ActiveGrant
	k.cdc.MustUnmarshal(bz, &grant)

	return &grant
}

func (k *BaseKeeper) GetAllActiveGrants(ctx sdk.Context) []*v2.FullActiveGrant {
	defer metrics.ReportFuncCallAndTiming(k.svcTags)()

	activeGrantsStore := prefix.NewStore(k.getStore(ctx), types.ActiveGrantPrefix)
	activeGrants := make([]*v2.FullActiveGrant, 0)

	iterateSafe(activeGrantsStore.Iterator(nil, nil), func(key, value []byte) bool {
		grantee := sdk.AccAddress(key)
		var grant v2.ActiveGrant
		k.cdc.MustUnmarshal(value, &grant)
		activeGrants = append(activeGrants, &v2.FullActiveGrant{
			Grantee:     grantee.String(),
			ActiveGrant: &grant,
		})
		return false
	})

	return activeGrants
}

func (k *BaseKeeper) SetActiveGrant(ctx sdk.Context, grantee sdk.AccAddress, grant *v2.ActiveGrant) {
	defer metrics.ReportFuncCallAndTiming(k.svcTags)()

	if grant.Amount.IsZero() {
		k.DeleteActiveGrant(ctx, grantee)
		return
	}

	key := types.GetActiveGrantKey(grantee)
	bz := k.cdc.MustMarshal(grant)

	k.getStore(ctx).Set(key, bz)
}

func (k *BaseKeeper) DeleteActiveGrant(ctx sdk.Context, grantee sdk.AccAddress) {
	defer metrics.ReportFuncCallAndTiming(k.svcTags)()
	key := types.GetActiveGrantKey(grantee)
	k.getStore(ctx).Delete(key)
}

func (k *BaseKeeper) SetLastValidGrantDelegationCheckTime(ctx sdk.Context, granter string, timestamp int64) {
	defer metrics.ReportFuncCallAndTiming(k.svcTags)()

	if granter == "" {
		return
	}

	key := types.GetLastValidGrantDelegationCheckTimeKey(sdk.MustAccAddressFromBech32(granter))
	k.getStore(ctx).Set(key, sdk.Uint64ToBigEndian(uint64(timestamp)))
}

func (k *BaseKeeper) GetLastValidGrantDelegationCheckTime(ctx sdk.Context, granter sdk.AccAddress) int64 {
	defer metrics.ReportFuncCallAndTiming(k.svcTags)()

	bz := k.getStore(ctx).Get(types.GetLastValidGrantDelegationCheckTimeKey(granter))
	if bz == nil {
		return 0
	}

	return int64(sdk.BigEndianToUint64(bz))
}
