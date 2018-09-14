# Livepeer Node Database Schema

Note that foreign keys constraints are not enforced at runtime, except in some tests.

Tables:
* [kv](#table-kv)
* [jobs](#table-jobs)
* [claims](#table-claims)
* [receipts](#table-receipts)
* [broadcasts](#table-broadcasts)
* [unbondingLocks](#table-unbondingLocks)

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


## Table `jobs`

**Transcoder only.** Stores information about jobs that the transcoder is assigned to.

Column | Type | Description
--- | --- | ---
id |  INTEGER PRIMARY KEY| The ID of the on-chain job
recordedAt | STRING DEFAULT CURRENT_TIMESTAMP | Time this row was inserted, *after* the job transaction has been confirmed on chain.
streamID | STRING | Stream identifier, composed of `<VideoID><Rendition>`.
segmentPrice | INTEGER | The cost of transcoding each segment, in wei.
transcodeOptions | STRING | [Encoded version of the rendition strings](https://github.com/livepeer/go-livepeer/blob/fac0393950e821f16555a17eb460b8abe74f6ada/common/videoprofile_ids.go#L5-L15).
broadcaster | STRING | Broadaster's Eth address.
transcoder | STRING | Transcoder's Eth address. Should be your node's address.
startBlock | INTEGER | Block height from when the job was submitted on-chain.
endBlock | INTEGER | Block height at which the job expires.
stopReason | STRING | Reason for stopping a job on the transcoder. This may be before `endBlock` is reached. **Deprecated**
stoppedAt | STRING | Time which the job was stopped on the transcoder. **Deprecated**

## Table `claims`

**Transcoder only.** Stores and tracks claims as they go through the submission process.

Column | Type | Description
--- | --- | ---
id | INTEGER | The claim ID. Starts at zero for every job.
jobID | INTEGER | The ID of the job to claim
claimRoot | STRING | Merkle root of the claim
claimBlock | INTEGER | Block the claim tx was submitted at
claimedAt | STRING DEFAULT CURRENT_TIMESTAMP | Time of submission
updatedAt | STRING DEFAULT CURRENT_TIMESTAMP | Time of last update
status  | STRING DEFAULT 'Created' | Claim status. May indicate an incomplete claim or error if the status is not `Complete`.
&nbsp; | PRIMARY KEY(id, jobID) | Composite PK to ensure we don't try to submit a claim several times.
&nbsp; | FOREIGN KEY(jobID) REFERENCES jobs(id) | FK constraints are not enforced at the moment

## Table `receipts`

**Transcoder only.** Tracks segments as they go through the transcoding process.

Column | Type | Description
--- | --- | ---
jobID |  INTEGER NOT NULL | The job this receipt belongs to.
claimID | INTEGER | The claim in which this receipt was submitted at. May be NULL if a claim was not yet submitted.
seqNo | INTEGER NOT NULL  | Sequence  number of this receipt.
bcastFile | STRING | Location on disk of the broadcast file. After verification succeeds, this file may not exist anymore.
bcastHash | STRING | Hash of the source segment from the broadcaster.
bcastSig | STRING | Broadcaster's signature verifying this segment, approximately `Sign(broadcasterKey, Hash(streamID \| seqNo \| bcastHash))`
transcodedHash | STRING | Hash of the concatenation of all transcoded outputs for this segment, `Hash(Hash(Transcode(Seg, Profile1)) \| ... \| Hash(Transcode(Seg, ProfileN)))`
transcodeStartedAt | STRING | Time that the transcode was started.
transcodeEndedAt | STRING | Time that the transcode completed, for all profiles.
errorMsg | STRING DEFAULT NULL | Errors encountered while transcoding, if any.
&nbsp; | PRIMARY KEY(jobID, seqNo) | PK ensures we have only one segment per job.
&nbsp; | FOREIGN KEY(jobID) REFERENCES jobs(id) | Ensures the job exists
&nbsp; | FOREIGN KEY(claimID, jobID) REFERENCES claims(id, jobID) | When the claimID is set, ensures the claim exists.

When starting up, a transcoder will check for any outstanding receipts that haven't been claimed yet. If the receipts are eligible to be claimed, then the transcoder will do so. Any segments that come in after startup will be claimed separately from recovered receipts.

Some reasons for a transcoder not claiming a receipt:
* The transcoder did not submit a claim within 256 blocks of being assigned a job.
* There was an error while processing the segment.
* [Claim interval policy](https://github.com/livepeer/go-livepeer/issues/371) (unimplemented)

## Table `broadcasts`

**Broadcaster only.** Tracks jobs that broadcasters have created. This is used for job reuse, among other things.

Column | Type | Description
---|---|---
id | INTEGER PRIMARY KEY | The ID of the on-chain job
recordedAt | STRING DEFAULT CURRENT_TIMESTAMP | Time this row was inserted, *after* the job transaction has been confirmed on chain.
streamID | STRING | Stream identifier, composed of `<VideoID><Rendition>`.
segmentPrice | INTEGER | The cost of transcoding each segment, in wei.
segmentCount | INTEGER DEFAULT 0 | The number of segments this job has produced.
transcodeOptions | STRING | [Encoded version of the rendition strings](https://github.com/livepeer/go-livepeer/blob/fac0393950e821f16555a17eb460b8abe74f6ada/common/videoprofile_ids.go#L5-L15).
broadcaster | STRING | Broadaster's Eth address. Should be your node's address.
transcoder | STRING | Transcoder's Eth address.
startBlock | INTEGER | Block height from when the job was submitted on-chain.
endBlock | INTEGER | Block height at which the job expires.
stopReason | STRING DEFAULT NULL | Reason for stopping a job on the broadcaster. This may be before `stopBlock` is reached.
stoppedAt | STRING DEFAULT NULL | Time which the job was stopped on the broadcaster.

## Table `unbondingLocks`

**All Nodes** Tracks unbonding in order to support partial unbonding.

Column | Type | Description
---|---|---
id | INTEGER NOT NULL | The ID of the on-chain unbonding lock
delegator | STRING | The owner of the unbonding lock. Should be your node's address.
amount | TEXT | Amount of tokens to be unbonded.
withdrawRound | int64 | Round at which unbonding period is over and tokens can be withdrawn.
usedBlock | int64 | Block at which the unbonding lock is used to withdraw or rebond tokens.
