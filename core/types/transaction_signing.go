// Copyright 2016 The evrynet-node Authors
// This file is part of the evrynet-node library.
//
// The evrynet-node library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The evrynet-node library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the evrynet-node library. If not, see <http://www.gnu.org/licenses/>.

package types

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math/big"

	"github.com/Evrynetlabs/evrynet-node/common"
	"github.com/Evrynetlabs/evrynet-node/crypto"
	"github.com/Evrynetlabs/evrynet-node/params"
)

var (
	ErrInvalidChainId = errors.New("invalid chain id for signer")
)

// sigCache is used to cache the derived sender and contains
// the signer used to derive it.
type sigCache struct {
	signer Signer
	from   common.Address
}

// MakeSigner returns a Signer based on the given chain config and block number.
func MakeSigner(config *params.ChainConfig, blockNumber *big.Int) Signer {
	return NewOmahaSigner(config.ChainID)
}

// ProviderSignTx signs the transaction using the given signer and private key
func ProviderSignTx(tx *Transaction, s Signer, prv *ecdsa.PrivateKey) (*Transaction, error) {
	h, err := s.HashWithSender(tx)
	if err != nil {
		return nil, err
	}
	sig, err := crypto.Sign(h[:], prv)
	if err != nil {
		return nil, err
	}
	return tx.WithProviderSignature(s, sig)
}

// SignTx signs the transaction using the given signer and private key
func SignTx(tx *Transaction, s Signer, prv *ecdsa.PrivateKey) (*Transaction, error) {
	h := s.Hash(tx)
	sig, err := crypto.Sign(h[:], prv)
	if err != nil {
		return nil, err
	}
	return tx.WithSignature(s, sig)
}

// Sender returns the address derived from the signature (V, R, S) using secp256k1
// elliptic curve and an error if it failed deriving or upon an incorrect
// signature.
//
// Sender may cache the address, allowing it to be used regardless of
// signing method. The cache is invalidated if the cached signer does
// not match the signer used in the current call.
func Sender(signer Signer, tx *Transaction) (common.Address, error) {
	if sc := tx.from.Load(); sc != nil {
		sigCache := sc.(sigCache)
		// If the signer used to derive from in a previous
		// call is not the same as used current, invalidate
		// the cache.
		if sigCache.signer.Equal(signer) {
			return sigCache.from, nil
		}
	}

	addr, err := signer.Sender(tx)
	if err != nil {
		return common.Address{}, err
	}
	tx.from.Store(sigCache{signer: signer, from: addr})
	return addr, nil
}

// Provider returns the address derived from the signature (V, R, S) using secp256k1
// If there is no provider signature, it will return nil address pointer and nill error.
func Provider(signer Signer, tx *Transaction) (*common.Address, error) {
	// Not caching provider for now
	// Short circuit
	if (tx.data.PV == nil || tx.data.PV.Cmp(big.NewInt(0)) == 0) &&
		(tx.data.PR == nil || tx.data.PR.Cmp(big.NewInt(0)) == 0) &&
		(tx.data.PS == nil || tx.data.PS.Cmp(big.NewInt(0)) == 0) {
		return nil, nil
	}
	provider, err := signer.Provider(tx)
	if err != nil {
		return nil, err
	}
	// if it recovered as zero address
	if provider == (common.Address{}) {
		return nil, nil
	}
	return &provider, nil
}

// Signer encapsulates transaction signature handling. Note that this interface is not a
// stable API and may change at any time to accommodate new protocol rules.
type Signer interface {
	// Sender returns the sender address of the transaction.
	Sender(tx *Transaction) (common.Address, error)
	// Provider returns the provider address of the transaction
	Provider(tx *Transaction) (common.Address, error)
	// SignatureValues returns the raw R, S, V values corresponding to the
	// given signature.
	SignatureValues(tx *Transaction, sig []byte) (r, s, v *big.Int, err error)
	// Hash returns the hash to be signed.
	Hash(tx *Transaction) common.Hash
	// HashWithSender returns the hash with sender address for provider to sign
	HashWithSender(tx *Transaction) (common.Hash, error)
	// Equal returns true if the given signer is the same as the receiver.
	Equal(Signer) bool
}

