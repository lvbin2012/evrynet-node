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
	"bytes"
	"encoding/binary"
	"math/big"

	"github.com/Evrynetlabs/evrynet-node/common"
	"github.com/Evrynetlabs/evrynet-node/core/types"
	"github.com/Evrynetlabs/evrynet-node/evrdb"
	"github.com/Evrynetlabs/evrynet-node/log"
	"github.com/Evrynetlabs/evrynet-node/params"
	"github.com/Evrynetlabs/evrynet-node/rlp"
)

// ReadCanonicalHash retrieves the hash assigned to a canonical block number.
func ReadCanonicalHash(db evrdb.Reader, number uint64, isFinalChain bool) common.Hash {
	table := freezerHashTable
	if isFinalChain {
		table = freezerFHashTable
	}
	data, _ := db.Ancient(table, number)
	if len(data) == 0 {
		data, _ = db.Get(getFinalKey(headerHashKey(number), isFinalChain))
		// In the background freezer is moving data from leveldb to flatten files.
		// So during the first check for ancient db, the data is not yet in there,
		// but when we reach into leveldb, the data was already moved. That would
		// result in a not found error.
		if len(data) == 0 {
			data, _ = db.Ancient(table, number)
		}
	}
	if len(data) == 0 {
		return common.Hash{}
	}
	return common.BytesToHash(data)
}

// WriteCanonicalHash stores the hash assigned to a canonical block number.
func WriteCanonicalHash(db evrdb.KeyValueWriter, hash common.Hash, number uint64, isFinalChain bool) {
	key := getFinalKey(headerHashKey(number), isFinalChain)
	if err := db.Put(key, hash.Bytes()); err != nil {
		log.Crit("Failed to store number to hash mapping", "err", err)
	}
}

// DeleteCanonicalHash removes the number to hash canonical mapping.
func DeleteCanonicalHash(db evrdb.KeyValueWriter, number uint64, isFinalChain bool) {
	key := getFinalKey(headerHashKey(number), isFinalChain)
	if err := db.Delete(key); err != nil {
		log.Crit("Failed to delete number to hash mapping", "err", err)
	}
}

// ReadAllHashes retrieves all the hashes assigned to blocks at a certain heights,
// both canonical and reorged forks included.
func ReadAllHashes(db evrdb.Iteratee, number uint64, isFinalChain bool) []common.Hash {
	prefix := getFinalKey(headerKeyPrefix(number), isFinalChain)

	hashes := make([]common.Hash, 0, 1)
	it := db.NewIteratorWithPrefix(prefix)
	defer it.Release()

	for it.Next() {
		if key := it.Key(); len(key) == len(prefix)+32 {
			hashes = append(hashes, common.BytesToHash(key[len(key)-32:]))
		}
	}
	return hashes
}

func ReadHeaderNumber(db evrdb.KeyValueReader, hash common.Hash, isFinalChain bool) *uint64 {
	return ReadHeaderNumberBase(db, hash, isFinalChain, false)
}

func ReadEvilHeaderNumber(db evrdb.KeyValueReader, hash common.Hash, isFinalChain bool) *uint64 {
	return ReadHeaderNumberBase(db, hash, isFinalChain, true)
}

// ReadHeaderNumber returns the header number assigned to a hash.
func ReadHeaderNumberBase(db evrdb.KeyValueReader, hash common.Hash, isFinalChain bool, isEvil bool) *uint64 {
	var keyOri []byte
	if isEvil {
		keyOri = evilHeaderNumberKey(hash)
	} else {
		keyOri = headerNumberKey(hash)
	}
	key := getFinalKey(keyOri, isFinalChain)
	data, _ := db.Get(key)
	if len(data) != 8 {
		return nil
	}
	number := binary.BigEndian.Uint64(data)
	return &number
}

func WriteHeaderNumber(db evrdb.KeyValueWriter, hash common.Hash, number uint64, isFinalChain bool) {
	WriteHeaderNumberBase(db, hash, number, isFinalChain, false)
}

func WriteEvilHeaderNumber(db evrdb.KeyValueWriter, hash common.Hash, number uint64, isFinalChain bool) {
	WriteHeaderNumberBase(db, hash, number, isFinalChain, true)
}

// WriteHeaderNumber stores the hash->number mapping.
func WriteHeaderNumberBase(db evrdb.KeyValueWriter, hash common.Hash, number uint64, isFinalChain bool, isEvil bool) {
	var keyOri []byte
	if isEvil {
		keyOri = evilHeaderNumberKey(hash)
	} else {
		keyOri = headerNumberKey(hash)
	}
	key := getFinalKey(keyOri, isFinalChain)
	enc := encodeBlockNumber(number)
	if err := db.Put(key, enc); err != nil {
		log.Crit("Failed to store hash to number mapping", "err", err)
	}
}

// DeleteHeaderNumber removes hash->number mapping.
func DeleteHeaderNumber(db evrdb.KeyValueWriter, hash common.Hash, isFinalChain bool) {
	key := getFinalKey(headerNumberKey(hash), isFinalChain)
	if err := db.Delete(key); err != nil {
		log.Crit("Failed to delete hash to number mapping", "err", err)
	}
}

