# Livepeer Orchestrator Selection and Reliability Strategy

## Overview

To achieve greater network scalability, a broadcaster works with multiple orchestrators at once. The Broadcaster stops working with any orchestrator that has gone offline or does not return transcoded segments, and "refreshes" the list of orchestrators it works with if "enough" orchestrators on its original list are unresponsive (see `Orchestrator List Refresh`). The Broadcaster distributes segments to Orchestrators using a "round robin" strategy (see `Orchestrator Selection`), given that Orchestrator cannot have more than one segment per stream in flight (this mitigates back logging). Therefore, a Broadcaster sends segments to "free" Orchestrators on their "saved list". The ability to send segments to a selection of Orchestrators gives individual Orchestrators more time to process segments, and prevents `OrchestratorBusy` errors.

## BroadcastSessionsManager

Orchestrators are managed by a `BroadcastSessionsManager` stored in the `rtmpConnections` on the `LivepeerServer` interface. The sessions manager is initiated when the RTMP stream is registered by `gotRTMPStreamHandler`. The orchestrator list is first populated then, when `refreshSessions` is called within `NewSessionManager`.

The `BroadcastSessionsManager` stores orchestrators in two lists, a `sessList` and a `sessMap`.  The `sessList` is an array containing a "working" list of orchestrators. When a Broadcaster is in need of an orchestrator, it selects and removes it from `sessList` only if said Orchestrator also exists in `sessMap`. Therefore, `sessMap` contains all orchestrators currently in use or available for use. It is a map with a string key of the URI of the orchestrator, and a value of that orchestrator's `BroadcastSession`.

## Orchestrator List Refresh

The orchestrator list is refreshed when the number of sessions in `sessList` is less than double the `HTTMPTimeout` in seconds (hard-coded to 8 seconds at the moment) divided by the length of segments (hard-coded to 2 seconds at the moment) OR less than the size of the OrchestratorPool saved on disk, whichever is less (i.e. when its length is less than what is required to keep in memory). This happens at startup (as described above), and when an orchestrator is selected for individual transcoding in `selectSession`.

## Orchestrator Selection

To give preference to O's that respond with transcoded segments quickly, instead of selecting an Orchestrator from the beginning of `sessList` when needed, and placing new Orchestrators that are finished processing a segment at the end, `selectSession` takes Orchestrators from the end of `sessList`. If transcoding is successful, it adds them back to the end of `sessList`. 

## Transcoding Errors & Retries

If there is an error uploading segment to an Orchestrator's OS, submitting the segment to an Orchestrator, downloading transcoded segments, or the segment signature check fails, the Orchestrator is removed from the `sessMap`. The segment is retried with a different Orchestrator. When `selectSession` is called in this retry scenario, though the removed session might still exist in `sessList`, only a session that still exists in `sessMap` will be selected.  If there is no error in segment transcoding, `completeSession` adds session back to `sessList`. Retries stop if `sessMap` is empty.

## Storage

To prevent segment front-running (when an Orchestrator writes to a file that should belong to another Orchestrator), each Orchestrator is given an external storage path prefix used to create its own unique OS session. The prefix is composed of the stream's ManifestID, and a randomly generated manifest Id.

## MaxSessions

When an Orchestrator - Transcoder are run on the same node, a `-maxSessions` flag can be used to specify the node's own capacity for transcoding. A `MaxSessions` hard-coded value in `Livepeernode.go` caps the number of segment channels that can be created per Orchestrator, which limits the number of streams it can ingest. `MaxSessions` is the default value that is overridden with `-maxSessions`.
