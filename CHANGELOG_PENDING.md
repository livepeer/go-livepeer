# Unreleased Changes

## vX.X

### Features ⚒

#### General

- \#1911 [Experimental] Enable scene classification for Adult/Soccer (@jailuthra, @yondonfu)
- \#1915 Use gas price monitor for gas price suggestions for all Ethereum transactions (@kyriediculous)
- \#1930 Support custom minimum gas price (@yondonfu)
- \#1942 Log min and max gas price when monitoring is enabled (@kyriediculous)
- \#1923 Use a transaction manager with better transaction handling and optional replacement transactions instead of the default JSON-RPC client (@kyriediculous)
- \#1954 Add signer to Ethereum client config (@kyriediculous)

#### Broadcaster

- \#1877 Refresh TicketParams for the active session before expiry (@kyriediculous)
- \#1879 Add mp4 download of recorded stream (@darkdarkdragon)
- \#1899 Record million pixels processed metric (@yondonfu)
- \#1888 Should not save (when recording) segments with zero video frames (@darkdarkdragon)
- \#1908 Prevent Broadcaster from sending low face value PM tickets (@kyriediculous)
- \#1934 http push: return 422 for non-retryable errors (@darkdarkdragon)
- \#1933 server: Return 0 video frame segments unchanged
- \#1943 log maximum transcoding price when monitoring is enabled (@kyriediculous)
- \#1950 Fix extremely long delay before uploaded segment gets transcoded (@darkdarkdragon)

#### Orchestrator

- \#1931 Bump ticket redemption gas estimate to 350k to account for occasional higher gas usage (@yondonfu)

#### Transcoder

- \#1944 Enable B-frames in Nvidia encoder output (@jailuthra)

### Bug Fixes 🐞

#### General

#### Broadcaster

#### Orchestrator

#### Transcoder
