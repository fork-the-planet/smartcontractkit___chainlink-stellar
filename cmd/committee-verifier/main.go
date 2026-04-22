package main

import (
	"fmt"

	_ "github.com/lib/pq"
	"go.uber.org/zap/zapcore"

	"github.com/smartcontractkit/chainlink-ccv/bootstrap"
	cmd "github.com/smartcontractkit/chainlink-ccv/verifier/cmd"

	_ "github.com/smartcontractkit/chainlink-stellar/ccv/accessors" // registers Stellar chainaccess constructor
)

func main() {
	if err := bootstrap.Run(
		"StellarCommitteeVerifier",
		cmd.NewCommitteeVerifierServiceFactory(),
		bootstrap.WithLogLevel(zapcore.InfoLevel),
	); err != nil {
		panic(fmt.Sprintf("failed to run Stellar committee verifier: %s", err.Error()))
	}
}
