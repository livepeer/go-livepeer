# Unreleased Changes

## vX.X

### Breaking Changes 🚨🚨

### Features ⚒

#### General
- \#2289 Add timeouts to ETH client (@leszko)

- \#2282 Add checksums and gpg signature support with binary releases. (@hjpotter92)

#### Broadcaster

#### Orchestrator

#### Transcoder

### Bug Fixes 🐞

#### General
- \#2299 Split devtool Orchestrator run scripts into versions with/without external transcoder and prevent Transcoder/Broadcaster run scripts from using same CLI port (@thomshutt)

#### Broadcaster
- \#2296 Increase orchestrator discovery timeout from `500ms` to `1` (@leszko)
- \#2291 Calling video comparison to improve the security strength (@oscar-davids)
- \#2326 Split Auth/Webhook functionality into its own file (@thomshutt)

#### Orchestrator
- \#2284 Fix issue with not redeeming tickets by Redeemer (@leszko)

#### Transcoder
