# Storage Engine Learning Repository

This repository documents my journey learning and implementing various storage engine approaches. Each subdirectory contains a focused implementation of a specific storage architecture.

## Learning Goals

- Understand the trade-offs between different storage engine designs
- Implement core concepts from scratch in Go
- Build a mental model for when to use each approach
- Explore persistence, concurrency, and recovery mechanisms

## Repository Structure

```
storage-engine/
├── README.md                    # This file - overview of all approaches
├── Append-Only-Log/             # Log-structured hash table (Bitcask-style)
│   ├── main.go                  # Core implementation
│   ├── main_test.go             # Comprehensive test suite
│   └── go.mod
├── Slotted-Page/                # (Planned) Page-based storage with slot arrays
├── LSM-Tree/                    # (Planned) Log-Structured Merge Tree
├── B-Tree/                      # (Planned) B+Tree implementation
└── WAL/                         # (Planned) Write-Ahead Logging variations
```

## Approaches to Implement

| Approach | Directory | Status | Key Characteristics |
|----------|-----------|--------|---------------------|
| **Append-Only Log (Bitcask)** | `Append-Only-Log/` | ✅ Done | Hash index + sequential log, O(1) reads, crash recovery via replay |
| **Slotted Page** | `Slotted-Page/` | 📋 Planned | Fixed-size pages, slot array for offsets, free space management |
| **LSM Tree** | `LSM-Tree/` | 📋 Planned | Memtable + SSTables, sorted runs, compaction, bloom filters |
| **B+Tree** | `B-Tree/` | 📋 Planned | Balanced tree, page splits/merges, range queries |
| **WAL Variations** | `WAL/` | 📋 Planned | ARIES, group commit, async replication |

## Concepts to Master Per Approach

### Append-Only Log (Current)
- [x] Record encoding/decoding with typed values
- [x] In-memory hash index (key → file offset)
- [x] Crash recovery via log replay
- [x] Tombstone-based deletion
- [x] Log compaction (rewrite live keys)
- [x] Concurrent access (RWMutex)
- [x] Typed values (INT, INT32, STRING, CHAR, BOOL)

### Slotted Page (Next)
- [ ] Page header with metadata (free space, slot count)
- [ ] Slot array for record offsets within page
- [ ] Variable-length record storage
- [ ] Free space fragmentation & compaction
- [ ] Page splitting and overflow handling

### LSM Tree
- [ ] Memtable (skiplist/B-tree) for writes
- [ ] Immutable SSTable generation
- [ ] Compaction strategies (leveled, tiered)
- [ ] Bloom filters for negative lookups
- [ ] SSTable index (sparse/block)

### B+Tree
- [ ] Node structure (internal vs leaf)
- [ ] Split/merge operations
- [ ] Range scans via leaf links
- [ ] Concurrency (latch crabbing, B-link)

## Running the Append-Only Log

```bash
cd Append-Only-Log
go run main.go
# Output: name=alice age=31 score=99 grade=A active=true
#         temp err = key not found
#         wrong type err = value type mismatch: expected STRING, got INT

go test -v ./...
go test -race ./...  # Race detector passes
```

## Resources & References

- **Bitcask Paper** - "Bitcask: A Log-Structured Hash Table for Fast Key/Value Data"
- **LSM Tree Paper** - "The Log-Structured Merge-Tree (LSM-Tree)" by O'Neil et al.
- **B+Tree** - "Ubiquitous B-Tree" by Comer
- **Database Internals** - Alex Petrov (book)
- **Designing Data-Intensive Applications** - Martin Kleppmann (book)