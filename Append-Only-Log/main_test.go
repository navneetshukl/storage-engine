package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
)

// TestOpen tests opening a new and existing database.
func TestOpen(t *testing.T) {
	// Test opening a new database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	kv, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer kv.Close()

	if kv == nil {
		t.Fatal("Open returned nil KV")
	}

	// Test opening an existing database
	kv2, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open existing failed: %v", err)
	}
	defer kv2.Close()

	// Test opening with non-existent parent directory
	kv3, err := Open(filepath.Join(tmpDir, "subdir", "test.db"))
	if err != nil {
		t.Fatalf("Open with nested dir failed: %v", err)
	}
	defer kv3.Close()
}

// TestEncodeRecord tests the encodeRecord function.
func TestEncodeRecord(t *testing.T) {
	tests := []struct {
		name       string
		o          op
		key        string
		vt         valueType
		value      []byte
		wantSize   int
		wantOp     op
		wantKeyLen int
		wantVt     valueType
		wantValLen int
	}{
		{
			name:       "put int",
			o:          opPut,
			key:        "key1",
			vt:         typeInt,
			value:      []byte{0, 0, 0, 0, 0, 0, 0, 42},
			wantSize:   headerSize + 4 + 8,
			wantOp:     opPut,
			wantKeyLen: 4,
			wantVt:     typeInt,
			wantValLen: 8,
		},
		{
			name:       "put string",
			o:          opPut,
			key:        "name",
			vt:         typeString,
			value:      []byte("hello"),
			wantSize:   headerSize + 4 + 5,
			wantOp:     opPut,
			wantKeyLen: 4,
			wantVt:     typeString,
			wantValLen: 5,
		},
		{
			name:       "delete",
			o:          opDelete,
			key:        "key1",
			vt:         typeNone,
			value:      nil,
			wantSize:   headerSize + 4 + 0,
			wantOp:     opDelete,
			wantKeyLen: 4,
			wantVt:     typeNone,
			wantValLen: 0,
		},
		{
			name:       "empty key",
			o:          opPut,
			key:        "",
			vt:         typeBool,
			value:      []byte{1},
			wantSize:   headerSize + 0 + 1,
			wantOp:     opPut,
			wantKeyLen: 0,
			wantVt:     typeBool,
			wantValLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := encodeRecord(tt.o, []byte(tt.key), tt.vt, tt.value)

			if len(rec) != tt.wantSize {
				t.Errorf("record size = %d, want %d", len(rec), tt.wantSize)
			}

			if op(rec[0]) != tt.wantOp {
				t.Errorf("op = %d, want %d", rec[0], tt.wantOp)
			}

			keyLen := binary.BigEndian.Uint32(rec[1:5])
			if int(keyLen) != tt.wantKeyLen {
				t.Errorf("keyLen = %d, want %d", keyLen, tt.wantKeyLen)
			}

			vt := valueType(rec[5])
			if vt != tt.wantVt {
				t.Errorf("valueType = %d, want %d", vt, tt.wantVt)
			}

			valLen := binary.BigEndian.Uint32(rec[6:10])
			if int(valLen) != tt.wantValLen {
				t.Errorf("valLen = %d, want %d", valLen, tt.wantValLen)
			}

			// Verify key
			key := string(rec[headerSize : headerSize+tt.wantKeyLen])
			if key != tt.key {
				t.Errorf("key = %q, want %q", key, tt.key)
			}

			// Verify value
			valueStart := headerSize + tt.wantKeyLen
			value := rec[valueStart:]
			if string(value) != string(tt.value) {
				t.Errorf("value = %v, want %v", value, tt.value)
			}
		})
	}
}

