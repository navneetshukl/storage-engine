# Append-Only Log Storage Engine

A **Bitcask-style** log-structured hash table implementation in Go. This is a complete, production-style key-value store with typed values, crash recovery, and log compaction.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      In-Memory Index                        │
│  map[string]int64  ──►  key → file offset of latest PUT    │
│                    O(1) lookup, all keys must fit in RAM   │
└──────────────────────────┬──────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────┐
│                   Append-Only Log File                      │
│  [Header(10B)][Key][Value][Header(10B)][Key][Value]...     │
│                                                             │
│  Header:  op(1B) │ keyLen(4B) │ valueType(1B) │ valLen(4B) │
│                                                             │
│  PUT    → append record with typed value                    │
│  DELETE → append tombstone (opDelete, no value)            │
└─────────────────────────────────────────────────────────────┘
```

## Features

| Feature | Implementation |
|---------|----------------|
| **Typed Values** | INT (8B), INT32 (4B), STRING (var), CHAR (4B Unicode), BOOL (1B) |
| **Crash Recovery** | Full log replay on `Open()` rebuilds index |
| **Deletion** | Tombstone records (opDelete) |
| **Compaction** | Rewrites log with only live keys, preserves types |
| **Concurrency** | `sync.RWMutex` for thread-safe access |
| **Durability** | `Sync()` + `Close()` flush to stable storage |

## Record Format

```
┌────┬────────┬──────────┬────────┐
│ op │ keyLen │ valType  │ valLen │  = 10 bytes header
├────┼────────┼──────────┼────────┤
│  1 │   4    │    1     │   4    │
└────┴────────┴──────────┴────────┘
┌──────────────────────────────────┐
│              Key                 │  keyLen bytes
├──────────────────────────────────┤
│              Value               │  valLen bytes
└──────────────────────────────────┘
```

**Value Types:**
| Type | Code | Storage |
|------|------|---------|
| INT | 1 | 8 bytes (int64, big-endian) |
| INT32 | 2 | 4 bytes (int32, big-endian) |
| STRING | 3 | Variable (UTF-8) |
| CHAR | 4 | 4 bytes (Unicode code point) |
| BOOL | 5 | 1 byte (0/1) |

## API

```go
// Open/create database
db, err := Open("data/mydb.log")
defer db.Close()

// Typed puts
db.PutInt("age", 30)
db.PutInt32("score", 100)
db.PutString("name", "alice")
db.PutChar("grade", 'A')
db.PutBool("active", true)

// Typed gets (return ErrWrongType on mismatch)
age, _ := db.GetInt("age")
name, _ := db.GetString("name")

// Delete (tombstone)
db.Delete("key")

// Maintenance
db.Sync()     // fsync
db.Compact()  // rewrite log, remove dead keys
```

## Running

```bash
cd Append-Only-Log

# Run demo
go run main.go

# Run tests (21 test functions)
go test -v ./...

# Race detector
go test -race ./...
```

## Test Coverage

```
=== RUN   TestOpen                    # Open new/existing DB
=== RUN   TestEncodeRecord            # Record encoding verification
=== RUN   TestPutInt / GetInt         # INT type (neg, zero, max, overwrite)
=== RUN   TestPutInt32 / GetInt32     # INT32 type
=== RUN   TestPutString / GetString   # STRING (empty, unicode, overwrite)
=== RUN   TestPutChar / GetChar       # CHAR (ASCII, emoji, CJK)
=== RUN   TestPutBool / GetBool       # BOOL (true/false, overwrite)
=== RUN   TestDelete                  # Tombstone + persistence
=== RUN   TestWrongType               # Type mismatch errors
=== RUN   TestNotFound                # Missing key errors
=== RUN   TestPersistence             # Close/reopen cycle
=== RUN   TestSync                    # fsync
=== RUN   TestClose                   # Close + reopen
=== RUN   TestCompact                 # Size reduction + data integrity
=== RUN   TestConcurrentAccess        # 10 writers + 10 readers
=== RUN   TestReplay                  # Crash recovery (no sync)
=== RUN   TestValueTypeString         # Enum String() method
=== RUN   TestCheckType               # Internal type checking
=== RUN   TestReadRecordValue         # Internal read at offset
=== RUN   TestTruncateTo              # Internal truncation
=== RUN   TestAppendRecord            # Internal append
--- PASS: All 21 tests
```

## Trade-offs

| Pros | Cons |
|------|------|
| ✅ Simple implementation | ❌ All keys must fit in memory |
| ✅ O(1) point reads | ❌ No range queries |
| ✅ Fast sequential writes | ❌ Log grows until compacted |
| ✅ Crash-safe (append-only) | ❌ Compaction rewrites entire log |
| ✅ Natural write ordering | ❌ Not suitable for large datasets > RAM |

## When to Use

- Small-to-medium datasets where all keys fit in RAM
- Write-heavy workloads with point lookups
- Embedded systems needing simple persistence
- Learning foundation for LSM trees (which add sorting + levels)

## Next Steps

This forms the **foundation** for:
1. **LSM Tree** - Add memtable, sorted SSTables, multi-level compaction
2. **WAL** - Extract replay logic for use with other engines
3. **Hybrid** - Combine with page-based storage for larger-than-RAM