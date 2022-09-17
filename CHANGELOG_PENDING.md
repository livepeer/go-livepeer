### Breaking Changes 🚨🚨
None

### Features ⚒
Enable orchestrator to set `pricePerUnit` and `pixelsPerUnit` for broadcasters by the broadcaster's ETH address.

#### Orchestrator
- `-pricePerBroadcaster` enables orchestrator to set pricePerPixel and pricePerUnit for *multiple broadcasters* by ETH address.

- `-freeStream` enables orchestrator to set pricePerPixel and pricePerUnit to 0 for a *specific broadcaster* by ETH address.

**Example Usage**

1. `-pricePerBroadcaster` is supported via CLI and HTTP interface:

- CLI 
`-pricePerBroadcaster {"broadcasters":[{"ethaddress":"0xEB62188121725A605f76a97791a1C69B4BdB435D","priceperunit":0,"pixelsperunit":1},{"ethaddress":"0xF115A3dfd39778A6719Ace6AA77A2aD228e90329","priceperunit":1200,"pixelsperunit":1}]}`

- CLI (JSON file) `-pricePerBroadcaster broadcasterPrices.json`

- HTTP 
`curl -X POST -d "pixelsPerUnit=1&pricePerUnit=9&broadcasterEthAddr=0xEB62188121725A605f76a97791a1C69B4BdB435D" http://127.0.0.1:7935/setPriceForBroadcaster`

2. `-freeStream` is supported via CLI interface:

- CLI 
`-freestream 0xEB62188121725A605f76a97791a1C69B4BdB435D`

#### Transcoder
None

### Bug Fixes 🐞
None