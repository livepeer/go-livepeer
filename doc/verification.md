# Verification

The Livepeer node supports two types of verification when running with the `-broadcaster` flag:

- Local verification
    - This currently involves pixel count and signature verification.
    - Pixel count verification ensures that the number of pixels that the orchestrator uses to charge the broadcaster for payments matches the number of pixels actually encoded by the orchestrator. The orchestrator reports the number of pixels encoded with each result returned to the broadcaster and the broadcaster compares this value with the actual number of pixels in the results.
    - Signature verification ensures that the results received are cryptographically signed using a known Ethereum account associated with an orchestrator. The Ethereum account used to sign the results may be an on-chain registered address or it may be an account specified in the address field of the `OrchestratorInfo` message sent to the broadcaster during discovery.
- Tamper verification
    - This currently uses an external verifier that checks if a video has been tampered.

Local verification is enabled by default when the node is connected to Rinkeby and mainnet and disabled by default when the node is running in off-chain mode. Local verification can be explicitly enabled by starting the node with `-localVerify` and can be explicitly disabled with `-localVerify=false`.

Tamper verification is disabled by default and can be enabled by specifying `-verifierURL`. See this [guide](https://livepeer.readthedocs.io/en/latest/broadcasting.html#transcoding-verification-experimental) for instructions on connecting the node to an external verifier that runs tamper verification. Note that when tamper verification is enabled, local verification is also enabled.