type OmahaSigner struct {
	chainId, chainIdMul *big.Int
}

func NewOmahaSigner(chainId *big.Int) OmahaSigner {
	if chainId == nil {
		chainId = new(big.Int)
	}
	return OmahaSigner{
		chainId:    chainId,
		chainIdMul: new(big.Int).Mul(chainId, big.NewInt(2)),
	}
}

func (s OmahaSigner) Equal(s2 Signer) bool {
	omaha, ok := s2.(OmahaSigner)
	return ok && omaha.chainId.Cmp(s.chainId) == 0
}

var big8 = big.NewInt(8)

func (s OmahaSigner) Sender(tx *Transaction) (common.Address, error) {
	if !tx.Protected() {
		return BaseSigner{}.Sender(tx)
	}
	if tx.ChainId().Cmp(s.chainId) != 0 {
		return common.Address{}, ErrInvalidChainId
	}
	V := new(big.Int).Sub(tx.data.V, s.chainIdMul)
	V.Sub(V, big8)
	return recoverPlain(s.Hash(tx), tx.data.R, tx.data.S, V, true)
}

//Provider return the Address of provider based on PV, PS, PR
func (s OmahaSigner) Provider(tx *Transaction) (common.Address, error) {
	if !tx.ProviderProtected() {
		return BaseSigner{}.Provider(tx)
	}
	if tx.ChainId().Cmp(s.chainId) != 0 {
		return common.Address{}, ErrInvalidChainId
	}
	V := new(big.Int).Sub(tx.data.PV, s.chainIdMul)
	V.Sub(V, big8)
	h, err := s.HashWithSender(tx)
	if err != nil {
		return common.Address{}, err
	}
	return recoverPlain(h, tx.data.PR, tx.data.PS, V, true)
}

// SignatureValues returns signature values. This signature
// needs to be in the [R || S || V] format where V is 0 or 1.
func (s OmahaSigner) SignatureValues(tx *Transaction, sig []byte) (R, S, V *big.Int, err error) {
	R, S, V, err = BaseSigner{}.SignatureValues(tx, sig)
	if err != nil {
		return nil, nil, nil, err
	}
	if s.chainId.Sign() != 0 {
		V = big.NewInt(int64(sig[64] + 35))
		V.Add(V, s.chainIdMul)
	}
	return R, S, V, nil
}

// Hash returns the hash to be signed by the sender.
// It does not uniquely identify the transaction.
func (s OmahaSigner) Hash(tx *Transaction) common.Hash {
	if tx.data.Provider == nil && len(tx.data.Extra) == 0 {
		return rlpHash([]interface{}{
			tx.data.AccountNonce,
			tx.data.Price,
			tx.data.GasLimit,
			tx.data.Recipient,
			tx.data.Amount,
			tx.data.Payload,
			s.chainId, uint(0), uint(0),
		})
	}
	return rlpHash([]interface{}{
		tx.data.AccountNonce,
		tx.data.Price,
		tx.data.GasLimit,
		tx.data.Recipient,
		tx.data.Amount,
		tx.data.Payload,
		tx.data.Owner,
		tx.data.Provider,
		tx.data.Extra,
		s.chainId, uint(0), uint(0),
	})
}

// HashWithSender returns the hash with sender address to be signed by the provider.
// It does not uniquely identify the transaction.
func (s OmahaSigner) HashWithSender(tx *Transaction) (common.Hash, error) {
	sender, err := s.Sender(tx)
	if err != nil {
		return common.Hash{}, err
	}

	return rlpHash([]interface{}{
		tx.data.AccountNonce,
		tx.data.Price,
		tx.data.GasLimit,
		tx.data.Recipient,
		tx.data.Amount,
		tx.data.Payload,
		tx.data.Owner,
		tx.data.Provider,
		tx.data.Extra,
		s.chainId, uint(0), uint(0),
		sender,
	}), nil
}

type BaseSigner struct{}

