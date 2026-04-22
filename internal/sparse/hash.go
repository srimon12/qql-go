package sparse

import (
	"encoding/binary"
	"hash/fnv"
)

const defaultHashSpace = 1 << 20

func hashToken(token string) uint32 {
	h := fnv.New32a()
	// Write little-endian uint64 length prefix, matching Rust's Hash trait
	// behavior for slices (length is hashed as usize before bytes).
	var lenBuf [8]byte
	binary.LittleEndian.PutUint64(lenBuf[:], uint64(len(token)))
	_, _ = h.Write(lenBuf[:])
	_, _ = h.Write([]byte(token))
	return h.Sum32() % defaultHashSpace
}
