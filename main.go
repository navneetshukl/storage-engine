package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
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

func main() {
	kv, err := Open("logs")
	if err != nil {
		log.Println("Open error ", err)
		return
	}
	log.Printf("%v\n", kv.file)

	kv.putValue("name",1,[]byte("navneet"))
}