// TestPutInt tests PutInt and GetInt.
func TestPutInt(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	kv, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer kv.Close()

	// Test basic put and get
	if err := kv.PutInt("age", 30); err != nil {
		t.Fatalf("PutInt failed: %v", err)
	}

	val, err := kv.GetInt("age")
	if err != nil {
		t.Fatalf("GetInt failed: %v", err)
	}
	if val != 30 {
		t.Errorf("GetInt = %d, want 30", val)
	}

	// Test overwrite
	if err := kv.PutInt("age", 31); err != nil {
		t.Fatalf("PutInt overwrite failed: %v", err)
	}

	val, err = kv.GetInt("age")
	if err != nil {
		t.Fatalf("GetInt after overwrite failed: %v", err)
	}
	if val != 31 {
		t.Errorf("GetInt after overwrite = %d, want 31", val)
	}

	// Test negative numbers
	if err := kv.PutInt("neg", -42); err != nil {
		t.Fatalf("PutInt negative failed: %v", err)
	}

	val, err = kv.GetInt("neg")
	if err != nil {
		t.Fatalf("GetInt negative failed: %v", err)
	}
	if val != -42 {
		t.Errorf("GetInt negative = %d, want -42", val)
	}

	// Test zero
	if err := kv.PutInt("zero", 0); err != nil {
		t.Fatalf("PutInt zero failed: %v", err)
	}

	val, err = kv.GetInt("zero")
	if err != nil {
		t.Fatalf("GetInt zero failed: %v", err)
	}
	if val != 0 {
		t.Errorf("GetInt zero = %d, want 0", val)
	}

	// Test large numbers
	if err := kv.PutInt("large", 9223372036854775807); err != nil { // max int64
		t.Fatalf("PutInt large failed: %v", err)
	}

	val, err = kv.GetInt("large")
	if err != nil {
		t.Fatalf("GetInt large failed: %v", err)
	}
	if val != 9223372036854775807 {
		t.Errorf("GetInt large = %d, want 9223372036854775807", val)
	}
}

// TestPutInt32 tests PutInt32 and GetInt32.
func TestPutInt32(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	kv, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer kv.Close()

	// Test basic put and get
	if err := kv.PutInt32("score", 99); err != nil {
		t.Fatalf("PutInt32 failed: %v", err)
	}

	val, err := kv.GetInt32("score")
	if err != nil {
		t.Fatalf("GetInt32 failed: %v", err)
	}
	if val != 99 {
		t.Errorf("GetInt32 = %d, want 99", val)
	}

	// Test negative
	if err := kv.PutInt32("neg", -100); err != nil {
		t.Fatalf("PutInt32 negative failed: %v", err)
	}

	val, err = kv.GetInt32("neg")
	if err != nil {
		t.Fatalf("GetInt32 negative failed: %v", err)
	}
	if val != -100 {
		t.Errorf("GetInt32 negative = %d, want -100", val)
	}

	// Test max int32
	if err := kv.PutInt32("max", 2147483647); err != nil {
		t.Fatalf("PutInt32 max failed: %v", err)
	}

	val, err = kv.GetInt32("max")
	if err != nil {
		t.Fatalf("GetInt32 max failed: %v", err)
	}
	if val != 2147483647 {
		t.Errorf("GetInt32 max = %d, want 2147483647", val)
	}
}

// TestPutString tests PutString and GetString.
func TestPutString(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	kv, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer kv.Close()

	// Test basic put and get
	if err := kv.PutString("name", "alice"); err != nil {
		t.Fatalf("PutString failed: %v", err)
	}

	val, err := kv.GetString("name")
	if err != nil {
		t.Fatalf("GetString failed: %v", err)
	}
	if val != "alice" {
		t.Errorf("GetString = %q, want %q", val, "alice")
	}

	// Test empty string
	if err := kv.PutString("empty", ""); err != nil {
		t.Fatalf("PutString empty failed: %v", err)
	}

	val, err = kv.GetString("empty")
	if err != nil {
		t.Fatalf("GetString empty failed: %v", err)
	}
	if val != "" {
		t.Errorf("GetString empty = %q, want empty string", val)
	}

	// Test unicode
	if err := kv.PutString("unicode", "你好世界🌍"); err != nil {
		t.Fatalf("PutString unicode failed: %v", err)
	}

	val, err = kv.GetString("unicode")
	if err != nil {
		t.Fatalf("GetString unicode failed: %v", err)
	}
	if val != "你好世界🌍" {
		t.Errorf("GetString unicode = %q, want %q", val, "你好世界🌍")
	}

	// Test overwrite
	if err := kv.PutString("name", "bob"); err != nil {
		t.Fatalf("PutString overwrite failed: %v", err)
	}

	val, err = kv.GetString("name")
	if err != nil {
		t.Fatalf("GetString after overwrite failed: %v", err)
	}
	if val != "bob" {
		t.Errorf("GetString after overwrite = %q, want %q", val, "bob")
	}
}

