package common

import (
	sourcereader "github.com/smartcontractkit/chainlink-stellar/ccv/source_reader"
)

// TODO: This should be a global constant in the ccv package.
const DefaultStellarConfigPath = "/etc/config/stellar.toml"

type Config struct {
	// ReaderConfigs is a map of chain selectors (as decimal strings) to reader
	// configurations.  The TOML key is "reader_configs".
	ReaderConfigs map[string]sourcereader.ReaderConfig `toml:"reader_configs"`
}
