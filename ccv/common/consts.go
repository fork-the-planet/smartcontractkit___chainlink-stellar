package common

import selectors "github.com/smartcontractkit/chain-selectors"

const (
	StellarDeployerKeypairEnv   = "STELLAR_DEPLOYER_PRIVATE_KEY"
	StellarCCIPMessageSentTopic = "onramp_1_7_CCIPMessageSent"

	// StellarTransmitterKeyName is the full keystore path of the Ed25519 key used by
	// the Stellar accessor as the transmitter / deployer keypair when signing Soroban
	// transactions. The "stellar/tx/" prefix mirrors the "evm/tx/" convention used by
	// chainlink-ccv's executor.DefaultEVMTransmitterKeyName.
	StellarTransmitterKeyName = selectors.FamilyStellar + "/tx/stellar_transmitter_ed25519_key"
)
