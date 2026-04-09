package stellarutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateContractAddress(t *testing.T) {
	networkPassphrase := "Test SDF Network ; September 2015"

	addr := GenerateContractAddress("test-contract", networkPassphrase)
	assert.Len(t, addr, 32)

	addr2 := GenerateContractAddress("test-contract", networkPassphrase)
	assert.Equal(t, addr, addr2)

	addr3 := GenerateContractAddress("other-contract", networkPassphrase)
	assert.NotEqual(t, addr, addr3)

	addr4 := GenerateContractAddress("test-contract", "Public Global Stellar Network ; September 2015")
	assert.NotEqual(t, addr, addr4)
}
