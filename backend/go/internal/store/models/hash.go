package models

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
)

// The Images.PerceptualHash column is bit(64). Postgres returns that type to
// pgx as either a string of '0'/'1' characters or raw bytes depending on the
// codec configuration, so both shapes are accepted here.
//
// Getting this conversion wrong is silent and dangerous: a misread hash would
// make duplicate detection fail open, letting duplicates through. It is
// therefore handled explicitly rather than by a generic scan.

// PerceptualHashFromBitString converts a Postgres bit(64) string into a uint64.
//
// The first character is the most significant bit, matching the convention the
// C# service uses when it stores hash.ToString("X16") and reads it back.
func PerceptualHashFromBitString(value string) (uint64, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, fmt.Errorf("perceptual hash bit string is empty")
	}
	if len(trimmed) != HashBits {
		return 0, fmt.Errorf("perceptual hash must be %d bits, got %d", HashBits, len(trimmed))
	}

	var result uint64
	for i := 0; i < HashBits; i++ {
		result <<= 1
		switch trimmed[i] {
		case '0':
		case '1':
			result |= 1
		default:
			return 0, fmt.Errorf("invalid bit %q at position %d in perceptual hash", trimmed[i], i)
		}
	}
	return result, nil
}

// HashBits is the width of the stored perceptual hash.
const HashBits = 64

// PerceptualHashToBitString converts a hash into the bit(64) representation
// Postgres stores.
//
// This is the inverse of PerceptualHashFromBitString and is used when writing
// a newly uploaded image.
func PerceptualHashToBitString(hash uint64) string {
	var builder strings.Builder
	builder.Grow(HashBits)
	for i := HashBits - 1; i >= 0; i-- {
		if hash&(1<<uint(i)) != 0 {
			builder.WriteByte('1')
		} else {
			builder.WriteByte('0')
		}
	}
	return builder.String()
}

// PerceptualHashFromBytes converts an 8-byte big-endian representation.
//
// pgx may deliver bit(64) as raw bytes depending on how the column is decoded,
// so this is provided as an alternative entry point.
func PerceptualHashFromBytes(data []byte) (uint64, error) {
	switch len(data) {
	case 8:
		return binary.BigEndian.Uint64(data), nil
	case HashBits:
		// A 64-byte string of '0' and '1' is also possible.
		return PerceptualHashFromBitString(string(data))
	default:
		return 0, fmt.Errorf("perceptual hash must be 8 bytes or %d bit characters, got %d bytes",
			HashBits, len(data))
	}
}

// PerceptualHashToHex renders the hash the way the API exposes it.
//
// BitArrayHelper.ToHex copies the bits into a big-endian byte array and prints
// it as lowercase hex, so the first bit of the hash is the high bit of the first
// byte. The frontend displays this value on the image page, so the exact
// formatting is part of the contract.
func PerceptualHashToHex(hash uint64) string {
	var bytes [HashBits / 8]byte
	binary.BigEndian.PutUint64(bytes[:], hash)
	return hex.EncodeToString(bytes[:])
}

// HammingDistance returns the number of differing bits between two hashes.
//
// This is the metric duplicate detection uses: an upload is rejected when its
// distance to any stored image is <= 4. See UploadImage/Command.cs.
//
// The threshold of 4 was chosen when the hash was assumed stable. It is not:
// stored values come from an algorithm that does not reproduce itself (see
// internal/images/phash), so the threshold needs re-validating once the corpus
// backfill has run.
func HammingDistance(a, b uint64) int {
	diff := a ^ b
	count := 0
	for diff != 0 {
		count += int(diff & 1)
		diff >>= 1
	}
	return count
}

// DuplicateThreshold is the maximum Hamming distance at which an upload is
// considered a duplicate.
const DuplicateThreshold = 4
