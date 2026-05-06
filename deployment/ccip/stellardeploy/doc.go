// Package stellardeploy implements full Stellar CCIP contract deployment for the CCV devenv.
//
// Layout mirrors the intent of github.com/smartcontractkit/chainlink-ccip/deployment
// (phased configuration applied around a shared orchestration path): helpers live in
// ../stellarutil; this package wires WASM deploy + bindings + datastore refs via Host.
package stellardeploy
