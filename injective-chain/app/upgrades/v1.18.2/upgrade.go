package v1dot18dot2

import (
	"strings"

	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	gethtypes "github.com/ethereum/go-ethereum/common"

	"github.com/InjectiveLabs/injective-core/injective-chain/app/upgrades"
	erc20types "github.com/InjectiveLabs/injective-core/injective-chain/modules/erc20/types"
	permissionskeeper "github.com/InjectiveLabs/injective-core/injective-chain/modules/permissions/keeper"
	permissionstypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/permissions/types"
)

const (
	UpgradeVersion = "v1.18.2"

	MainnetUSDC = "0xa00C59fF5a080D2b954d0c75e46E22a0c371235a"
	TestnetUSDC = "0x0C382e685bbeeFE5d3d9C29e29E341fEE8E84C5d"
)

func StoreUpgrades() storetypes.StoreUpgrades {
	return storetypes.StoreUpgrades{
		Added:   nil,
		Renamed: nil,
		Deleted: nil,
	}
}

func UpgradeSteps() []*upgrades.UpgradeHandlerStep {
	return []*upgrades.UpgradeHandlerStep{
		upgrades.NewUpgradeHandlerStep(
			"Migrate EnforcedRestictionsContracts",
			UpgradeVersion,
			upgrades.MainnetChainID,
			MigrateEnforcedRestrictionContracts,
		),
	}
}

func MigrateEnforcedRestrictionContracts(ctx sdk.Context, app upgrades.InjectiveApplication, logger log.Logger) error {
	exchangeKeeper := app.GetExchangeKeeper()
	permissionsKeeper := app.GetPermissionsKeeper()

	exchangeParams := exchangeKeeper.GetParams(ctx)
	permissionsParams := permissionsKeeper.GetParams(ctx)
	deprecatedPermissionsContracts := permissionsParams.GetDeprecatedEnforcedRestrictionsContracts()

	for i, contract := range exchangeParams.GetDeprecatedEnforcedRestrictionsContracts() {
		pauseEventSignature := contract.PauseEventSignature
		if pauseEventSignature == "" {
			pauseEventSignature = "Pause()"
		}

		enforcedEVMContrat := permissionstypes.EnforcedRestrictionsEVMContract{
			ContractAddress:           contract.ContractAddress,
			PauseEventSignature:       pauseEventSignature,
			BlacklistEventSignature:   "Blacklisted(address)",
			UnblacklistEventSignature: "", // we don't need this for USDC
			UnpauseEventSignature:     "", // we don't need this for USDC
		}

		permissionsParams.EnforcedRestrictionsEvmContracts = append(permissionsParams.EnforcedRestrictionsEvmContracts, enforcedEVMContrat)

		logger.Info("Migrated contract", "address", contract.ContractAddress)

		// consistency check – exchange.EnforcedRestrictionsContracts should match permissions.EnforcedRestrictionsContracts
		if i >= len(deprecatedPermissionsContracts) || deprecatedPermissionsContracts[i] != contract.ContractAddress {
			logger.Warn("exchangeParams.DeprecatedEnforcedRestrictionsContracts and permissionsParams.DeprecatedEnforcedRestrictionsContracts DO NOT MATCH!!!", "at_index", i)
		}
	}

	usdcContract := getUSDCContractByChainID(ctx.ChainID())
	if usdcContract != "" {
		ensureUSDCEnforcedRestrictionContract(&permissionsParams, usdcContract, logger)
	}

	permissionsParams.DeprecatedEnforcedRestrictionsContracts = nil
	permissionsKeeper.SetParams(ctx, permissionsParams)

	exchangeParams.DeprecatedEnforcedRestrictionsContracts = nil
	exchangeKeeper.SetParams(ctx, exchangeParams)

	if err := ensureUSDCNamespace(ctx, permissionsKeeper, usdcContract, logger); err != nil {
		return err
	}

	logger.Info("Migrated enforced restrictions contracts from exchange params to permissions params")

	return nil
}

func getUSDCContractByChainID(chainID string) string {
	switch chainID {
	case upgrades.MainnetChainID:
		return MainnetUSDC
	case upgrades.TestnetChainID:
		return TestnetUSDC
	default:
		return ""
	}
}

func ensureUSDCEnforcedRestrictionContract(params *permissionstypes.Params, usdcContract string, logger log.Logger) {
	contractAddr := gethtypes.HexToAddress(usdcContract).Hex()
	for _, contract := range params.EnforcedRestrictionsEvmContracts {
		if strings.EqualFold(contract.ContractAddress, contractAddr) {
			return
		}
	}

	params.EnforcedRestrictionsEvmContracts = append(params.EnforcedRestrictionsEvmContracts, permissionstypes.EnforcedRestrictionsEVMContract{
		ContractAddress:           contractAddr,
		PauseEventSignature:       "Pause()",
		BlacklistEventSignature:   "Blacklisted(address)",
		UnblacklistEventSignature: "",
		UnpauseEventSignature:     "",
	})

	logger.Info("Added USDC enforced restrictions EVM contract", "address", contractAddr)
}

func ensureUSDCNamespace(ctx sdk.Context, permissionsKeeper *permissionskeeper.Keeper, usdcContract string, logger log.Logger) error {
	if usdcContract == "" {
		return nil
	}

	contractAddr := gethtypes.HexToAddress(usdcContract).Hex()
	denom := erc20types.DenomPrefix + contractAddr
	if permissionsKeeper.HasNamespace(ctx, denom) {
		logger.Info("USDC namespace already exists", "denom", denom)
		return nil
	}

	govAddr := authtypes.NewModuleAddress(govtypes.ModuleName)

	namespace := permissionstypes.Namespace{
		Denom:   denom,
		EvmHook: contractAddr,
		RolePermissions: []*permissionstypes.Role{
			permissionstypes.NewRole(permissionstypes.EVERYONE, 0, permissionstypes.Action_RECEIVE, permissionstypes.Action_SEND),
			permissionstypes.NewRole(
				"admin",
				1,
				permissionstypes.Action_MODIFY_POLICY_MANAGERS,
				permissionstypes.Action_MODIFY_CONTRACT_HOOK,
				permissionstypes.Action_MODIFY_ROLE_PERMISSIONS,
				permissionstypes.Action_MODIFY_ROLE_MANAGERS,
			),
		},
		ActorRoles: []*permissionstypes.ActorRoles{
			permissionstypes.NewActorRoles(govAddr, "admin"),
		},
	}

	namespace.PopulateEmptyValuesWithDefaults(govAddr)

	if err := permissionsKeeper.CreateNamespaceForMigration(ctx, namespace); err != nil {
		return err
	}

	logger.Info("Configured USDC namespace with EVM hook", "denom", denom, "evm_hook", contractAddr)

	return nil
}
