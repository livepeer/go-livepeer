# Livepeer Node Database Schema

Note that foreign keys constraints are not enforced at runtime, except in some tests.

Tables:
* [kv](#table-kv)
* [orchestrators](#table-orchestrators)
* [unbondingLocks](#table-unbondingLocks)
* [winningTickets](#table-winningTickets)

## Table `kv`

**All Nodes** Generic key-value table for miscellaneous data.

Column | Type | Description
--- | --- | ---
key | STRING PRIMARY KEY |
value | STRING |
updatedAt | STRING DEFAULT CURRENT_TIMESTAMP |

Values that may be set in the `kv` table:

Key | Description
--- | ---
dbVersion |  The version of this database schema. Used to check compatibility and run migrations if needed.
lastBlock | The last seen block.

## Table `orchestrators`

**Broadcaster only.** Cache for the orchestrators that a broadcaster is aware of.

Column | Type | Description
--- | --- | ---
ethereumAddr | STRING PRIMARY KEY | Eth address of the orchestrator.
createdAt | STRING DEFAULT CURRENT_TIMESTAMP NOT NULL | Time this row was inserted.
updatedAt | STRING DEFAULT CURRENT_TIMESTAMP NOT NULL | Time this row was updated.
serviceURI | STRING | The serviceURI that can be used to contact the orchestrator.

## Table `unbondingLocks`

**All Nodes** Tracks unbonding in order to support partial unbonding.

Column | Type | Description
---|---|---
id | INTEGER NOT NULL | The ID of the on-chain unbonding lock
delegator | STRING | The owner of the unbonding lock. Should be your node's address.
amount | TEXT | Amount of tokens to be unbonded.
withdrawRound | int64 | Round at which unbonding period is over and tokens can be withdrawn.
usedBlock | int64 | Block at which the unbonding lock is used to withdraw or rebond tokens.

## Table `winningTickets` (DEPRECATED)

**Orchestrator only.** Tracks winning tickets for probabilistic micropayments.

Column | Type | Description
---|---|---
createdAt | STRING DEFAULT CURRENT_TIMESTAMP | Time this row was inserted.
sender | STRING | Address of the broadcaster that sent the winning ticket.
recipient | STRING | Address of the orchestrator that payments will be credited to.
faceValue | BLOB | Face value of the ticket, in wei.
winProb | BLOB | The ticket's winning probability in the range of 0 through 2^256-1.
senderNonce | INTEGER | Nonce incorporated by the broadcaster with each ticket.
recipientRand | BLOB | Value used by the orchestrator when constructing the initial ticket parameters.
recipientRandHash | STRING | Hash of the recipient rand, keccak256(recipientRand).
sig | BLOB | The broadcaster's signature over the ticket parameters.
sessionID | STRING | Broadcast session which this ticket belongs to.

## Table `ticketQueue`

**Orchestrator/Redeemer** only. `ticketQueue` Tracks winning tickets for probabilistic micropayments.

Column | Type | Description
---|---|---
createdAt | DATETIME DEFAULT CURRENT_TIMESTAMP | Time this row was inserted. 
sender | STRING | Address of the broadcaster that sent the winning ticket.
recipient | STRING | Address of the orchestrator that payments will be credited to.
faceValue | BLOB | Face value of the ticket, in wei.
winProb | BLOB | The ticket's winning probability in the range of 0 through 2^256-1.
senderNonce | INTEGER | Nonce incorporated by the broadcaster with each ticket.
recipientRand | BLOB | Value used by the orchestrator when constructing the initial ticket parameters.
recipientRandHash | STRING | Hash of the recipient rand, keccak256(recipientRand).
sig | BLOB PRIMARY KEY | The broadcaster's signature over the ticket parameters.
creationRound | int64 | The round in which the ticket was created.
creationRoundBlockHash | STRING | The block hash of the block the ticket creation round was initialised.
paramsExpirationBlock | int64 | The block height at which the current recipientRand expires.
redeemedAt | DATETIME | Time the ticket was redeemed on-chain.
txHash | STRING | Transaction hash of the winning ticket redemption on-chain. 