package sparse

const offset32 = 2166136261
const prime32 = 16777619

func hashToken(token string) uint32 {
	var h uint32 = offset32

	// Write little-endian uint64 length prefix, matching Rust's Hash trait
	// behavior for slices (length is hashed as usize before bytes).
	l := uint64(len(token))
	for range 8 {
		h ^= uint32(byte(l))
		h *= prime32
		l >>= 8
	}

	for i := 0; i < len(token); i++ {
		h ^= uint32(token[i])
		h *= prime32
	}

	return h
}
