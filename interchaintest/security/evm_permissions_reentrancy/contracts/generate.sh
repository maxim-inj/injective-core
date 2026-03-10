#!/bin/sh -e

solc --combined-json abi,bin \
  Bank.sol \
  CosmosTypes.sol \
  ReentrancyHook.sol \
  > ReentrancyHook.json
abigen --combined-json ReentrancyHook.json --pkg contracts --type ReentrancyHook --out ReentrancyHook.go
rm ReentrancyHook.json

exit 0