// ReadHeadHeaderHash retrieves the hash of the current canonical head header.
func ReadHeadHeaderHash(db evrdb.KeyValueReader, isFinalChain bool) common.Hash {
	data, _ := db.Get(getFinalKey(headHeaderKey, isFinalChain))
	if len(data) == 0 {
		return common.Hash{}
	}
	return common.BytesToHash(data)
}

// WriteHeadHeaderHash stores the hash of the current canonical head header.
func WriteHeadHeaderHash(db evrdb.KeyValueWriter, hash common.Hash, isFinalChain bool) {
	if err := db.Put(getFinalKey(headHeaderKey, isFinalChain), hash.Bytes()); err != nil {
		log.Crit("Failed to store last header's hash", "err", err)
	}
}

// ReadHeadBlockHash retrieves the hash of the current canonical head block.
func ReadHeadBlockHash(db evrdb.KeyValueReader, isFinalChain bool) common.Hash {
	data, _ := db.Get(getFinalKey(headBlockKey, isFinalChain))
	if len(data) == 0 {
		return common.Hash{}
	}
	return common.BytesToHash(data)
}

// WriteHeadBlockHash stores the head block's hash.
func WriteHeadBlockHash(db evrdb.KeyValueWriter, hash common.Hash, isFinalChain bool) {
	if err := db.Put(getFinalKey(headBlockKey, isFinalChain), hash.Bytes()); err != nil {
		log.Crit("Failed to store last block's hash", "err", err)
	}
}

// ReadHeadFastBlockHash retrieves the hash of the current fast-sync head block.
func ReadHeadFastBlockHash(db evrdb.KeyValueReader, isFinalChain bool) common.Hash {
	data, _ := db.Get(getFinalKey(headFastBlockKey, isFinalChain))
	if len(data) == 0 {
		return common.Hash{}
	}
	return common.BytesToHash(data)
}

// WriteHeadFastBlockHash stores the hash of the current fast-sync head block.
func WriteHeadFastBlockHash(db evrdb.KeyValueWriter, hash common.Hash, isFinalChain bool) {
	if err := db.Put(getFinalKey(headFastBlockKey, isFinalChain), hash.Bytes()); err != nil {
		log.Crit("Failed to store last fast block's hash", "err", err)
	}
}

// ReadFastTrieProgress retrieves the number of tries nodes fast synced to allow
// reporting correct numbers across restarts.
func ReadFastTrieProgress(db evrdb.KeyValueReader, isFinalChain bool) uint64 {
	data, _ := db.Get(getFinalKey(fastTrieProgressKey, isFinalChain))
	if len(data) == 0 {
		return 0
	}
	return new(big.Int).SetBytes(data).Uint64()
}

// WriteFastTrieProgress stores the fast sync trie process counter to support
// retrieving it across restarts.
func WriteFastTrieProgress(db evrdb.KeyValueWriter, count uint64, isFinalChain bool) {
	if err := db.Put(getFinalKey(fastTrieProgressKey, isFinalChain), new(big.Int).SetUint64(count).Bytes()); err != nil {
		log.Crit("Failed to store fast sync trie progress", "err", err)
	}
}
func ReadHeaderRLP(db evrdb.Reader, hash common.Hash, number uint64, isFinalChain bool) rlp.RawValue {
	return ReadHeaderRLPBase(db, hash, number, isFinalChain, false)
}

func ReadEvilHeaderRLP(db evrdb.Reader, hash common.Hash, number uint64, isFinalChain bool) rlp.RawValue {
	return ReadHeaderRLPBase(db, hash, number, isFinalChain, true)
}

// ReadHeaderRLP retrieves a block header in its raw RLP database encoding.
func ReadHeaderRLPBase(db evrdb.Reader, hash common.Hash, number uint64, isFinalChain bool, isEvil bool) rlp.RawValue {
	if isEvil {
		data, _ := db.Get(getFinalKey(evilHeaderKey(number, hash), isFinalChain))
		return data
	}

	table := freezerHeaderTable
	if isFinalChain {
		table = freezerFHeaderTable
	}
	data, _ := db.Ancient(table, number)
	if len(data) == 0 {
		data, _ = db.Get(getFinalKey(headerKey(number, hash), isFinalChain))
		// In the background freezer is moving data from leveldb to flatten files.
		// So during the first check for ancient db, the data is not yet in there,
		// but when we reach into leveldb, the data was already moved. That would
		// result in a not found error.
		if len(data) == 0 {
			data, _ = db.Ancient(table, number)
		}
	}
	return data
}

func HasHeader(db evrdb.Reader, hash common.Hash, number uint64, isFinalChain bool) bool {
	return HasHeaderBase(db, hash, number, isFinalChain, false)
}

func HasEvilHeader(db evrdb.Reader, hash common.Hash, number uint64, isFinalChain bool) bool {
	return HasHeaderBase(db, hash, number, isFinalChain, true)
}

