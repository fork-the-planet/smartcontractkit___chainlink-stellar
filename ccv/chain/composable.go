package ccvchain

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/stellar/go-stellar-sdk/strkey"

	"github.com/smartcontractkit/chainlink-ccv/build/devenv/cciptestinterfaces"
	"github.com/smartcontractkit/chainlink-ccv/protocol"
	routerbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/router"
)

// StellarSendOptions is the cciptestinterfaces.ChainSendOption implementation for Stellar.
// Fields may be extended (e.g. alternate signer); unknown sendOption values are ignored.
type StellarSendOptions struct{}

// IsSendOption implements cciptestinterfaces.ChainSendOption.
func (StellarSendOptions) IsSendOption() bool { return true }

var (
	_ cciptestinterfaces.ChainAsSource      = (*Chain)(nil)
	_ cciptestinterfaces.ChainAsDestination = (*Chain)(nil)
)

// BuildChainMessage implements cciptestinterfaces.ChainAsSource.
// It returns a routerbindings.StellarToAnyMessage as cciptestinterfaces.ChainAsSourceMessage.
func (c *Chain) BuildChainMessage(ctx context.Context, destChain uint64, fields cciptestinterfaces.MessageFields, opts cciptestinterfaces.MessageOptions) (cciptestinterfaces.ChainAsSourceMessage, error) {
	_ = ctx
	_ = destChain
	// CCIP devenv policy: allow out-of-order execution on the destination path. The
	// Soroban GenericExtraArgsV3 struct has no OOO field yet; we still normalize opts
	// so callers and any future dest_blob / metadata wiring stay consistent.
	forced := opts
	forced.OutOfOrderExecution = true
	extraArgs, err := EncodeStellarSourceExtraArgsForOnRamp(c.deployerKeypair.Address(), c.vvrContractID, forced)
	if err != nil {
		return nil, fmt.Errorf("encode extra args for Stellar OnRamp: %w", err)
	}

	if c.feeTokenContractID == "" {
		return nil, fmt.Errorf("fee token not deployed; run DeployContractsForSelector first")
	}
	feeToken := c.feeTokenContractID
	if len(fields.FeeToken) > 0 {
		ft, encErr := strkey.Encode(strkey.VersionByteContract, []byte(fields.FeeToken))
		if encErr != nil {
			return nil, fmt.Errorf("encode fee token address: %w", encErr)
		}
		feeToken = ft
	}

	var tokenAmounts []routerbindings.TokenAmount
	if fields.TokenAmount.Amount != nil && fields.TokenAmount.Amount.Sign() > 0 && len(fields.TokenAmount.TokenAddress) > 0 {
		if !fields.TokenAmount.Amount.IsInt64() {
			return nil, fmt.Errorf("token amount out of int64 range: %s", fields.TokenAmount.Amount.String())
		}
		tokenAddr, encErr := strkey.Encode(strkey.VersionByteContract, []byte(fields.TokenAmount.TokenAddress))
		if encErr != nil {
			return nil, fmt.Errorf("encode token address for send: %w", encErr)
		}
		tokenAmounts = []routerbindings.TokenAmount{{
			Token:  tokenAddr,
			Amount: fields.TokenAmount.Amount.Int64(),
		}}
	}

	msg := routerbindings.StellarToAnyMessage{
		Receiver:     fields.Receiver,
		Data:         fields.Data,
		TokenAmounts: tokenAmounts,
		FeeToken:     feeToken,
		ExtraArgs:    extraArgs,
	}
	return msg, nil
}

// SendChainMessage implements cciptestinterfaces.ChainAsSource.
// msg must be the routerbindings.StellarToAnyMessage returned from BuildChainMessage.
func (c *Chain) SendChainMessage(ctx context.Context, destChain uint64, msg cciptestinterfaces.ChainAsSourceMessage, sendOption cciptestinterfaces.ChainSendOption) (cciptestinterfaces.MessageSentEvent, protocol.ByteSlice, error) {
	// Optional StellarSendOptions; other ChainSendOption types are ignored (EVM-style defaults).
	if _, ok := sendOption.(StellarSendOptions); ok {
		// Reserved for future per-send overrides.
	}
	routerMsg, ok := msg.(routerbindings.StellarToAnyMessage)
	if !ok {
		return cciptestinterfaces.MessageSentEvent{}, nil, fmt.Errorf("expected routerbindings.StellarToAnyMessage, got %T", msg)
	}
	if c.routerClient == nil {
		return cciptestinterfaces.MessageSentEvent{}, nil, fmt.Errorf("Router client not initialized")
	}
	sender := c.deployerKeypair.Address()

	requiredFee, err := c.routerClient.GetFee(ctx, destChain, routerMsg)
	if err != nil {
		return cciptestinterfaces.MessageSentEvent{}, nil, fmt.Errorf("get fee from Router: %w", err)
	}
	c.logger.Info().Int64("requiredFee", requiredFee).Msg("Fee quote from Router (SendChainMessage)")

	messageID, err := c.routerClient.CcipSend(ctx, sender, destChain, routerMsg, requiredFee)
	if err != nil {
		return cciptestinterfaces.MessageSentEvent{}, nil, fmt.Errorf("ccip_send: %w", err)
	}
	c.logger.Info().
		Str("messageID", hexutil.Encode(messageID[:])).
		Msg("CCIP message sent from Stellar via Router (SendChainMessage)")

	// Soroban deployer does not currently plumb transaction hash through CcipSend; composable
	// helpers treat tx hash as optional.
	return cciptestinterfaces.MessageSentEvent{
		MessageID: messageID,
		Sender:    protocol.UnknownAddress([]byte(sender)),
	}, nil, nil
}
