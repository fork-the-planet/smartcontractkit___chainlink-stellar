package mcmsutil

import (
	"crypto/sha256"
	"fmt"

	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"

	"github.com/smartcontractkit/chainlink-ccip/deployment/deploy"
	"github.com/smartcontractkit/chainlink-ccip/deployment/utils"

	mcmsops "github.com/smartcontractkit/chainlink-stellar/deployment/operations/mcms"
)

// QualifierStr returns the qualifier string or empty if nil.
func QualifierStr(q *string) string {
	if q == nil {
		return ""
	}
	return *q
}

// ChainNetworkID is SHA-256 of the network passphrase (Soroban MCMS chain_network_id).
func ChainNetworkID(passphrase string) [32]byte {
	return sha256.Sum256([]byte(passphrase))
}

// MCMSDeploySalt derives a deterministic deploy salt for a Stellar MCMS instance.
func MCMSDeploySalt(chainSelector uint64, qual string) [32]byte {
	h := sha256.Sum256([]byte(fmt.Sprintf("stellar-mcms:%d:%s", chainSelector, qual)))
	return h
}

// TimelockDeploySalt derives a deterministic deploy salt for a Soroban timelock instance.
func TimelockDeploySalt(chainSelector uint64, qual string) [32]byte {
	h := sha256.Sum256([]byte(fmt.Sprintf("stellar-timelock:%d:%s", chainSelector, qual)))
	return h
}

// FindExistingStellarMCMS returns a contract id from ExistingAddresses if one of the MCMS alias types is present.
func FindExistingStellarMCMS(refs []datastore.AddressRef, chainSelector uint64, qual string) (string, bool) {
	v := deploy.MCMSVersion
	typesToMatch := []datastore.ContractType{
		datastore.ContractType(mcmsops.ContractType),
		datastore.ContractType(utils.ProposerManyChainMultisig),
		datastore.ContractType(utils.BypasserManyChainMultisig),
		datastore.ContractType(utils.CancellerManyChainMultisig),
	}
	for _, r := range refs {
		if r.ChainSelector != chainSelector {
			continue
		}
		if r.Qualifier != qual {
			continue
		}
		if !r.Version.Equal(v) {
			continue
		}
		for _, t := range typesToMatch {
			if r.Type == t && r.Address != "" {
				return r.Address, true
			}
		}
	}
	return "", false
}

// StellarMCMSDatastoreRefs emits the four datastore refs for a single Soroban MCMS (MCMS + EVM-style aliases).
func StellarMCMSDatastoreRefs(chainSelector uint64, qual, contractID string) []datastore.AddressRef {
	v := deploy.MCMSVersion
	mk := func(ct datastore.ContractType) datastore.AddressRef {
		return datastore.AddressRef{
			ChainSelector: chainSelector,
			Type:          ct,
			Version:       v,
			Qualifier:     qual,
			Address:       contractID,
		}
	}
	return []datastore.AddressRef{
		mk(datastore.ContractType(mcmsops.ContractType)),
		mk(datastore.ContractType(utils.ProposerManyChainMultisig)),
		mk(datastore.ContractType(utils.BypasserManyChainMultisig)),
		mk(datastore.ContractType(utils.CancellerManyChainMultisig)),
	}
}

// FindExistingStellarTimelock returns the RBACTimelock contract id from refs when present.
func FindExistingStellarTimelock(refs []datastore.AddressRef, chainSelector uint64, qual string) (string, bool) {
	v := deploy.MCMSVersion
	for _, r := range refs {
		if r.ChainSelector != chainSelector {
			continue
		}
		if r.Qualifier != qual {
			continue
		}
		if !r.Version.Equal(v) {
			continue
		}
		if r.Type == datastore.ContractType(utils.RBACTimelock) && r.Address != "" {
			return r.Address, true
		}
	}
	return "", false
}

// StellarTimelockDatastoreRef is a single RBACTimelock ref (matches EVM datastore labeling).
func StellarTimelockDatastoreRef(chainSelector uint64, qual, contractID string) datastore.AddressRef {
	return datastore.AddressRef{
		ChainSelector: chainSelector,
		Type:          datastore.ContractType(utils.RBACTimelock),
		Version:       deploy.MCMSVersion,
		Qualifier:     qual,
		Address:       contractID,
	}
}
