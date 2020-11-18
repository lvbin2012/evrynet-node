package evrdb

import (
	"errors"
	"sync"

	"github.com/Evrynetlabs/evrynet-node/common"
)

/*
 * This is a test memory database. Do not use for any production it does not get persisted
 */
type MemDatabase struct {
	db   map[string][]byte
	lock sync.RWMutex
}

func NewMemDatabase() *MemDatabase {
	return &MemDatabase{
		db: make(map[string][]byte),
	}
}

func (db *MemDatabase) Put(key []byte, value []byte) error {
	db.lock.Lock()
	defer db.lock.Unlock()

	db.db[string(key)] = common.CopyBytes(value)
	return nil
}

func (db *MemDatabase) Has(key []byte) (bool, error) {
	panic("implement me")
}

func (db *MemDatabase) Get(key []byte) ([]byte, error) {
	db.lock.RLock()
	defer db.lock.RUnlock()

	if entry, ok := db.db[string(key)]; ok {
		return common.CopyBytes(entry), nil
	}
	return nil, errors.New("not found")
}

func (db *MemDatabase) Keys() [][]byte {
	panic("implement me")
}

func (db *MemDatabase) Delete(key []byte) error {
	panic("implement me")
}

func (db *MemDatabase) NewBatch() Batch {
	panic("implement me")
}

func (db *MemDatabase) Len() int { return len(db.db) }

func (db *MemDatabase) HasAncient(kind string, number uint64) (bool, error) {
	return false, nil
}

func (db *MemDatabase) Ancient(kind string, number uint64) ([]byte, error) {
	return nil, nil
}

func (db *MemDatabase) Ancients() (uint64, error) {
	panic("implement me")
}

func (db *MemDatabase) AncientSize(kind string) (uint64, error) {
	panic("implement me")
}

func (db *MemDatabase) AppendAncient(number uint64, hash, header, body, receipt, td []byte,  isFinalChain bool) error {
	panic("implement me")
}

func (db *MemDatabase) TruncateAncients(n uint64) error {
	panic("implement me")
}

func (db *MemDatabase) Sync() error {
	panic("implement me")
}

func (db *MemDatabase) NewIterator() Iterator {
	panic("implement me")
}

func (db *MemDatabase) NewIteratorWithStart(start []byte) Iterator {
	panic("implement me")
}

func (db *MemDatabase) NewIteratorWithPrefix(prefix []byte) Iterator {
	panic("implement me")
}

func (db *MemDatabase) Stat(property string) (string, error) {
	panic("implement me")
}

func (db *MemDatabase) Compact(start []byte, limit []byte) error {
	panic("implement me")
}

func (db *MemDatabase) Close() error {
	panic("implement me")
}

type kv struct {
	k, v []byte
	del  bool
}

type memBatch struct {
	db     *MemDatabase
	writes []kv
	size   int
}

func (b *memBatch) Replay(w KeyValueWriter) error {
	panic("implement me")
}