func (s BaseSigner) Equal(s2 Signer) bool {
	_, ok := s2.(BaseSigner)
	return ok
}

// SignatureValues returns signature values. This signature
// needs to be in the [R || S || V] format where V is 0 or 1.
func (bs BaseSigner) SignatureValues(tx *Transaction, sig []byte) (r, s, v *big.Int, err error) {
	if len(sig) != 65 {
		panic(fmt.Sprintf("wrong size for signature: got %d, want 65", len(sig)))
	}
	r = new(big.Int).SetBytes(sig[:32])
	s = new(big.Int).SetBytes(sig[32:64])
	v = new(big.Int).SetBytes([]byte{sig[64] + 27})
	return r, s, v, nil
}

// Hash returns the hash to be signed by the sender.
// It does not uniquely identify the transaction.
func (bs BaseSigner) Hash(tx *Transaction) common.Hash {
	if tx.data.Provider == nil && len(tx.data.Extra) == 0 {
		return rlpHash([]interface{}{
			tx.data.AccountNonce,
			tx.data.Price,
			tx.data.GasLimit,
			tx.data.Recipient,
			tx.data.Amount,
			tx.data.Payload,
		})
	}

	return rlpHash([]interface{}{
		tx.data.AccountNonce,
		tx.data.Price,
		tx.data.GasLimit,
		tx.data.Recipient,
		tx.data.Amount,
		tx.data.Payload,
		tx.data.Owner,
		tx.data.Provider,
		tx.data.Extra,
	})
}

// Hash returns the hash to be signed by the sender.
// It does not uniquely identify the transaction.
func (bs BaseSigner) HashWithSender(tx *Transaction) (common.Hash, error) {
	sender, err := bs.Sender(tx)
	if err != nil {
		return common.Hash{}, nil
	}
	return rlpHash([]interface{}{
		tx.data.AccountNonce,
		tx.data.Price,
		tx.data.GasLimit,
		tx.data.Recipient,
		tx.data.Amount,
		tx.data.Payload,
		tx.data.Owner,
		tx.data.Provider,
		sender,
	}), nil
}

//Provider return the Address of provider based on PV, PS, PR
func (bs BaseSigner) Provider(tx *Transaction) (common.Address, error) {
	h, err := bs.HashWithSender(tx)
	if err != nil {
		return common.Address{}, err
	}
	return recoverPlain(h, tx.data.PR, tx.data.PS, tx.data.PV, true)
}

func (bs BaseSigner) Sender(tx *Transaction) (common.Address, error) {
	return recoverPlain(bs.Hash(tx), tx.data.R, tx.data.S, tx.data.V, true)
}

func recoverPlain(sighash common.Hash, R, S, Vb *big.Int, base bool) (common.Address, error) {
	if Vb == nil || Vb.BitLen() > 8 {
		return common.Address{}, ErrInvalidSig
	}
	V := byte(Vb.Uint64() - 27)
	if !crypto.ValidateSignatureValues(V, R, S, base) {
		return common.Address{}, ErrInvalidSig
	}
	// encode the signature in uncompressed format
	r, s := R.Bytes(), S.Bytes()
	sig := make([]byte, 65)
	copy(sig[32-len(r):32], r)
	copy(sig[64-len(s):64], s)
	sig[64] = V
	// recover the public key from the signature
	pub, err := crypto.Ecrecover(sighash[:], sig)
	if err != nil {
		return common.Address{}, err
	}
	if len(pub) == 0 || pub[0] != 4 {
		return common.Address{}, errors.New("invalid public key")
	}
	var addr common.Address
	copy(addr[:], crypto.Keccak256(pub[1:])[12:])
	return addr, nil
}

// deriveChainId derives the chain id from the given v parameter
func deriveChainId(v *big.Int) *big.Int {
	if v.BitLen() <= 64 {
		v := v.Uint64()
		if v == 27 || v == 28 {
			return new(big.Int)
		}
		return new(big.Int).SetUint64((v - 35) / 2)
	}
	v = new(big.Int).Sub(v, big.NewInt(35))
	return v.Div(v, big.NewInt(2))
}
