---
sidebar_position: 2
title: Messages  
---

# Messages

In this section we describe the processing of the auction messages and the corresponding updates to the state.

## Msg/Bid

An auction basket from a given round is bid upon by using the `Msg/Bid` service message.

```protobuf
// Bid defines a SDK message for placing a bid for an auction
message MsgBid {
  option (gogoproto.equal) = false;
  option (gogoproto.goproto_getters) = false;
  string sender = 1;
  // amount of the bid in INJ tokens
  cosmos.base.v1beta1.Coin bid_amount = 2 [(gogoproto.nullable) = false];
  // the current auction round being bid on
  uint64 round = 3;
}
```

### Behavior

The `BidAmount` in `Msg/Bid` always represents the **total bid amount** the sender wishes to have as their bid.

The `Msg/Bid` message supports two distinct use cases:

1. **New Bidder**: When a different address than the current highest bidder submits a bid, the full `BidAmount` is transferred from the sender to the auction module. The previous highest bidder is refunded, and the new bid is stored.

2. **Bid Increase**: When the current highest bidder submits another bid, the `BidAmount` must exceed their current bid. Only the **difference** between the new `BidAmount` and their existing bid is transferred from the sender to the auction module (no refund occurs since they're increasing their own bid).

### Validation

This service message is expected to fail if:

- `Round` does not equal the current auction round
- `BidAmount` does not exceed the previous highest bid amount by at least `min_next_increment_rate` percent.

### State Changes

- The stored bid always reflects the **total bid amount**
- Events emitted contain the **total bid amount**