// TestPutChar tests PutChar and GetChar.
func TestPutChar(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	kv, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer kv.Close()

	// Test basic ASCII
	if err := kv.PutChar("grade", 'A'); err != nil {
		t.Fatalf("PutChar failed: %v", err)
	}

	val, err := kv.GetChar("grade")
	if err != nil {
		t.Fatalf("GetChar failed: %v", err)
	}
	if val != 'A' {
		t.Errorf("GetChar = %c, want 'A'", val)
	}

	// Test unicode
	if err := kv.PutChar("emoji", '😀'); err != nil {
		t.Fatalf("PutChar unicode failed: %v", err)
	}

	val, err = kv.GetChar("emoji")
	if err != nil {
		t.Fatalf("GetChar unicode failed: %v", err)
	}
	if val != '😀' {
		t.Errorf("GetChar unicode = %c, want '😀'", val)
	}

	// Test chinese character
	if err := kv.PutChar("chinese", '中'); err != nil {
		t.Fatalf("PutChar chinese failed: %v", err)
	}

	val, err = kv.GetChar("chinese")
	if err != nil {
		t.Fatalf("GetChar chinese failed: %v", err)
	}
	if val != '中' {
		t.Errorf("GetChar chinese = %c, want '中'", val)
	}

	// Test overwrite
	if err := kv.PutChar("grade", 'B'); err != nil {
		t.Fatalf("PutChar overwrite failed: %v", err)
	}

	val, err = kv.GetChar("grade")
	if err != nil {
		t.Fatalf("GetChar after overwrite failed: %v", err)
	}
	if val != 'B' {
		t.Errorf("GetChar after overwrite = %c, want 'B'", val)
	}
}

// TestPutBool tests PutBool and GetBool.
func TestPutBool(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	kv, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer kv.Close()

	// Test true
	if err := kv.PutBool("active", true); err != nil {
		t.Fatalf("PutBool true failed: %v", err)
	}

	val, err := kv.GetBool("active")
	if err != nil {
		t.Fatalf("GetBool true failed: %v", err)
	}
	if !val {
		t.Errorf("GetBool true = %v, want true", val)
	}

	// Test false
	if err := kv.PutBool("inactive", false); err != nil {
		t.Fatalf("PutBool false failed: %v", err)
	}

	val, err = kv.GetBool("inactive")
	if err != nil {
		t.Fatalf("GetBool false failed: %v", err)
	}
	if val {
		t.Errorf("GetBool false = %v, want false", val)
	}

	// Test overwrite
	if err := kv.PutBool("active", false); err != nil {
		t.Fatalf("PutBool overwrite failed: %v", err)
	}

	val, err = kv.GetBool("active")
	if err != nil {
		t.Fatalf("GetBool after overwrite failed: %v", err)
	}
	if val {
		t.Errorf("GetBool after overwrite = %v, want false", val)
	}
}

// TestDelete tests Delete functionality.
func TestDelete(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	kv, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer kv.Close()

	// Put some values
	if err := kv.PutString("key1", "value1"); err != nil {
		t.Fatalf("PutString failed: %v", err)
	}
	if err := kv.PutInt("key2", 42); err != nil {
		t.Fatalf("PutInt failed: %v", err)
	}

	// Delete existing key
	if err := kv.Delete("key1"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify key is deleted
	_, err = kv.GetString("key1")
	if err != ErrNotFound {
		t.Errorf("GetString after delete = %v, want ErrNotFound", err)
	}

	// Delete non-existing key should not error
	if err := kv.Delete("nonexistent"); err != nil {
		t.Fatalf("Delete non-existing failed: %v", err)
	}

	// Other key should still exist
	val, err := kv.GetInt("key2")
	if err != nil {
		t.Fatalf("GetInt after delete other key failed: %v", err)
	}
	if val != 42 {
		t.Errorf("GetInt after delete other key = %d, want 42", val)
	}

	// Re-open and verify deletion persisted
	kv.Close()
	kv2, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Reopen failed: %v", err)
	}
	defer kv2.Close()

	_, err = kv2.GetString("key1")
	if err != ErrNotFound {
		t.Errorf("GetString after reopen = %v, want ErrNotFound", err)
	}

	val, err = kv2.GetInt("key2")
	if err != nil {
		t.Fatalf("GetInt after reopen failed: %v", err)
	}
	if val != 42 {
		t.Errorf("GetInt after reopen = %d, want 42", val)
	}
}

