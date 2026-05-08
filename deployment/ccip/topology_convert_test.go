package ccip

import (
	"testing"

	ccvdeployment "github.com/smartcontractkit/chainlink-ccv/deployment"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCCVEnvironmentTopologyToOffchain_nilInput(t *testing.T) {
	t.Parallel()
	out, err := CCVEnvironmentTopologyToOffchain(nil)
	require.NoError(t, err)
	assert.Nil(t, out)
}

func TestCCVEnvironmentTopologyToOffchain_roundTripPyroscope(t *testing.T) {
	t.Parallel()
	in := &ccvdeployment.EnvironmentTopology{PyroscopeURL: "http://convert-test"}
	out, err := CCVEnvironmentTopologyToOffchain(in)
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, in.PyroscopeURL, out.PyroscopeURL)
}
