# Discovery

A broadcaster uses a *discovery* algorithm to fetch eligible active orchestrators to consider for selection.

A discovery algorithm uses a data source for fetch active orchestrators and then filters for eligible orchestrators.

At the moment, the following data sources are supported by the discovery algorithm:

- [On-chain](https://github.com/livepeer/go-livepeer/blob/master/discovery/db_discovery.go): The list of active orchestrators is fetched from the node's database which is populated with active orchestrator ETH addresses and their service URIs by a [OrchestratorWatcher](https://github.com/livepeer/go-livepeer/blob/master/eth/watchers/orchestratorwatcher.go) and [ServiceRegistryWatcher](https://github.com/livepeer/go-livepeer/blob/master/eth/watchers/serviceRegistryWatcher.go) respectively by monitoring and processing on-chain events. This data source is the default.
- [Webhook](https://github.com/livepeer/go-livepeer/blob/master/discovery/wh_discovery.go): The list of active orchestrators is fetched from a webhook server. The Livepeer Studio [webhook implementation](https://github.com/livepeer/studio/blob/master/packages/api/src/middleware/subgraph.ts) returns cached responses from the [Livepeer subgraph](https://thegraph.com/hosted-service/subgraph/livepeer/arbitrum-one). This data source can be configured using the `-orchWebhookUrl` flag.
- [Hardcoded list](https://github.com/livepeer/go-livepeer/blob/master/discovery/discovery.go): The list of active orchestrators is fetched from a hardcoded list. This data source can be configured using the `-orchAddr` flag.

After feching active orchestrators, the discovery algorithm filters for eligible orchestrators by:

- Choosing the first M orchestrators out of N that respond to a `GetOrchestrator` request within a [timeout](https://github.com/livepeer/go-livepeer/blob/1af0a5182cd3a9aa38d961b6d1d104a3693ec814/discovery/discovery.go#L127)
	- If an insufficient number of orchestrators are found within the timeout, a timeout escalation is used [here](https://github.com/livepeer/go-livepeer/blob/1af0a5182cd3a9aa38d961b6d1d104a3693ec814/discovery/discovery.go#L146) to continue trying to find orchestrators
	- At the moment, a broadcaster uses separate pools of trusted and untrusted orchestrators for fast verification and for the untrusted orchestrator pool discovery will set M = N (see [here](https://github.com/livepeer/go-livepeer/blob/1af0a5182cd3a9aa38d961b6d1d104a3693ec814/server/broadcast.go#L411) and [here](https://github.com/livepeer/go-livepeer/blob/1af0a5182cd3a9aa38d961b6d1d104a3693ec814/server/broadcast.go#L726))
- Exclusing orchestrators that are temporarily suspended
- Excluding orchestrators that do not have compatible capabilities for the job
- Excluding orchestrators that advertise invalid ticket parameters, that advertise a price that exceeds the broadcaster's max price

## Suspension

The discovery algorithm uses an in-memory suspension list (added in [this PR](https://github.com/livepeer/go-livepeer/pull/1435)) *per stream* (meaning this suspension list is *not* applied to all streams and if an orchestrator is suspended for stream A it is not necessarily suspended for stream B) to keep track of orchestrators that should be temporarily considered ineligible. If an orchestrator [is suspended](https://github.com/livepeer/go-livepeer/blob/1af0a5182cd3a9aa38d961b6d1d104a3693ec814/discovery/discovery.go#L133) it will be excluded unless there are an [insufficient number](https://github.com/livepeer/go-livepeer/blob/1af0a5182cd3a9aa38d961b6d1d104a3693ec814/discovery/discovery.go#L159) of non-suspended orchestrators (i.e. if the current number < M). `server/broadcast.go` contains the logic for initializing a suspension list (implemented as a [suspender](https://github.com/livepeer/go-livepeer/blob/1af0a5182cd3a9aa38d961b6d1d104a3693ec814/server/suspensions.go#L9)) and suspending orchestrators if the broadcaster encounters a suspendable error condition.