// HasHeader verifies the existence of a block header corresponding to the hash.
func HasHeaderBase(db evrdb.Reader, hash common.Hash, number uint64, isFinalChain bool, isEvil bool) bool {
	table := freezerHashTable
	if isFinalChain {
		table = freezerFHashTable
	}
	if !isEvil {
		if has, err := db.Ancient(table, number); err == nil && common.BytesToHash(has) == hash {
			return true
		}
	}
	var keyInput []byte
	if isEvil {
		keyInput = evilHeaderKey(number, hash)
	} else {
		keyInput = headerKey(number, hash)
	}
	if has, err := db.Has(getFinalKey(keyInput, isFinalChain)); !has || err != nil {
		return false
	}
	return true
}

func ReadHeader(db evrdb.Reader, hash common.Hash, number uint64, isFinalChain bool) *types.Header {
	return ReadHeaderBase(db, hash, number, isFinalChain, false)
}

func ReadEvilHeader(db evrdb.Reader, hash common.Hash, number uint64, isFinalChain bool) *types.Header {
	return ReadHeaderBase(db, hash, number, isFinalChain, true)
}

// ReadHeader retrieves the block header corresponding to the hash.
func ReadHeaderBase(db evrdb.Reader, hash common.Hash, number uint64, isFinalChain bool, isEvil bool) *types.Header {
	data := ReadHeaderRLPBase(db, hash, number, isFinalChain, isEvil)
	if len(data) == 0 {
		return nil
	}
	header := new(types.Header)
	if err := rlp.Decode(bytes.NewReader(data), header); err != nil {
		log.Error("Invalid block header RLP", "hash", hash, "err", err)
		return nil
	}
	return header
}

func WriteHeader(db evrdb.KeyValueWriter, header *types.Header, isFinalChain bool) {
	WriteHeaderBase(db, header, isFinalChain, false)
}

func WriteEvilHeader(db evrdb.KeyValueWriter, header *types.Header, isFinalChain bool) {
	WriteHeaderBase(db, header, isFinalChain, true)
}

// WriteHeader stores a block header into the database and also stores the hash-
// to-number mapping.
func WriteHeaderBase(db evrdb.KeyValueWriter, header *types.Header, isFinalChain bool, isEvil bool) {
	var (
		hash   = header.Hash()
		number = header.Number.Uint64()
	)
	// Write the hash -> number mapping
	WriteHeaderNumberBase(db, hash, number, isFinalChain, isEvil)

	// Write the encoded header
	data, err := rlp.EncodeToBytes(header)
	if err != nil {
		log.Crit("Failed to RLP encode header", "err", err)
	}
	var keyOri []byte
	if isEvil {
		keyOri = evilHeaderKey(number, hash)
	} else {
		keyOri = headerKey(number, hash)
	}

	key := getFinalKey(keyOri, isFinalChain)
	if err := db.Put(key, data); err != nil {
		log.Crit("Failed to store header", "err", err)
	}
}
func DeleteHeader(db evrdb.KeyValueWriter, hash common.Hash, number uint64, isFinalChain bool) {
	DeleteHeaderBase(db, hash, number, isFinalChain, false)
}

func DeleteEvilHeader(db evrdb.KeyValueWriter, hash common.Hash, number uint64, isFinalChain bool) {
	DeleteHeaderBase(db, hash, number, isFinalChain, true)
}

// DeleteHeader removes all block header data associated with a hash.
func DeleteHeaderBase(db evrdb.KeyValueWriter, hash common.Hash, number uint64, isFinalChain bool, isEvil bool) {
	deleteHeaderWithoutNumberBase(db, hash, number, isFinalChain, isEvil)
	var keyOri []byte
	if isEvil {
		keyOri = evilHeaderNumberKey(hash)
	} else {
		keyOri = headerNumberKey(hash)
	}
	if err := db.Delete(getFinalKey(keyOri, isFinalChain)); err != nil {
		log.Crit("Failed to delete hash to number mapping", "err", err)
	}
}

func deleteHeaderWithoutNumber(db evrdb.KeyValueWriter, hash common.Hash, number uint64, isFinalChain bool) {
	deleteHeaderWithoutNumberBase(db, hash, number, isFinalChain, false)
}

func deleteEvilHeaderWithoutNumber(db evrdb.KeyValueWriter, hash common.Hash, number uint64, isFinalChain bool) {
	deleteHeaderWithoutNumberBase(db, hash, number, isFinalChain, true)
}

// deleteHeaderWithoutNumber removes only the block header but does not remove
// the hash to number mapping.
func deleteHeaderWithoutNumberBase(db evrdb.KeyValueWriter, hash common.Hash, number uint64, isFinalChain bool, isEvil bool) {
	var keyOri []byte
	if isEvil {
		keyOri = evilHeaderKey(number, hash)
	} else {
		keyOri = headerKey(number, hash)
	}
	if err := db.Delete(getFinalKey(keyOri, isFinalChain)); err != nil {
		log.Crit("Failed to delete header", "err", err)
	}
}