// TestWrongType tests type mismatch errors.
func TestWrongType(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	kv, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer kv.Close()

	// Put an int
	if err := kv.PutInt("age", 30); err != nil {
		t.Fatalf("PutInt failed: %v", err)
	}

	// Try to get as string
	_, err = kv.GetString("age")
	if err == nil {
		t.Fatal("GetString on int key should fail")
	}
	if !errors.Is(err, ErrWrongType) {
		t.Errorf("GetString on int key error = %v, want ErrWrongType", err)
	}

	// Try to get as int32
	_, err = kv.GetInt32("age")
	if err == nil {
		t.Fatal("GetInt32 on int key should fail")
	}
	if !errors.Is(err, ErrWrongType) {
		t.Errorf("GetInt32 on int key error = %v, want ErrWrongType", err)
	}

	// Try to get as char
	_, err = kv.GetChar("age")
	if err == nil {
		t.Fatal("GetChar on int key should fail")
	}
	if !errors.Is(err, ErrWrongType) {
		t.Errorf("GetChar on int key error = %v, want ErrWrongType", err)
	}

	// Try to get as bool
	_, err = kv.GetBool("age")
	if err == nil {
		t.Fatal("GetBool on int key should fail")
	}
	if !errors.Is(err, ErrWrongType) {
		t.Errorf("GetBool on int key error = %v, want ErrWrongType", err)
	}

	// Put a string
	if err := kv.PutString("name", "alice"); err != nil {
		t.Fatalf("PutString failed: %v", err)
	}

	// Try to get as int
	_, err = kv.GetInt("name")
	if err == nil {
		t.Fatal("GetInt on string key should fail")
	}
	if !errors.Is(err, ErrWrongType) {
		t.Errorf("GetInt on string key error = %v, want ErrWrongType", err)
	}
}

// TestNotFound tests ErrNotFound for missing keys.
func TestNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	kv, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer kv.Close()

	// Try to get non-existing key
	_, err = kv.GetInt("nonexistent")
	if err == nil {
		t.Fatal("GetInt on missing key should fail")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("GetInt on missing key error = %v, want ErrNotFound", err)
	}

	_, err = kv.GetString("nonexistent")
	if err == nil {
		t.Fatal("GetString on missing key should fail")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("GetString on missing key error = %v, want ErrNotFound", err)
	}

	_, err = kv.GetInt32("nonexistent")
	if err == nil {
		t.Fatal("GetInt32 on missing key should fail")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("GetInt32 on missing key error = %v, want ErrNotFound", err)
	}

	_, err = kv.GetChar("nonexistent")
	if err == nil {
		t.Fatal("GetChar on missing key should fail")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("GetChar on missing key error = %v, want ErrNotFound", err)
	}

	_, err = kv.GetBool("nonexistent")
	if err == nil {
		t.Fatal("GetBool on missing key should fail")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("GetBool on missing key error = %v, want ErrNotFound", err)
	}
}

// TestPersistence tests that data persists across reopens.
func TestPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Write data
	kv, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	if err := kv.PutString("name", "alice"); err != nil {
		t.Fatalf("PutString failed: %v", err)
	}
	if err := kv.PutInt("age", 30); err != nil {
		t.Fatalf("PutInt failed: %v", err)
	}
	if err := kv.PutInt32("score", 100); err != nil {
		t.Fatalf("PutInt32 failed: %v", err)
	}
	if err := kv.PutChar("grade", 'A'); err != nil {
		t.Fatalf("PutChar failed: %v", err)
	}
	if err := kv.PutBool("active", true); err != nil {
		t.Fatalf("PutBool failed: %v", err)
	}

	if err := kv.Sync(); err != nil {
		t.Fatalf("Sync failed: %v", err)
	}
	kv.Close()

	// Reopen and verify
	kv2, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Reopen failed: %v", err)
	}
	defer kv2.Close()

	name, err := kv2.GetString("name")
	if err != nil || name != "alice" {
		t.Errorf("name = %q, %v, want alice, nil", name, err)
	}

	age, err := kv2.GetInt("age")
	if err != nil || age != 30 {
		t.Errorf("age = %d, %v, want 30, nil", age, err)
	}

	score, err := kv2.GetInt32("score")
	if err != nil || score != 100 {
		t.Errorf("score = %d, %v, want 100, nil", score, err)
	}

	grade, err := kv2.GetChar("grade")
	if err != nil || grade != 'A' {
		t.Errorf("grade = %c, %v, want A, nil", grade, err)
	}

	active, err := kv2.GetBool("active")
	if err != nil || !active {
		t.Errorf("active = %v, %v, want true, nil", active, err)
	}
}

// TestSync tests Sync functionality.
func TestSync(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	kv, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer kv.Close()

	if err := kv.PutString("key", "value"); err != nil {
		t.Fatalf("PutString failed: %v", err)
	}

	if err := kv.Sync(); err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	// Verify data is readable
	val, err := kv.GetString("key")
	if err != nil || val != "value" {
		t.Errorf("GetString after sync = %q, %v, want value, nil", val, err)
	}
}

