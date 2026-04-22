package accessors

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/BurntSushi/toml"

	chainsel "github.com/smartcontractkit/chain-selectors"
	"github.com/smartcontractkit/chainlink-ccv/pkg/chainaccess"
	"github.com/smartcontractkit/chainlink-stellar/bindings/scval"
	"github.com/smartcontractkit/chainlink-stellar/ccv/common"
	sourcereader "github.com/smartcontractkit/chainlink-stellar/ccv/source_reader"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validContractHex = "0x0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"

func TestStellarConfigPath(t *testing.T) {
	t.Run("uses env when set", func(t *testing.T) {
		t.Setenv(StellarConfigPathEnv, "/custom/stellar.toml")
		assert.Equal(t, "/custom/stellar.toml", stellarConfigPath())
	})

	t.Run("uses default when env unset", func(t *testing.T) {
		require.NoError(t, os.Unsetenv(StellarConfigPathEnv))
		assert.Equal(t, common.DefaultStellarConfigPath, stellarConfigPath())
	})
}

func TestMergeReaderConfig(t *testing.T) {
	base := sourcereader.ReaderConfig{
		NetworkPassphrase:   "file-pass",
		SorobanRPCURL:       "http://file",
		OnRampContractID:    "file-onramp",
		RMNRemoteContractID: "file-rmn",
	}
	overlay := sourcereader.ReaderConfig{
		SorobanRPCURL:    "http://job",
		OnRampContractID: "job-onramp",
	}
	out := mergeReaderConfig(base, overlay)
	assert.Equal(t, "file-pass", out.NetworkPassphrase, "empty overlay field preserves base")
	assert.Equal(t, "http://job", out.SorobanRPCURL)
	assert.Equal(t, "job-onramp", out.OnRampContractID)
	assert.Equal(t, "file-rmn", out.RMNRemoteContractID, "zero-value overlay string does not overwrite")
}

func TestMergeFileAndJobReaderConfigs(t *testing.T) {
	sel := strconv.FormatUint(chainsel.STELLAR_LOCALNET.Selector, 10)
	file := map[string]sourcereader.ReaderConfig{
		sel: {
			NetworkPassphrase: "from-file",
			SorobanRPCURL:     "http://file-rpc",
		},
	}
	job := chainaccess.Infos[sourcereader.ReaderConfig]{
		sel: {SorobanRPCURL: "http://job-rpc"},
	}
	out := mergeFileAndJobReaderConfigs(file, job)
	require.Contains(t, out, sel)
	assert.Equal(t, "from-file", out[sel].NetworkPassphrase)
	assert.Equal(t, "http://job-rpc", out[sel].SorobanRPCURL)
}

func TestMergeFileAndJobReaderConfigs_jobOnlyChain(t *testing.T) {
	sel := strconv.FormatUint(chainsel.STELLAR_LOCALNET.Selector, 10)
	job := chainaccess.Infos[sourcereader.ReaderConfig]{
		sel: {SorobanRPCURL: "http://rpc", NetworkPassphrase: "p"},
	}
	out := mergeFileAndJobReaderConfigs(nil, job)
	require.Len(t, out, 1)
	assert.Equal(t, "http://rpc", out[sel].SorobanRPCURL)
}

func TestApplyOnRampRMNHexOverrides(t *testing.T) {
	sel := strconv.FormatUint(chainsel.STELLAR_LOCALNET.Selector, 10)
	wantStrkey, err := scval.HexToContractStrkey(validContractHex)
	require.NoError(t, err)

	t.Run("fills empty ids from hex maps", func(t *testing.T) {
		rc := map[string]sourcereader.ReaderConfig{
			sel: {SorobanRPCURL: "http://x"},
		}
		onRamp := map[string]string{sel: validContractHex}
		rmn := map[string]string{sel: validContractHex}
		require.NoError(t, applyOnRampRMNHexOverrides(rc, onRamp, rmn))
		assert.Equal(t, wantStrkey, rc[sel].OnRampContractID)
		assert.Equal(t, wantStrkey, rc[sel].RMNRemoteContractID)
	})

	t.Run("skips when strkey already set", func(t *testing.T) {
		rc := map[string]sourcereader.ReaderConfig{
			sel: {OnRampContractID: "already-set", SorobanRPCURL: "http://x"},
		}
		onRamp := map[string]string{sel: validContractHex}
		require.NoError(t, applyOnRampRMNHexOverrides(rc, onRamp, nil))
		assert.Equal(t, "already-set", rc[sel].OnRampContractID)
	})

	t.Run("invalid onramp hex returns error", func(t *testing.T) {
		rc := map[string]sourcereader.ReaderConfig{sel: {}}
		onRamp := map[string]string{sel: "0xZZZZ"}
		err := applyOnRampRMNHexOverrides(rc, onRamp, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "convert OnRamp hex")
	})
}

func TestLoadStellarJobReaderInfos(t *testing.T) {
	sel := chainsel.STELLAR_LOCALNET.Selector
	raw := `
[blockchain_infos.` + strconv.FormatUint(sel, 10) + `]
soroban_rpc_url = "http://from-job"
network_passphrase = "job-pass"
`
	var gc chainaccess.GenericConfig
	_, err := toml.Decode(raw, &gc)
	require.NoError(t, err)

	infos, err := loadStellarJobReaderInfos(gc)
	require.NoError(t, err)
	require.Len(t, infos, 1)
	got := infos[strconv.FormatUint(sel, 10)]
	assert.Equal(t, "http://from-job", got.SorobanRPCURL)
	assert.Equal(t, "job-pass", got.NetworkPassphrase)
}

func TestBuildStellarReaderConfigs(t *testing.T) {
	sel := chainsel.STELLAR_LOCALNET.Selector
	selStr := strconv.FormatUint(sel, 10)

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "stellar.toml")
	fileContents := `
[reader_configs.` + selStr + `]
network_passphrase = "file-pass"
soroban_rpc_url = "http://file-rpc"
`
	require.NoError(t, os.WriteFile(cfgPath, []byte(fileContents), 0o600))

	rawJob := `
[blockchain_infos.` + selStr + `]
network_passphrase = "job-pass"
`
	var gc chainaccess.GenericConfig
	_, err := toml.Decode(rawJob, &gc)
	require.NoError(t, err)
	gc.OnRampAddresses = map[string]string{selStr: validContractHex}

	out, err := buildStellarReaderConfigs(cfgPath, gc)
	require.NoError(t, err)
	require.Contains(t, out, selStr)
	assert.Equal(t, "job-pass", out[selStr].NetworkPassphrase)
	assert.Equal(t, "http://file-rpc", out[selStr].SorobanRPCURL)
	require.NotEmpty(t, out[selStr].OnRampContractID, "onramp filled from hex map")
}