func ReadBodyRLP(db evrdb.Reader, hash common.Hash, number uint64, isFinalChain bool) rlp.RawValue {
	return ReadBodyRLPBase(db, hash, number, isFinalChain, false)
}

func ReadEvilBodyRLP(db evrdb.Reader, hash common.Hash, number uint64, isFinalChain bool) rlp.RawValue {
	return ReadBodyRLPBase(db, hash, number, isFinalChain, true)
}

// ReadBodyRLP retrieves the block body (transactions and uncles) in RLP encoding.
func ReadBodyRLPBase(db evrdb.Reader, hash common.Hash, number uint64, isFinalChain bool, isEvil bool) rlp.RawValue {
	if isEvil {
		data, _ := db.Get(getFinalKey(evilBlockBodyKey(number, hash), isFinalChain))
		return data
	}
	table := freezerBodiesTable
	if isFinalChain {
		table = freezerFBodiesTable
	}
	data, _ := db.Ancient(table, number)
	if len(data) == 0 {
		data, _ = db.Get(getFinalKey(blockBodyKey(number, hash), isFinalChain))
		// In the background freezer is moving data from leveldb to flatten files.
		// So during the first check for ancient db, the data is not yet in there,
		// but when we reach into leveldb, the data was already moved. That would
		// result in a not found error.
		if len(data) == 0 {
			data, _ = db.Ancient(table, number)
		}
	}
	return data
}

func WriteBodyRLP(db evrdb.KeyValueWriter, hash common.Hash, number uint64, rlp rlp.RawValue, isFinalChain bool) {
	WriteBodyRLPBase(db, hash, number, rlp, isFinalChain, false)
}

func WriteEvilBodyRLP(db evrdb.KeyValueWriter, hash common.Hash, number uint64, rlp rlp.RawValue, isFinalChain bool) {
	WriteBodyRLPBase(db, hash, number, rlp, isFinalChain, true)
}

// WriteBodyRLP stores an RLP encoded block body into the database.
func WriteBodyRLPBase(db evrdb.KeyValueWriter, hash common.Hash, number uint64, rlp rlp.RawValue, isFinalChain bool, isEvil bool) {
	var keyOri []byte
	if isEvil {
		keyOri = evilBlockBodyKey(number, hash)
	} else {
		keyOri = blockBodyKey(number, hash)
	}
	if err := db.Put(getFinalKey(keyOri, isFinalChain), rlp); err != nil {
		log.Crit("Failed to store block body", "err", err)
	}
}

func HasBody(db evrdb.Reader, hash common.Hash, number uint64, isFinalChain bool) bool {
	return HasHeaderBase(db, hash, number, isFinalChain, false)
}

func HasEvilBody(db evrdb.Reader, hash common.Hash, number uint64, isFinalChain bool) bool {
	return HasHeaderBase(db, hash, number, isFinalChain, true)
}

// HasBody verifies the existence of a block body corresponding to the hash.
func HasBodyBase(db evrdb.Reader, hash common.Hash, number uint64, isFinalChain bool, isEvil bool) bool {
	if isEvil {
		if has, err := db.Has(getFinalKey(evilBlockBodyKey(number, hash), isFinalChain)); !has || err != nil {
			return false
		}
		return true
	}
	table := freezerHashTable
	if isFinalChain {
		table = freezerFHashTable
	}
	if has, err := db.Ancient(table, number); err == nil && common.BytesToHash(has) == hash {
		return true
	}
	if has, err := db.Has(getFinalKey(blockBodyKey(number, hash), isFinalChain)); !has || err != nil {
		return false
	}
	return true
}

func ReadBody(db evrdb.Reader, hash common.Hash, number uint64, isFinalChain bool) *types.Body {
	return ReadBodyBase(db, hash, number, isFinalChain, false)
}

func ReadEvilBody(db evrdb.Reader, hash common.Hash, number uint64, isFinalChain bool) *types.Body {
	return ReadBodyBase(db, hash, number, isFinalChain, true)
}

// ReadBody retrieves the block body corresponding to the hash.
func ReadBodyBase(db evrdb.Reader, hash common.Hash, number uint64, isFinalChain bool, isEvil bool) *types.Body {
	data := ReadBodyRLPBase(db, hash, number, isFinalChain, isEvil)
	if len(data) == 0 {
		return nil
	}
	body := new(types.Body)
	if err := rlp.Decode(bytes.NewReader(data), body); err != nil {
		log.Error("Invalid block body RLP", "hash", hash, "err", err)
		return nil
	}
	return body
}

func WriteBody(db evrdb.KeyValueWriter, hash common.Hash, number uint64, body *types.Body, isFinalChain bool) {
	WriteBodyBase(db, hash, number, body, isFinalChain, false)
}

func WriteEvilBody(db evrdb.KeyValueWriter, hash common.Hash, number uint64, body *types.Body, isFinalChain bool) {
	WriteBodyBase(db, hash, number, body, isFinalChain, true)
}

