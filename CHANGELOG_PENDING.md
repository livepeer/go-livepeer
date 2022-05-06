# Unreleased Changes

## vX.X

### Breaking Changes 🚨🚨

### Features ⚒

#### General
- \#2348 Support Netint transcoding hardware (@cyberj0g)
- \#2289 Add timeouts to ETH client (@leszko)
- \#2282 Add checksums and gpg signature support with binary releases. (@hjpotter92)
- \#2344 Use T.TempDir to create temporary test directory (@Juneezee)
- \#2353 Codesign and notarize macOS binaries to be allowed to run without warnings on apple devices (@hjpotter92)
- \#2351 Refactor livepeer.go to enable running Livepeer node from the code (@leszko)
- \#2372 Upload test coverage reports to codecov (@hjpotter92)

#### Broadcaster
- \#2309 Add dynamic timeout for the orchestrator discovery (@leszko)
- \#2337 Fix dynamic discovery timeout to not retry sending requests, but wait for the same request to complete (@leszko)

#### Orchestrator
- \#2362 Backdate tickets by one round if the block hash for the current round is 0 (@leszko)

#### Transcoder

### Bug Fixes 🐞
- \#2355 Fix ZeroSegments error (@AlexKordic)

#### CLI
- \#2345 Improve user feedback when specifying numeric values for some wizard options (@kparkins)

#### General
- \#2299 Split devtool Orchestrator run scripts into versions with/without external transcoder and prevent Transcoder/Broadcaster run scripts from using same CLI port (@thomshutt)
- \#2346 Fix syntax errors in example JSON

#### Broadcaster
- \#2296 Increase orchestrator discovery timeout from `500ms` to `1` (@leszko)
- \#2291 Calling video comparison to improve the security strength (@oscar-davids)
- \#2326 Split Auth/Webhook functionality into its own file (@thomshutt)
- \#2357 Begin accepting auth header (from Mist) and have it override callback URL values
- \#2385 Change verification randomness variable meaning

#### Orchestrator
- \#2284 Fix issue with not redeeming tickets by Redeemer (@leszko)
- \#2352 Fix standalone orchestrator not crashing under UnrecoverableError (@leszko)
- \#2359 Fix redeeming tickets with zero block hash (@leszko)

#### Transcoder