// TestClose tests Close functionality.
func TestClose(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	kv, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	if err := kv.PutString("key", "value"); err != nil {
		t.Fatalf("PutString failed: %v", err)
	}

	if err := kv.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Verify data persisted
	kv2, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Reopen after close failed: %v", err)
	}
	defer kv2.Close()

	val, err := kv2.GetString("key")
	if err != nil || val != "value" {
		t.Errorf("GetString after close = %q, %v, want value, nil", val, err)
	}
}

// TestCompact tests Compact functionality.
func TestCompact(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	kv, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	// Add some data
	if err := kv.PutString("key1", "value1"); err != nil {
		t.Fatalf("PutString failed: %v", err)
	}
	if err := kv.PutInt("key2", 42); err != nil {
		t.Fatalf("PutInt failed: %v", err)
	}
	if err := kv.PutString("key3", "value3"); err != nil {
		t.Fatalf("PutString failed: %v", err)
	}

	// Delete one key
	if err := kv.Delete("key2"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Overwrite a key (creates old record + new record)
	if err := kv.PutString("key1", "newvalue1"); err != nil {
		t.Fatalf("PutString overwrite failed: %v", err)
	}

	// Get file size before compact
	info, err := kv.file.Stat()
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}
	sizeBefore := info.Size()

	if err := kv.Compact(); err != nil {
		t.Fatalf("Compact failed: %v", err)
	}

	// Get file size after compact
	info, err = kv.file.Stat()
	if err != nil {
		t.Fatalf("Stat after compact failed: %v", err)
	}
	sizeAfter := info.Size()

	// Compact should reduce file size (removed deleted key and old version of key1)
	if sizeAfter >= sizeBefore {
		t.Errorf("Compact did not reduce file size: before=%d, after=%d", sizeBefore, sizeAfter)
	}

	// Verify data integrity after compact
	val, err := kv.GetString("key1")
	if err != nil || val != "newvalue1" {
		t.Errorf("GetString key1 after compact = %q, %v, want newvalue1, nil", val, err)
	}

	_, err = kv.GetInt("key2")
	if err != ErrNotFound {
		t.Errorf("GetInt key2 after compact = %v, want ErrNotFound", err)
	}

	val, err = kv.GetString("key3")
	if err != nil || val != "value3" {
		t.Errorf("GetString key3 after compact = %q, %v, want value3, nil", val, err)
	}

	// Verify reopen works
	kv.Close()
	kv2, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Reopen after compact failed: %v", err)
	}
	defer kv2.Close()

	val, err = kv2.GetString("key1")
	if err != nil || val != "newvalue1" {
		t.Errorf("GetString key1 after reopen = %q, %v, want newvalue1, nil", val, err)
	}
}