// WriteBody stores a block body into the database.
func WriteBodyBase(db evrdb.KeyValueWriter, hash common.Hash, number uint64, body *types.Body, isFinalChain bool, isEvil bool) {
	data, err := rlp.EncodeToBytes(body)
	if err != nil {
		log.Crit("Failed to RLP encode body", "err", err)
	}
	WriteBodyRLPBase(db, hash, number, data, isFinalChain, isEvil)
}

func DeleteBody(db evrdb.KeyValueWriter, hash common.Hash, number uint64, isFinalChain bool) {
	DeleteBodyBase(db, hash, number, isFinalChain, false)
}

func DeleteEvilBody(db evrdb.KeyValueWriter, hash common.Hash, number uint64, isFinalChain bool) {
	DeleteBodyBase(db, hash, number, isFinalChain, true)
}

// DeleteBody removes all block body data associated with a hash.
func DeleteBodyBase(db evrdb.KeyValueWriter, hash common.Hash, number uint64, isFinalChain bool, isEvil bool) {
	if err := db.Delete(getFinalKey(blockBodyKey(number, hash), isFinalChain)); err != nil {
		log.Crit("Failed to delete block body", "err", err)
	}
}

func ReadTdRLP(db evrdb.Reader, hash common.Hash, number uint64, isFinalChain bool) rlp.RawValue {
	return ReadBodyRLPBase(db, hash, number, isFinalChain, false)
}

func ReadEvilTdRLP(db evrdb.Reader, hash common.Hash, number uint64, isFinalChain bool) rlp.RawValue {
	return ReadBodyRLPBase(db, hash, number, isFinalChain, true)
}

// ReadTdRLP retrieves a block's total difficulty corresponding to the hash in RLP encoding.
func ReadTdRLPBase(db evrdb.Reader, hash common.Hash, number uint64, isFinalChain bool, isEvil bool) rlp.RawValue {
	if isEvil {
		data, _ := db.Get(getFinalKey(evilHeaderTDKey(number, hash), isFinalChain))
		return data
	}
	table := freezerDifficultyTable
	if isFinalChain {
		table = freezerFDifficultyTable
	}
	data, _ := db.Ancient(table, number)
	if len(data) == 0 {
		data, _ = db.Get(getFinalKey(headerTDKey(number, hash), isFinalChain))
		// In the background freezer is moving data from leveldb to flatten files.
		// So during the first check for ancient db, the data is not yet in there,
		// but when we reach into leveldb, the data was already moved. That would
		// result in a not found error.
		if len(data) == 0 {
			data, _ = db.Ancient(table, number)
		}
	}
	return data
}

func ReadTd(db evrdb.Reader, hash common.Hash, number uint64, isFinalChain bool) *big.Int {
	return ReadTdBase(db, hash, number, isFinalChain, false)
}

func ReadEvilTd(db evrdb.Reader, hash common.Hash, number uint64, isFinalChain bool) *big.Int {
	return ReadTdBase(db, hash, number, isFinalChain, true)
}

// ReadTd retrieves a block's total difficulty corresponding to the hash.
func ReadTdBase(db evrdb.Reader, hash common.Hash, number uint64, isFinalChain bool, isEvil bool) *big.Int {
	data := ReadTdRLPBase(db, hash, number, isFinalChain, isEvil)
	if len(data) == 0 {
		return nil
	}
	td := new(big.Int)
	if err := rlp.Decode(bytes.NewReader(data), td); err != nil {
		log.Error("Invalid block total difficulty RLP", "hash", hash, "err", err)
		return nil
	}
	return td
}

func WriteTd(db evrdb.KeyValueWriter, hash common.Hash, number uint64, td *big.Int, isFinalChain bool) {
	WriteTdBase(db, hash, number, td, isFinalChain, false)
}

func WriteEvilTd(db evrdb.KeyValueWriter, hash common.Hash, number uint64, td *big.Int, isFinalChain bool) {
	WriteTdBase(db, hash, number, td, isFinalChain, true)
}

// WriteTd stores the total difficulty of a block into the database.
func WriteTdBase(db evrdb.KeyValueWriter, hash common.Hash, number uint64, td *big.Int, isFinalChain bool, isEvil bool) {
	data, err := rlp.EncodeToBytes(td)
	if err != nil {
		log.Crit("Failed to RLP encode block total difficulty", "err", err)
	}
	var keyOri []byte
	if isEvil {
		keyOri = evilHeaderTDKey(number, hash)
	} else {
		keyOri = headerTDKey(number, hash)
	}
	key := getFinalKey(keyOri, isFinalChain)
	if err := db.Put(key, data); err != nil {
		log.Crit("Failed to store block total difficulty", "err", err)
	}
}

func DeleteTd(db evrdb.KeyValueWriter, hash common.Hash, number uint64, isFinalChain bool) {
	DeleteTdBase(db, hash, number, isFinalChain, false)
}

func DeleteEvilTd(db evrdb.KeyValueWriter, hash common.Hash, number uint64, isFinalChain bool) {
	DeleteTdBase(db, hash, number, isFinalChain, true)
}

