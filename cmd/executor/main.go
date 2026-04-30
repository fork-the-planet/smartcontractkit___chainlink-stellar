package main

import (
	"fmt"

	_ "github.com/lib/pq"
	"go.uber.org/zap/zapcore"

	chainsel "github.com/smartcontractkit/chain-selectors"
	"github.com/smartcontractkit/chainlink-ccv/bootstrap"
	executorcmd "github.com/smartcontractkit/chainlink-ccv/cmd/executor"
	sourcereader "github.com/smartcontractkit/chainlink-stellar/ccv/source_reader"
)

func main() {
	if err := bootstrap.Run(
		"StellarExecutor",
		executorcmd.NewServiceFactory[sourcereader.ReaderConfig](
			chainsel.FamilyStellar,
			CreateStellarExecutorComponents,
		),
		bootstrap.WithLogLevel(zapcore.InfoLevel),
	); err != nil {
		panic(fmt.Sprintf("failed to run Stellar executor: %s", err.Error()))
	}
}