// TestConcurrentAccess tests concurrent reads and writes.
func TestConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	kv, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer kv.Close()

	// Initial data
	if err := kv.PutInt("counter", 0); err != nil {
		t.Fatalf("PutInt initial failed: %v", err)
	}

	const numGoroutines = 10
	const numIterations = 100

	done := make(chan bool, numGoroutines*2)

	// Concurrent writers
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			for j := 0; j < numIterations; j++ {
				key := fmt.Sprintf("key-%d-%d", id, j)
				if err := kv.PutInt(key, id*numIterations+j); err != nil {
					t.Errorf("PutInt concurrent failed: %v", err)
				}
			}
			done <- true
		}(i)
	}

	// Concurrent readers
	for i := 0; i < numGoroutines; i++ {
		go func() {
			for j := 0; j < numIterations; j++ {
				kv.GetInt("counter")
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < numGoroutines*2; i++ {
		<-done
	}

	// Verify all written keys exist
	for i := 0; i < numGoroutines; i++ {
		for j := 0; j < numIterations; j++ {
			key := fmt.Sprintf("key-%d-%d", i, j)
			val, err := kv.GetInt(key)
			if err != nil {
				t.Errorf("GetInt %s failed: %v", key, err)
			}
			expected := i*numIterations + j
			if val != expected {
				t.Errorf("GetInt %s = %d, want %d", key, val, expected)
			}
		}
	}
}

// TestReplay tests the replay functionality (crash recovery).
func TestReplay(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Write data
	kv, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	if err := kv.PutString("key1", "value1"); err != nil {
		t.Fatalf("PutString failed: %v", err)
	}
	if err := kv.PutInt("key2", 42); err != nil {
		t.Fatalf("PutInt failed: %v", err)
	}

	// Close without sync to simulate crash
	kv.file.Close()

	// Reopen - should replay and recover
	kv2, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Reopen after crash failed: %v", err)
	}
	defer kv2.Close()

	val, err := kv2.GetString("key1")
	if err != nil || val != "value1" {
		t.Errorf("GetString after crash = %q, %v, want value1, nil", val, err)
	}

	val2, err := kv2.GetInt("key2")
	if err != nil || val2 != 42 {
		t.Errorf("GetInt after crash = %d, %v, want 42, nil", val2, err)
	}
}

// TestValueTypeString tests the String() method on valueType.
func TestValueTypeString(t *testing.T) {
	tests := []struct {
		vt   valueType
		want string
	}{
		{typeNone, "NONE"},
		{typeInt, "INT"},
		{typeInt32, "INT32"},
		{typeString, "STRING"},
		{typeChar, "CHAR"},
		{typeBool, "BOOL"},
		{valueType(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.vt.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestCheckType tests the checkType function.
func TestCheckType(t *testing.T) {
	// Test matching types
	if err := checkType(typeInt, typeInt); err != nil {
		t.Errorf("checkType(int, int) = %v, want nil", err)
	}
	if err := checkType(typeString, typeString); err != nil {
		t.Errorf("checkType(string, string) = %v, want nil", err)
	}

	// Test mismatched types
	err := checkType(typeInt, typeString)
	if err == nil {
		t.Fatal("checkType(int, string) should fail")
	}
	if !errors.Is(err, ErrWrongType) {
		t.Errorf("checkType(int, string) error = %v, want ErrWrongType", err)
	}

	err = checkType(typeBool, typeInt)
	if err == nil {
		t.Fatal("checkType(bool, int) should fail")
	}
	if !errors.Is(err, ErrWrongType) {
		t.Errorf("checkType(bool, int) error = %v, want ErrWrongType", err)
	}
}

// TestReadRecordValue tests readRecordValue internal function.
func TestReadRecordValue(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	kv, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer kv.Close()

	// Put a value
	if err := kv.PutString("testkey", "testvalue"); err != nil {
		t.Fatalf("PutString failed: %v", err)
	}

	// Get the offset from index
	kv.mu.RLock()
	offset := kv.index["testkey"]
	kv.mu.RUnlock()

	// Read the record value
	vt, value, err := kv.readRecordValue(offset)
	if err != nil {
		t.Fatalf("readRecordValue failed: %v", err)
	}

	if vt != typeString {
		t.Errorf("valueType = %v, want STRING", vt)
	}
	if string(value) != "testvalue" {
		t.Errorf("value = %q, want testvalue", string(value))
	}
}

// TestTruncateTo tests the truncateTo internal function.
func TestTruncateTo(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	kv, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer kv.Close()

	// Put some data
	if err := kv.PutString("key", "value"); err != nil {
		t.Fatalf("PutString failed: %v", err)
	}

	// Get current size
	kv.mu.Lock()
	size := kv.size
	kv.mu.Unlock()

	// Truncate to same size (no-op)
	if err := kv.truncateTo(size); err != nil {
		t.Errorf("truncateTo same size failed: %v", err)
	}

	// Truncate to smaller size
	newSize := size - 10
	if err := kv.truncateTo(newSize); err != nil {
		t.Errorf("truncateTo smaller failed: %v", err)
	}

	// Verify size updated
	kv.mu.Lock()
	if kv.size != newSize {
		t.Errorf("size after truncate = %d, want %d", kv.size, newSize)
	}
	kv.mu.Unlock()
}

// TestAppendRecord tests the appendRecord internal function.
func TestAppendRecord(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	kv, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer kv.Close()

	rec := encodeRecord(opPut, []byte("test"), typeString, []byte("value"))

	offset, err := kv.appendRecord(rec)
	if err != nil {
		t.Fatalf("appendRecord failed: %v", err)
	}

	if offset != 0 {
		t.Errorf("appendRecord offset = %d, want 0", offset)
	}

	// Manually update index since appendRecord doesn't do it
	kv.mu.Lock()
	kv.index["test"] = offset
	kv.mu.Unlock()

	// Verify data was written
	val, err := kv.GetString("test")
	if err != nil || val != "value" {
		t.Errorf("GetString after appendRecord = %q, %v, want value, nil", val, err)
	}
}