// DeleteTd removes all block total difficulty data associated with a hash.
func DeleteTdBase(db evrdb.KeyValueWriter, hash common.Hash, number uint64, isFinalChain bool, isEvil bool) {
	var keyOri []byte
	if isEvil {
		keyOri = evilHeaderTDKey(number, hash)
	} else {
		keyOri = headerTDKey(number, hash)
	}
	if err := db.Delete(getFinalKey(keyOri, isFinalChain)); err != nil {
		log.Crit("Failed to delete block total difficulty", "err", err)
	}
}

func HasReceipts(db evrdb.Reader, hash common.Hash, number uint64, isFinalChain bool) bool {
	return HasReceiptsBase(db, hash, number, isFinalChain, false)
}

func HasEvilReceipts(db evrdb.Reader, hash common.Hash, number uint64, isFinalChain bool) bool {
	return HasReceiptsBase(db, hash, number, isFinalChain, true)
}

// HasReceipts verifies the existence of all the transaction receipts belonging
// to a block.
func HasReceiptsBase(db evrdb.Reader, hash common.Hash, number uint64, isFinalChain bool, isEvil bool) bool {
	if isEvil {
		if has, err := db.Has(getFinalKey(evilBlockReceiptsKey(number, hash), isFinalChain)); !has || err != nil {
			return false
		}
		return true
	}
	table := freezerHashTable
	if isFinalChain {
		table = freezerFHashTable
	}
	if has, err := db.Ancient(table, number); err == nil && common.BytesToHash(has) == hash {
		return true
	}
	if has, err := db.Has(getFinalKey(blockReceiptsKey(number, hash), isFinalChain)); !has || err != nil {
		return false
	}
	return true
}

func ReadReceiptsRLP(db evrdb.Reader, hash common.Hash, number uint64, isFinalChain bool) rlp.RawValue {
	return ReadReceiptsRLPBase(db, hash, number, isFinalChain, false)
}

func ReadEvilReceiptsRLP(db evrdb.Reader, hash common.Hash, number uint64, isFinalChain bool) rlp.RawValue {
	return ReadReceiptsRLPBase(db, hash, number, isFinalChain, true)
}

// ReadReceiptsRLP retrieves all the transaction receipts belonging to a block in RLP encoding.
func ReadReceiptsRLPBase(db evrdb.Reader, hash common.Hash, number uint64, isFinalChain bool, isEvil bool) rlp.RawValue {
	if isEvil {
		data, _ := db.Get(getFinalKey(blockReceiptsKey(number, hash), isFinalChain))
		return data
	}
	table := freezerReceiptTable
	if isFinalChain {
		table = freezerFReceiptTable
	}
	data, _ := db.Ancient(table, number)
	if len(data) == 0 {
		data, _ = db.Get(getFinalKey(blockReceiptsKey(number, hash), isFinalChain))
		// In the background freezer is moving data from leveldb to flatten files.
		// So during the first check for ancient db, the data is not yet in there,
		// but when we reach into leveldb, the data was already moved. That would
		// result in a not found error.
		if len(data) == 0 {
			data, _ = db.Ancient(table, number)
		}
	}
	return data
}

func ReadRawReceipts(db evrdb.Reader, hash common.Hash, number uint64, isFinalChain bool) types.Receipts {
	return ReadRawReceiptsBase(db, hash, number, isFinalChain, false)
}

func ReadRawEvilReceipts(db evrdb.Reader, hash common.Hash, number uint64, isFinalChain bool) types.Receipts {
	return ReadRawReceiptsBase(db, hash, number, isFinalChain, true)
}

// ReadRawReceipts retrieves all the transaction receipts belonging to a block.
// The receipt metadata fields are not guaranteed to be populated, so they
// should not be used. Use ReadReceipts instead if the metadata is needed.
func ReadRawReceiptsBase(db evrdb.Reader, hash common.Hash, number uint64, isFinalChain bool, isEvil bool) types.Receipts {
	// Retrieve the flattened receipt slice
	data := ReadReceiptsRLPBase(db, hash, number, isFinalChain, isEvil)
	if len(data) == 0 {
		return nil
	}
	// Convert the receipts from their storage form to their internal representation
	storageReceipts := []*types.ReceiptForStorage{}
	if err := rlp.DecodeBytes(data, &storageReceipts); err != nil {
		log.Error("Invalid receipt array RLP", "hash", hash, "err", err)
		return nil
	}
	receipts := make(types.Receipts, len(storageReceipts))
	for i, storageReceipt := range storageReceipts {
		receipts[i] = (*types.Receipt)(storageReceipt)
	}
	return receipts
}

func ReadReceipts(db evrdb.Reader, hash common.Hash, number uint64, config *params.ChainConfig) types.Receipts {
	return ReadReceiptsBase(db, hash, number, config, false)
}

func ReadEvilReceipts(db evrdb.Reader, hash common.Hash, number uint64, config *params.ChainConfig) types.Receipts {
	return ReadReceiptsBase(db, hash, number, config, true)
}

