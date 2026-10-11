package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// Record header:
//
// op        : 1 byte
// keyLen    : 4 bytes
// valueType : 1 byte
// valueLen  : 4 bytes
//
// Total header size = 10 bytes.
const headerSize = 10

type op byte

const (
	opPut    op = 1
	opDelete op = 2
)

type valueType byte

const (
	typeNone   valueType = 0
	typeInt    valueType = 1
	typeInt32  valueType = 2
	typeString valueType = 3
	typeChar   valueType = 4
	typeBool   valueType = 5
)

func (t valueType) String() string {
	switch t {
	case typeNone:
		return "NONE"
	case typeInt:
		return "INT"
	case typeInt32:
		return "INT32"
	case typeString:
		return "STRING"
	case typeChar:
		return "CHAR"
	case typeBool:
		return "BOOL"
	default:
		return "UNKNOWN"
	}
}

var (
	ErrNotFound  = errors.New("key not found")
	ErrWrongType = errors.New("value type mismatch")
)

// KV is a tiny append-only typed key-value storage engine.
type KV struct {
	mu    sync.RWMutex
	path  string
	file  *os.File
	index map[string]int64 // key -> offset of latest PUT record
	size  int64            // current logical file size
}

// Open opens or creates the database file.
func Open(path string) (*KV, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}

	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}

	kv := &KV{
		path:  path,
		file:  file,
		index: make(map[string]int64),
		size:  info.Size(),
	}

	// Rebuild in-memory index by replaying the log.
	if err := kv.replay(); err != nil {
		file.Close()
		return nil, err
	}

	return kv, nil
}

// encodeRecord converts an operation into bytes.

func encodeRecord(o op, key []byte, vt valueType, value []byte) []byte {
	buf := make([]byte, headerSize+len(key)+len(value))

	buf[0] = byte(o)
	binary.BigEndian.PutUint32(buf[1:5], uint32(len(key)))
	buf[5] = byte(vt)
	binary.BigEndian.PutUint32(buf[6:10], uint32(len(value)))

	copy(buf[headerSize:], key)
	copy(buf[headerSize+len(key):], value)

	return buf
}

// replay reads the whole log file and rebuilds the index.
func (kv *KV) replay() error {
	var offset int64

	for {
		header := make([]byte, headerSize)
		if _, err := kv.file.ReadAt(header, offset); err != nil {
			if err == io.EOF {
				// Reached end of valid data
				return kv.truncateTo(offset)

			}
			return err
		}

		o := op(header[0])
		keyLen := binary.BigEndian.Uint32(header[1:5])
		vt := valueType(header[5])
		valLen := binary.BigEndian.Uint32(header[6:10])

		payloadLen := int64(keyLen) + int64(valLen)
		recordSize := int64(headerSize) + payloadLen

		// Incomplete record at the end, likely due to crash.
		// Truncate it so future appends start at a valid boundary.
		if offset+recordSize > kv.size {
			return kv.truncateTo(offset)
		}

		payload := make([]byte, payloadLen)

		if _, err := kv.file.ReadAt(payload, offset+headerSize); err != nil {
			return err
		}

		key := string(payload[:keyLen])

		switch o {
		case opPut:
			if vt < typeInt || vt > typeBool {
				return fmt.Errorf("invalid value type %d at offset %d", vt, offset)
			}
			kv.index[key] = offset
		case opDelete:
			delete(kv.index, key)
		default:
			return fmt.Errorf("unknown op %d at offset %d", o, offset)

		}
		offset += recordSize

	}
}

// truncateTo truncates the file to offset.
// This is useful for removing a partially written tail record.
func (kv *KV) truncateTo(offset int64) error {
	if offset == kv.size {
		return nil
	}

	if err := kv.file.Truncate(offset); err != nil {
		return err
	}

	kv.size = offset
	return nil
}

func (kv *KV) appendRecord(rec []byte) (int64, error) {
	offset := kv.size

	n, err := kv.file.Write(rec)
	if err != nil {
		return 0, err
	}
	if n != len(rec) {
		return 0, io.ErrShortWrite
	}
	kv.size += int64(n)
	return offset, nil
}

// putValue is the internal typed PUT implementation.
func (kv *KV) putValue(key string, vt valueType, value []byte) error {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	rec := encodeRecord(opPut, []byte(key), vt, value)

	offset, err := kv.appendRecord(rec)
	if err != nil {
		return err
	}

	kv.index[key] = offset
	return nil
}

// PutInt stores an INT value.
// We store it as 8 bytes on disk.
func (kv *KV) PutInt(key string, v int) error {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(int64(v)))
	return kv.putValue(key, typeInt, b)
}

// PutInt32 stores an INT32 value.
// We store it as 4 bytes on disk.
func (kv *KV) PutInt32(key string, v int32) error {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, uint32(v))
	return kv.putValue(key, typeInt32, b)
}

// PutString stores a STRING value.
func (kv *KV) PutString(key string, v string) error {
	return kv.putValue(key, typeString, []byte(v))
}

// PutChar stores a CHAR value.
// We store it as a 4-byte Unicode code point.
func (kv *KV) PutChar(key string, c rune) error {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, uint32(c))
	return kv.putValue(key, typeChar, b)
}

// PutBool stores a BOOL value.
func (kv *KV) PutBool(key string, v bool) error {
	var b []byte

	if v {
		b = []byte{1}
	} else {
		b = []byte{0}
	}

	return kv.putValue(key, typeBool, b)
}

// Delete removes a key by appending a tombstone.
func (kv *KV) Delete(key string) error {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	if _, ok := kv.index[key]; !ok {
		return nil
	}

	rec := encodeRecord(opDelete, []byte(key), typeNone, nil)

	if _, err := kv.appendRecord(rec); err != nil {
		return err
	}

	delete(kv.index, key)
	return nil
}

