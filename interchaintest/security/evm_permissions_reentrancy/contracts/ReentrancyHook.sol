// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import {IBankModule} from "./Bank.sol";
import {Cosmos} from "./CosmosTypes.sol";

contract ReentrancyHook {
    address constant BANK_PRECOMPILE = 0x0000000000000000000000000000000000000064;
    address public owner;

    constructor() { owner = msg.sender; }
    receive() external payable {}

    function isTransferRestricted(
        address, address, Cosmos.Coin calldata
    ) external returns (bool) {
        IBankModule(BANK_PRECOMPILE).transfer(address(this), address(this), 1);
        return false;
    }

    function mintTokens() external {
        IBankModule(BANK_PRECOMPILE).mint(address(this), 1000);
    }

    function triggerRecursion() external {
        IBankModule(BANK_PRECOMPILE).transfer(address(this), address(this), 1);
    }
}
