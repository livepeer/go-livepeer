# Discovery

A broadcaster uses a *discovery* algorithm to fetch eligible active orchestrators to consider for selection.

A discovery algorithm uses a data source for fetch active orchestrators and then filters for eligible orchestrators.

At the moment, the following data sources are supported by the discovery algorithm:

- [On-chain](https://github.com/livepeer/go-livepeer/blob/master/discovery/db_discovery.go): The list of active orchestrators is fetched from the node's database which is populated with active orchestrator ETH addresses and their service URIs by a [OrchestratorWatcher](https://github.com/livepeer/go-livepeer/blob/master/eth/watchers/orchestratorwatcher.go) and [ServiceRegistryWatcher](https://github.com/livepeer/go-livepeer/blob/master/eth/watchers/serviceRegistryWatcher.go) respectively by monitoring and processing on-chain events. This data source is the default.
- [Webhook](https://github.com/livepeer/go-livepeer/blob/master/discovery/wh_discovery.go): The list of active orchestrators is fetched from a webhook server. The Livepeer Studio [webhook implementation](https://github.com/livepeer/studio/blob/master/packages/api/src/middleware/subgraph.ts) returns cached responses from the [Livepeer subgraph](https://thegraph.com/hosted-service/subgraph/livepeer/arbitrum-one). This data source can be configured using the `-orchWebhookUrl` flag.
- [Hardcoded list](https://github.com/livepeer/go-livepeer/blob/master/discovery/discovery.go): The list of active orchestrators is fetched from a hardcoded list. This data source can be configured using the `-orchAddr` flag.

After feching active orchestrators, the discovery algorithm filters for eligible orchestrators by:

- Choosing the first M orchestrators out of N that respond to a `GetOrchestrator` request
- Excluding orchestrators that do not have compatible capabilities for the job
- Excluding orchestrators that advertise invalid ticket parameters, that advertise a price that exceeds the broadcaster's max price