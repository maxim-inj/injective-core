all:

gen:
	solc --combined-json abi,bin src/PermissionsHook.sol > PermissionsHook.json
	abigen --pkg hooks --combined-json PermissionsHook.json > permissions_hook.abigen.go
	rm PermissionsHook.json