// ReadReceipts retrieves all the transaction receipts belonging to a block, including
// its correspoinding metadata fields. If it is unable to populate these metadata
// fields then nil is returned.
//
// The current implementation populates these metadata fields by reading the receipts'
// corresponding block body, so if the block body is not found it will return nil even
// if the receipt itself is stored.
func ReadReceiptsBase(db evrdb.Reader, hash common.Hash, number uint64, config *params.ChainConfig, isEvil bool) types.Receipts {
	// We're deriving many fields from the block body, retrieve beside the receipt
	receipts := ReadRawReceiptsBase(db, hash, number, config.IsFinalChain, isEvil)
	if receipts == nil {
		return nil
	}
	body := ReadBodyBase(db, hash, number, config.IsFinalChain, isEvil)
	if body == nil {
		log.Error("Missing body but have receipt", "hash", hash, "number", number)
		return nil
	}
	if err := receipts.DeriveFields(config, hash, number, body.Transactions); err != nil {
		log.Error("Failed to derive block receipts fields", "hash", hash, "number", number, "err", err)
		return nil
	}
	return receipts
}

func WriteReceipts(db evrdb.KeyValueWriter, hash common.Hash, number uint64, receipts types.Receipts, isFinalChain bool) {
	WriteReceiptsBase(db, hash, number, receipts, isFinalChain, false)
}

func WriteEvilReceipts(db evrdb.KeyValueWriter, hash common.Hash, number uint64, receipts types.Receipts, isFinalChain bool) {
	WriteReceiptsBase(db, hash, number, receipts, isFinalChain, true)
}

// WriteReceipts stores all the transaction receipts belonging to a block.
func WriteReceiptsBase(db evrdb.KeyValueWriter, hash common.Hash, number uint64, receipts types.Receipts,
	isFinalChain bool, isEvil bool) {
	// Convert the receipts into their storage form and serialize them
	storageReceipts := make([]*types.ReceiptForStorage, len(receipts))
	for i, receipt := range receipts {
		storageReceipts[i] = (*types.ReceiptForStorage)(receipt)
	}
	bytes, err := rlp.EncodeToBytes(storageReceipts)
	if err != nil {
		log.Crit("Failed to encode block receipts", "err", err)
	}
	var keyOri []byte
	if isEvil {
		keyOri = evilBlockReceiptsKey(number, hash)
	} else {
		keyOri = blockReceiptsKey(number, hash)
	}
	// Store the flattened receipt slice
	if err := db.Put(getFinalKey(keyOri, isFinalChain), bytes); err != nil {
		log.Crit("Failed to store block receipts", "err", err)
	}
}

func DeleteReceipts(db evrdb.KeyValueWriter, hash common.Hash, number uint64, isFinalChain bool) {
	DeleteReceiptsBase(db, hash, number, isFinalChain, false)
}

func DeleteEvilReceipts(db evrdb.KeyValueWriter, hash common.Hash, number uint64, isFinalChain bool) {
	DeleteReceiptsBase(db, hash, number, isFinalChain, true)
}

// DeleteReceipts removes all receipt data associated with a block hash.
func DeleteReceiptsBase(db evrdb.KeyValueWriter, hash common.Hash, number uint64, isFinalChain bool, isEvil bool) {
	var keyOri []byte
	if isEvil {
		keyOri = evilBlockReceiptsKey(number, hash)
	} else {
		keyOri = blockReceiptsKey(number, hash)
	}
	if err := db.Delete(getFinalKey(keyOri, isFinalChain)); err != nil {
		log.Crit("Failed to delete block receipts", "err", err)
	}
}

func ReadBlock(db evrdb.Reader, hash common.Hash, number uint64, isFinalChain bool) *types.Block {
	return ReadBlockBase(db, hash, number, isFinalChain, false)
}

func ReadEvilBlock(db evrdb.Reader, hash common.Hash, number uint64, isFinalChain bool) *types.Block {
	return ReadBlockBase(db, hash, number, isFinalChain, true)
}

// ReadBlock retrieves an entire block corresponding to the hash, assembling it
// back from the stored header and body. If either the header or body could not
// be retrieved nil is returned.
//
// Note, due to concurrent download of header and block body the header and thus
// canonical hash can be stored in the database but the body data not (yet).
func ReadBlockBase(db evrdb.Reader, hash common.Hash, number uint64, isFinalChain bool, isEvil bool) *types.Block {
	header := ReadHeaderBase(db, hash, number, isFinalChain, isEvil)
	if header == nil {
		return nil
	}
	body := ReadBodyBase(db, hash, number, isFinalChain, isEvil)
	if body == nil {
		return nil
	}
	return types.NewBlockWithHeader(header).WithBody(body.Transactions, body.Uncles)
}

func WriteBlock(db evrdb.KeyValueWriter, block *types.Block, isFinalChain bool) {
	WriteBlockBase(db, block, isFinalChain, false)
}

