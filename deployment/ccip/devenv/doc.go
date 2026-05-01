// Package devenv holds backward-compatible aliases and thin wrappers around
// [github.com/smartcontractkit/chainlink-stellar/deployment/ccip/stellardeploy]
// and [github.com/smartcontractkit/chainlink-stellar/deployment/ccip].
//
// Layout mirrors the intent of github.com/smartcontractkit/chainlink-ccip/deployment
// (phased configuration applied around a shared orchestration path): helpers live in
// ../stellarutil; Soroban ops run on a caller-supplied CLDF bundle via stellardeploy.
package devenv