// getRaw returns the raw value type and value bytes.
func (kv *KV) getRaw(key string) (valueType, []byte, error) {
	kv.mu.RLock()
	defer kv.mu.RUnlock()

	offset, ok := kv.index[key]
	if !ok {
		return typeNone, nil, ErrNotFound
	}

	return kv.readRecordValue(offset)
}

// readRecordValue reads the value type and value bytes from disk.
// Caller should hold at least a read lock.
func (kv *KV) readRecordValue(offset int64) (valueType, []byte, error) {
	if offset < 0 || offset+headerSize > kv.size {
		return typeNone, nil, errors.New("invalid offset")
	}
	header := make([]byte, headerSize)

	if _, err := kv.file.ReadAt(header, offset); err != nil {
		return typeNone, nil, err
	}

	if op(header[0]) != opPut {
		return typeNone, nil, fmt.Errorf("record at offset %d is not PUT", offset)
	}
	keyLen := binary.BigEndian.Uint32(header[1:5])
	vt := valueType(header[5])
	valLen := binary.BigEndian.Uint32(header[6:10])

	payloadLen := int64(keyLen) + int64(valLen)

	if offset+int64(headerSize)+payloadLen > kv.size {
		return typeNone, nil, errors.New("record extends past end of file")
	}

	payload := make([]byte, payloadLen)

	if _, err := kv.file.ReadAt(payload, offset+headerSize); err != nil {
		return typeNone, nil, err
	}

	return vt, payload[keyLen:], nil

}

func checkType(actual, expected valueType) error {
	if actual != expected {
		return fmt.Errorf("%w: expected %s, got %s", ErrWrongType, expected, actual)
	}
	return nil
}

func (kv *KV) GetInt(key string) (int, error) {
	vt, b, err := kv.getRaw(key)
	if err != nil {
		return 0, err
	}

	if err := checkType(vt, typeInt); err != nil {
		return 0, err
	}
	if len(b) != 8 {
		return 0, errors.New("invalid INT record")
	}

	n := int64(binary.BigEndian.Uint64(b))
	return int(n), nil
}

// GetInt32 returns an INT32 value.
func (kv *KV) GetInt32(key string) (int32, error) {
	vt, b, err := kv.getRaw(key)
	if err != nil {
		return 0, err
	}

	if err := checkType(vt, typeInt32); err != nil {
		return 0, err
	}

	if len(b) != 4 {
		return 0, errors.New("invalid INT32 record")
	}

	u := binary.BigEndian.Uint32(b)
	return int32(u), nil
}

// GetString returns a STRING value.
func (kv *KV) GetString(key string) (string, error) {
	vt, b, err := kv.getRaw(key)
	if err != nil {
		return "", err
	}

	if err := checkType(vt, typeString); err != nil {
		return "", err
	}

	return string(b), nil
}

// GetChar returns a CHAR value.
func (kv *KV) GetChar(key string) (rune, error) {
	vt, b, err := kv.getRaw(key)
	if err != nil {
		return 0, err
	}

	if err := checkType(vt, typeChar); err != nil {
		return 0, err
	}

	if len(b) != 4 {
		return 0, errors.New("invalid CHAR record")
	}

	u := binary.BigEndian.Uint32(b)
	return rune(u), nil
}

// GetBool returns a BOOL value.
func (kv *KV) GetBool(key string) (bool, error) {
	vt, b, err := kv.getRaw(key)
	if err != nil {
		return false, err
	}

	if err := checkType(vt, typeBool); err != nil {
		return false, err
	}

	if len(b) != 1 {
		return false, errors.New("invalid BOOL record")
	}

	return b[0] == 1, nil
}

// Sync flushes file data to stable storage.
func (kv *KV) Sync() error {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	return kv.file.Sync()
}

// Close syncs and closes the file.
func (kv *KV) Close() error {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	if err := kv.file.Sync(); err != nil {
		return err
	}

	return kv.file.Close()
}

func main() {
	db, err := Open("data/typed.log")
	if err != nil {
		panic(err)
	}

	// Write typed values.
	if err := db.PutString("name", "alice"); err != nil {
		panic(err)
	}

	if err := db.PutInt("age", 30); err != nil {
		panic(err)
	}

	if err := db.PutInt32("score", 99); err != nil {
		panic(err)
	}

	if err := db.PutChar("grade", 'A'); err != nil {
		panic(err)
	}

	if err := db.PutBool("active", true); err != nil {
		panic(err)
	}

	// Overwrite an existing key.
	if err := db.PutInt("age", 31); err != nil {
		panic(err)
	}

	// Delete a key.
	if err := db.PutString("temp", "delete me"); err != nil {
		panic(err)
	}

	if err := db.Delete("temp"); err != nil {
		panic(err)
	}

	// Flush and close.
	if err := db.Sync(); err != nil {
		panic(err)
	}

	if err := db.Close(); err != nil {
		panic(err)
	}

	// Reopen and recover from disk.
	db, err = Open("data/typed.log")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	name, err := db.GetString("name")
	if err != nil {
		panic(err)
	}

	age, err := db.GetInt("age")
	if err != nil {
		panic(err)
	}

	score, err := db.GetInt32("score")
	if err != nil {
		panic(err)
	}

	grade, err := db.GetChar("grade")
	if err != nil {
		panic(err)
	}

	active, err := db.GetBool("active")
	if err != nil {
		panic(err)
	}

	fmt.Printf("name=%s age=%d score=%d grade=%c active=%t\n",
		name, age, score, grade, active)

	_, err = db.GetString("temp")
	fmt.Println("temp err =", err)

	// Wrong type access example.
	_, err = db.GetString("age")
	fmt.Println("wrong type err =", err)
}
