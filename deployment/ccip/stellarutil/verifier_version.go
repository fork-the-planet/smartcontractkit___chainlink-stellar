package stellarutil

// DefaultCommitteeVerifierVersionTag is the default bytes4 committee verifier
// version tag used when initializing the Stellar CommitteeVerifier. Matches
// bytes4(keccak256("CommitteeVerifier 2.0.0")) — same as EVM
// chains/evm/deployment/v2_0_0/verifier_tags.CommitteeVerifierV2 and
// ccvs-committee-verifier `DEFAULT_VERIFIER_VERSION_TAG`.
var DefaultCommitteeVerifierVersionTag = [4]byte{0xe9, 0xa0, 0x5a, 0x20}
