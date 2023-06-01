# Probabilistic Micropayments (PM) Client

The `pm` package contains a client implementation of the [Livepeer probabilistic micropayment protocol](https://github.com/livepeer/wiki/blob/master/spec/streamflow/pm.md). The main areas of interest are:

- A `Ticket` type which defines the ticket format that both senders and recipients accept.
- A `Sender` type which is responsible for creating tickets.
- A `Recipient` type which is responsible for validating/receiving tickets and redeeming winning tickets using the `TicketBroker` interface (implemented by an Ethereum smart contract). This type will be able to translate a `Ticket` type into the correct format expected by the `TicketBroker`

In practice, the package user will need to define a communication channel (i.e. HTTP connection) and a network message format (i.e. protocol buffer) that peers will use to send and receive tickets. The sending peer will use the `Sender` type and the receiving peer will use the `Recipient` type.

## High Level Flow

![flow diagram](assets/diagram.png)

## RNG

A `Sender` and a `Recipient` execute a collaborative commit-reveal protocol in order to generate the random number used to determine if a ticket won or not. The random number construction is:

```
keccak256(abi.encodePacked(senderSig, recipientRand))
```

The `abi.encodePacked()` Solidity built-in is used to match the behavior of the contract implementing the `TicketBroker` interface. `senderSig` is a signature produced by `Sender` over the hash of the ticket and `recipientRand` is the pre-image of the `recipientRandHash` that is included in the ticket.

Before a `Sender` can start creating and sending tickets, it must first fetch ticket parameters from a `Recipient`. When generating ticket parameters, a `Recipient` will generate a random number `recipientRand`, compute the hash of `recipientRand` as `recipientRandHash` and include the hash with the ticket parameters. Once a `Sender` has a set of ticket parameters with a `recipientRandHash`, the `Sender` will use a monontonically increasing `senderNonce` with each new ticket that it creates.

The following conditions are met in order to ensure that the random number generation for winning ticket selection is fair:

- Once a `Sender` has a set of ticket parameters with a `recipientRandHash`, the `Sender` will use a montonically increasing `senderNonce` with each new ticket that it creates. The montonically increasing `senderNonce` will ensure that `senderSig` is unpredictable to the `Recipient`.
- Whenever a `Recipient` receives a ticket that uses a particular `recipientRandHash`, it will update an in-memory map of already received `senderNonce` values for the `recipientRandHash`. If the `Recipient` ever receives a ticket with a `senderNonce` that is already tracked by the map OR the maximum number of `senderNonce` values have been received for the `recipientRandHash` (a `Sender` is expected to regularly request for a new set of ticket parameters with a new `recipientRandHash` which means the maximum number of `senderNonce` values for a `recipientRandHash` would typically never be reached) it will consider the ticket a replay attack where the `Sender` is reusing an old `senderNonce`
- Whenever a `Recipient` redeems a winning ticket, it will wait until the ticket parameters expire (based on the `ParamsExpirationBlock` of the ticket parameters) to redeem the ticket. This behavior allows the `Recipient` to avoid tracking `recipientRandHash` values to prevent replays because a `Sender` that attempts to replay a `recipientRandHash` value that already is associated with a redeemed winning ticket (and thus the `recipientRand` pre-image was revealed on-chain) would fail the ticket parameters expiration check

Refer to the [spec](https://github.com/livepeer/wiki/blob/master/spec/streamflow/pm.md) for more information on the specifics of the random number generation for winning ticket selection.