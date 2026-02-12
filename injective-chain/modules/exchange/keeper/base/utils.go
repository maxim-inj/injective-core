package base

import (
	storetypes "cosmossdk.io/store/types"
)

// SubtractBitFromPrefix returns a prev prefix. It is calculated by subtracting 1 bit from the start value. Nil is not allowed as prefix.
//
//	Example: []byte{1, 3, 4} becomes []byte{1, 3, 5}
//			 []byte{15, 42, 255, 255} becomes []byte{15, 43, 0, 0}
//
// In case of an overflow the end is set to nil.
//
//	Example: []byte{255, 255, 255, 255} becomes nil
//
// MARK finish-batches: this is where some crazy shit happens
func SubtractBitFromPrefix(prefix []byte) []byte {
	if prefix == nil {
		panic("nil key not allowed")
	}

	// special case: no prefix is whole range
	if len(prefix) == 0 {
		return nil
	}

	// copy the prefix and update last byte
	newPrefix := make([]byte, len(prefix))
	copy(newPrefix, prefix)
	l := len(newPrefix) - 1
	newPrefix[l]--

	// wait, what if that overflowed?....
	for newPrefix[l] == 255 && l > 0 {
		l--
		newPrefix[l]--
	}

	// okay, funny guy, you gave us FFF, no end to this range...
	if l == 0 && newPrefix[0] == 255 {
		newPrefix = nil
	}

	return newPrefix
}

// AddBitToPrefix returns a prefix calculated by adding 1 bit to the start value. Nil is not allowed as prefix.
//
//	Example: []byte{1, 3, 4} becomes []byte{1, 3, 5}
//			 []byte{15, 42, 255, 255} becomes []byte{15, 43, 0, 0}
//
// In case of an overflow the end is set to nil.
//
//	Example: []byte{255, 255, 255, 255} becomes nil
func AddBitToPrefix(prefix []byte) []byte {
	if prefix == nil {
		panic("nil key not allowed")
	}

	// special case: no prefix is whole range
	if len(prefix) == 0 {
		return nil
	}

	// copy the prefix and update last byte
	newPrefix := make([]byte, len(prefix))
	copy(newPrefix, prefix)
	l := len(newPrefix) - 1
	newPrefix[l]++

	// wait, what if that overflowed?....
	for newPrefix[l] == 0 && l > 0 {
		l--
		newPrefix[l]++
	}

	// okay, funny guy, you gave us FFF, no end to this range...
	if l == 0 && newPrefix[0] == 0 {
		newPrefix = nil
	}

	return newPrefix
}

type iterCb func(k, v []byte) (stop bool)

// iterateSafe ensures the Iterator is closed even if the work done inside the callback panics.
func iterateSafe(iter storetypes.Iterator, callback iterCb) {
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		if callback(iter.Key(), iter.Value()) {
			return
		}
	}
}

type iterKeyCb func(k []byte) (stop bool)

// iterateKeysSafe only iterates over keys and ensures the Iterator is closed even if the work done inside the callback panics.
func iterateKeysSafe(iter storetypes.Iterator, callback iterKeyCb) {
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		if callback(iter.Key()) {
			return
		}
	}
}
