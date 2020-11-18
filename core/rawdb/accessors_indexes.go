// Copyright 2018 The evrynet-node Authors
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

package rawdb

import (
	"math/big"

	"github.com/Evrynetlabs/evrynet-node/common"
	"github.com/Evrynetlabs/evrynet-node/core/types"
	"github.com/Evrynetlabs/evrynet-node/evrdb"
	"github.com/Evrynetlabs/evrynet-node/log"
	"github.com/Evrynetlabs/evrynet-node/params"
	"github.com/Evrynetlabs/evrynet-node/rlp"
)

// ReadTxLookupEntry retrieves the positional metadata associated with a transaction
// hash to allow retrieving the transaction or receipt by hash.
func ReadTxLookupEntry(db evrdb.Reader, hash common.Hash, isFinalChain bool) *uint64 {
	data, _ := db.Get(getFinalKey(txLookupKey(hash), isFinalChain))
	if len(data) == 0 {
		return nil
	}
	// Database v6 tx lookup just stores the block number
	if len(data) < common.HashLength {
		number := new(big.Int).SetBytes(data).Uint64()
		return &number
	}
	// Database v4-v5 tx lookup format just stores the hash
	if len(data) == common.HashLength {
		return ReadHeaderNumber(db, common.BytesToHash(data), isFinalChain)
	}
	// Finally try database v3 tx lookup format
	var entry LegacyTxLookupEntry
	if err := rlp.DecodeBytes(data, &entry); err != nil {
		log.Error("Invalid transaction lookup entry RLP", "hash", hash, "blob", data, "err", err)
		return nil
	}
	return &entry.BlockIndex
}

// WriteTxLookupEntries stores a positional metadata for every transaction from
// a block, enabling hash based transaction and receipt lookups.
func WriteTxLookupEntries(db evrdb.KeyValueWriter, block *types.Block, isFinalChain bool) {
	number := block.Number().Bytes()
	for _, tx := range block.Transactions() {
		if err := db.Put(getFinalKey(txLookupKey(tx.Hash()), isFinalChain), number); err != nil {
			log.Crit("Failed to store transaction lookup entry", "err", err)
		}
	}
}

// DeleteTxLookupEntry removes all transaction data associated with a hash.
func DeleteTxLookupEntry(db evrdb.KeyValueWriter, hash common.Hash, isFinalChain bool) {
	db.Delete(getFinalKey(txLookupKey(hash), isFinalChain))
}

// ReadTransaction retrieves a specific transaction from the database, along with
// its added positional metadata.
func ReadTransaction(db evrdb.Reader, hash common.Hash, isFinalChain bool) (*types.Transaction, common.Hash, uint64, uint64) {
	blockNumber := ReadTxLookupEntry(db, hash, isFinalChain)
	if blockNumber == nil {
		return nil, common.Hash{}, 0, 0
	}
	blockHash := ReadCanonicalHash(db, *blockNumber, isFinalChain)
	if blockHash == (common.Hash{}) {
		return nil, common.Hash{}, 0, 0
	}
	body := ReadBody(db, blockHash, *blockNumber, isFinalChain)
	if body == nil {
		log.Error("Transaction referenced missing", "number", blockNumber, "hash", blockHash)
		return nil, common.Hash{}, 0, 0
	}
	for txIndex, tx := range body.Transactions {
		if tx.Hash() == hash {
			return tx, blockHash, *blockNumber, uint64(txIndex)
		}
	}
	log.Error("Transaction not found", "number", blockNumber, "hash", blockHash, "txhash", hash)
	return nil, common.Hash{}, 0, 0
}

// ReadReceipt retrieves a specific transaction receipt from the database, along with
// its added positional metadata.
func ReadReceipt(db evrdb.Reader, hash common.Hash, config *params.ChainConfig) (*types.Receipt, common.Hash, uint64, uint64) {
	// Retrieve the context of the receipt based on the transaction hash
	blockNumber := ReadTxLookupEntry(db, hash, config.IsFinalChain)
	if blockNumber == nil {
		return nil, common.Hash{}, 0, 0
	}
	blockHash := ReadCanonicalHash(db, *blockNumber, config.IsFinalChain)
	if blockHash == (common.Hash{}) {
		return nil, common.Hash{}, 0, 0
	}
	// Read all the receipts from the block and return the one with the matching hash
	receipts := ReadReceipts(db, blockHash, *blockNumber, config)
	for receiptIndex, receipt := range receipts {
		if receipt.TxHash == hash {
			return receipt, blockHash, *blockNumber, uint64(receiptIndex)
		}
	}
	log.Error("Receipt not found", "number", blockNumber, "hash", blockHash, "txhash", hash)
	return nil, common.Hash{}, 0, 0
}

// ReadBloomBits retrieves the compressed bloom bit vector belonging to the given
// section and bit index from the.
func ReadBloomBits(db evrdb.KeyValueReader, bit uint, section uint64, head common.Hash, isFinalChain bool) ([]byte, error) {
	return db.Get(getFinalKey(bloomBitsKey(bit, section, head), isFinalChain))
}

// WriteBloomBits stores the compressed bloom bits vector belonging to the given
// section and bit index.
func WriteBloomBits(db evrdb.KeyValueWriter, bit uint, section uint64, head common.Hash, bits []byte, isFinalChain bool) {
	if err := db.Put(getFinalKey(bloomBitsKey(bit, section, head), isFinalChain), bits); err != nil {
		log.Crit("Failed to store bloom bits", "err", err)
	}
}
