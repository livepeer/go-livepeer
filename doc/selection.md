# Selection

A broadcaster uses a *selection* algorithm to select the orchestrator to use for the next segment to be transcoded from a list of eligible active orchestrators.

The selection algorithm is implemented by a selector that implements the [BroadcastSessionsSelector](https://github.com/livepeer/go-livepeer/blob/master/server/selection.go) interface.

The current default selector implementation is the [MinLSSelectorWithRandFreq](https://github.com/livepeer/go-livepeer/blob/1af0a5182cd3a9aa38d961b6d1d104a3693ec814/server/selection.go#L118) which does the following:

- Tracks "unknown sessions" and "known sessions"
- A session is unknown if there is no latency score tracked yet i.e. no segment was transcoded by the orchestrator for this session yet
- A session is known if there is a latency score tracked i.e. a segment was transcoded by the orchestrator for this session already
- A latency score is calculated as the ratio between segment duration and the round trip response time (i.e. upload, transcode, download)
- If there are no known sessions available, then select from the unknown sessions
- If the best latency score of all known sessions does not meet the latency score threshold, then select from the unknown sessions
- Otherwise, select from the known sessions

**Selecting Unknown Sessions**

- X% of the time select an unknown session randomly
- The rest of the time, use stake weighted random selection to select the unknown session

**Selecting Known Sessions**

- Select the known session with the best latency score