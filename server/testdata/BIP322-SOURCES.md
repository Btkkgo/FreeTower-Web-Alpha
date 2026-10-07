# Official BIP-322 test fixtures

Retrieved 2026-10-07 from the Bitcoin BIPs repository:
- https://github.com/bitcoin/bips/blob/master/bip-0322.mediawiki
- https://github.com/bitcoin/bips/blob/master/bip-0322/basic-test-vectors.json
- https://github.com/bitcoin/bips/blob/master/bip-0322/generated-test-vectors.json

The BIP and vectors are CC0-1.0. Public test private keys were removed from the copies because verification only needs messages, addresses and signatures. Signatures and transaction hashes are unchanged.

Supported: P2WPKH and P2TR key-path simple signatures, current `smp` prefix and historical unprefixed UniSat witnesses. Unsupported P2WSH, P2SH, legacy and Taproot script-path signatures are rejected rather than reported verified.