func WriteEvilBlock(db evrdb.KeyValueWriter, block *types.Block, isFinalChain bool) {
	WriteBlockBase(db, block, isFinalChain, true)
}

// WriteBlock serializes a block into the database, header and body separately.
func WriteBlockBase(db evrdb.KeyValueWriter, block *types.Block, isFinalChain bool, isEvil bool) {
	WriteBodyBase(db, block.Hash(), block.NumberU64(), block.Body(), isFinalChain, isEvil)
	WriteHeaderBase(db, block.Header(), isFinalChain, isEvil)
}

// WriteAncientBlock writes entire block data into ancient store and returns the total written size.
func WriteAncientBlock(db evrdb.AncientWriter, block *types.Block, receipts types.Receipts, td *big.Int, isFinalChain bool) int {
	// Encode all block components to RLP format.
	headerBlob, err := rlp.EncodeToBytes(block.Header())
	if err != nil {
		log.Crit("Failed to RLP encode block header", "err", err)
	}
	bodyBlob, err := rlp.EncodeToBytes(block.Body())
	if err != nil {
		log.Crit("Failed to RLP encode body", "err", err)
	}
	storageReceipts := make([]*types.ReceiptForStorage, len(receipts))
	for i, receipt := range receipts {
		storageReceipts[i] = (*types.ReceiptForStorage)(receipt)
	}
	receiptBlob, err := rlp.EncodeToBytes(storageReceipts)
	if err != nil {
		log.Crit("Failed to RLP encode block receipts", "err", err)
	}
	tdBlob, err := rlp.EncodeToBytes(td)
	if err != nil {
		log.Crit("Failed to RLP encode block total difficulty", "err", err)
	}
	// Write all blob to flatten files.
	err = db.AppendAncient(block.NumberU64(), block.Hash().Bytes(), headerBlob, bodyBlob, receiptBlob, tdBlob, isFinalChain)
	if err != nil {
		log.Crit("Failed to write block data to ancient store", "err", err)
	}
	return len(headerBlob) + len(bodyBlob) + len(receiptBlob) + len(tdBlob) + common.HashLength
}

func DeleteBlock(db evrdb.KeyValueWriter, hash common.Hash, number uint64, isFinalChain bool) {
	DeleteBlockBase(db, hash, number, isFinalChain, false)
}

func DeleteEvilBlock(db evrdb.KeyValueWriter, hash common.Hash, number uint64, isFinalChain bool) {
	DeleteBlockBase(db, hash, number, isFinalChain, true)
}

// DeleteBlock removes all block data associated with a hash.
func DeleteBlockBase(db evrdb.KeyValueWriter, hash common.Hash, number uint64, isFinalChain bool, isEvil bool) {
	DeleteReceiptsBase(db, hash, number, isFinalChain, isEvil)
	DeleteHeaderBase(db, hash, number, isFinalChain, isEvil)
	DeleteBodyBase(db, hash, number, isFinalChain, isEvil)
	DeleteTdBase(db, hash, number, isFinalChain, isEvil)
}

func DeleteBlockWithoutNumber(db evrdb.KeyValueWriter, hash common.Hash, number uint64, isFinalChain bool) {
	DeleteBlockWithoutNumberBase(db, hash, number, isFinalChain, false)
}

func DeleteEvilBlockWithoutNumber(db evrdb.KeyValueWriter, hash common.Hash, number uint64, isFinalChain bool) {
	DeleteBlockWithoutNumberBase(db, hash, number, isFinalChain, true)
}

// DeleteBlockWithoutNumber removes all block data associated with a hash, except
// the hash to number mapping.
func DeleteBlockWithoutNumberBase(db evrdb.KeyValueWriter, hash common.Hash, number uint64, isFinalChain bool, isEvil bool) {
	DeleteReceiptsBase(db, hash, number, isFinalChain, isEvil)
	deleteHeaderWithoutNumberBase(db, hash, number, isFinalChain, isEvil)
	DeleteBodyBase(db, hash, number, isFinalChain, isEvil)
	DeleteTdBase(db, hash, number, isFinalChain, isEvil)
}

// FindCommonAncestor returns the last common ancestor of two block headers
func FindCommonAncestor(db evrdb.Reader, a, b *types.Header, isFinalChain bool) *types.Header {
	for bn := b.Number.Uint64(); a.Number.Uint64() > bn; {
		a = ReadHeader(db, a.ParentHash, a.Number.Uint64()-1, isFinalChain)
		if a == nil {
			return nil
		}
	}
	for an := a.Number.Uint64(); an < b.Number.Uint64(); {
		b = ReadHeader(db, b.ParentHash, b.Number.Uint64()-1, isFinalChain)
		if b == nil {
			return nil
		}
	}
	for a.Hash() != b.Hash() {
		a = ReadHeader(db, a.ParentHash, a.Number.Uint64()-1, isFinalChain)
		if a == nil {
			return nil
		}
		b = ReadHeader(db, b.ParentHash, b.Number.Uint64()-1, isFinalChain)
		if b == nil {
			return nil
		}
	}
	return a
}
