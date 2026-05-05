package accessors

import (
	"context"
	"fmt"

	"github.com/smartcontractkit/chainlink-common/keystore"
	"github.com/stellar/go-stellar-sdk/network"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/txnbuild"
	"github.com/stellar/go-stellar-sdk/xdr"

	"github.com/smartcontractkit/chainlink-stellar/deployment"
)

// keystoreTxSigner is a deployment.TxSigner backed by a chainlink-common
// keystore. The Ed25519 private key never leaves the keystore: the transaction
// envelope is hashed locally, the keystore signs the hash, and the resulting
// 64-byte signature is appended to the transaction as a DecoratedSignature.
//
// This mirrors what (*txnbuild.Transaction).Sign does internally, but routes
// the actual signing call through keystore.Signer.Sign so production binaries
// can keep their key material isolated.
type keystoreTxSigner struct {
	ks       keystore.Keystore
	keyName  string
	address  string
	publicKey []byte
}

var _ deployment.TxSigner = (*keystoreTxSigner)(nil)

// LoadStellarKeystoreSigner constructs a deployment.TxSigner whose Ed25519
// private key lives in the given chainlink-common keystore. It validates that
// the requested key exists and is of type keystore.Ed25519, and pre-computes
// the Stellar G... address so callers can synchronously fund / log it without
// another keystore round-trip.
func LoadStellarKeystoreSigner(ctx context.Context, ks keystore.Keystore, keyName string) (deployment.TxSigner, error) {
	if ks == nil {
		return nil, fmt.Errorf("keystore is required")
	}
	if keyName == "" {
		return nil, fmt.Errorf("keyName is required")
	}
	resp, err := ks.GetKeys(ctx, keystore.GetKeysRequest{KeyNames: []string{keyName}})
	if err != nil {
		return nil, fmt.Errorf("get keystore key %q: %w", keyName, err)
	}
	if len(resp.Keys) != 1 {
		return nil, fmt.Errorf("expected exactly one keystore key for %q, got %d", keyName, len(resp.Keys))
	}
	info := resp.Keys[0].KeyInfo
	if info.KeyType != keystore.Ed25519 {
		return nil, fmt.Errorf("keystore key %q has type %s, expected %s", keyName, info.KeyType, keystore.Ed25519)
	}
	if len(info.PublicKey) != 32 {
		return nil, fmt.Errorf("keystore key %q has invalid Ed25519 public key length %d (want 32)", keyName, len(info.PublicKey))
	}

	address, err := strkey.Encode(strkey.VersionByteAccountID, info.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("encode Stellar address from public key: %w", err)
	}

	pk := make([]byte, len(info.PublicKey))
	copy(pk, info.PublicKey)
	return &keystoreTxSigner{
		ks:        ks,
		keyName:   keyName,
		address:   address,
		publicKey: pk,
	}, nil
}

// Address implements deployment.TxSigner.
func (s *keystoreTxSigner) Address() string { return s.address }

// SignTransaction implements deployment.TxSigner. It hashes the transaction
// envelope, asks the keystore to Ed25519-sign the hash, and returns a clone of
// tx with a new DecoratedSignature appended. The signing path is identical to
// (*keypair.Full).SignDecorated, just routed through the keystore Signer.
func (s *keystoreTxSigner) SignTransaction(networkPassphrase string, tx *txnbuild.Transaction) (*txnbuild.Transaction, error) {
	envelope := tx.ToXDR()
	h, err := network.HashTransactionInEnvelope(envelope, networkPassphrase)
	if err != nil {
		return nil, fmt.Errorf("hash transaction envelope: %w", err)
	}
	signResp, err := s.ks.Sign(context.Background(), keystore.SignRequest{
		KeyName: s.keyName,
		Data:    h[:],
	})
	if err != nil {
		return nil, fmt.Errorf("keystore sign with %q: %w", s.keyName, err)
	}

	hint := s.hint()
	decorated := xdr.NewDecoratedSignature(signResp.Signature, hint)
	signed, err := tx.AddSignatureDecorated(decorated)
	if err != nil {
		return nil, fmt.Errorf("attach decorated signature: %w", err)
	}
	return signed, nil
}

// hint mirrors (*keypair.Full).Hint: the last 4 bytes of the Ed25519 public key.
// Stellar uses the hint to pick the right signer when multiple keys could sign
// for an account; for a single-signer account it is purely informational.
func (s *keystoreTxSigner) hint() [4]byte {
	var h [4]byte
	copy(h[:], s.publicKey[len(s.publicKey)-4:])
	return h
}
