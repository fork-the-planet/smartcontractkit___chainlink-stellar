package sequences

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	cldfstellar "github.com/smartcontractkit/chainlink-deployments-framework/chain/stellar"
	"github.com/stellar/go-stellar-sdk/keypair"
	"github.com/stellar/go-stellar-sdk/xdr"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/chainlink-ccip/deployment/deploy"
)

type testStellarSigner struct{ addr string }

func (testStellarSigner) Sign([]byte) ([]byte, error)                          { return nil, nil }
func (testStellarSigner) SignDecorated([]byte) (xdr.DecoratedSignature, error)  { return xdr.DecoratedSignature{}, nil }
func (s testStellarSigner) Address() string                                     { return s.addr }
func (testStellarSigner) KeypairFull() *keypair.Full                            { return nil }

func TestStellarTimelockAdmin_zeroEVMUsesChainSigner(t *testing.T) {
	ch := cldfstellar.Chain{Signer: testStellarSigner{addr: "G_CHAIN_SIGNER"}}
	in := deploy.MCMSDeploymentConfigPerChainWithAddress{
		MCMSDeploymentConfigPerChain: deploy.MCMSDeploymentConfigPerChain{
			TimelockAdmin: common.Address{},
		},
	}
	got, err := stellarTimelockAdmin(in, ch)
	require.NoError(t, err)
	require.Equal(t, "G_CHAIN_SIGNER", got)
}

func TestStellarTimelockAdmin_rejectsNonZeroEVMAddress(t *testing.T) {
	ch := cldfstellar.Chain{Signer: testStellarSigner{addr: "G_CHAIN_SIGNER"}}
	in := deploy.MCMSDeploymentConfigPerChainWithAddress{
		MCMSDeploymentConfigPerChain: deploy.MCMSDeploymentConfigPerChain{
			TimelockAdmin: common.HexToAddress("0x00000000000000000000000000000000000000f1"),
		},
	}
	_, err := stellarTimelockAdmin(in, ch)
	require.Error(t, err)
}
