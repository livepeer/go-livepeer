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

## Future

A few considerations for future iterations on selection algorithms:

- Consider additional inputs outside of just speed and price including transcoding quality (i.e. lowest loss compared to original signal) and efficiency (i.e. highest quality per bit)
	- Previous work:
		- [Leaderboard scoring framework](https://livepeer.notion.site/Leaderboard-Score-Framework-a420c0e9b6e4408b81cf0d9ffcd9d40e)
		- [Presentation](https://www.youtube.com/watch?v=ZDCg5feDELA) on selection framework based on technical constraints and economic preferences
- Should be able to express their preferences in a way that adjusts how much each input into the selection algorithm is weighted (i.e. weight speed over price, weight quality over speed, etc) and a list of weights (or a weights generation algorithm) could be used to customize how the selection algorithm works