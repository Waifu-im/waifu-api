package models

import "testing"

// TestHashBitStringRoundTrip checks the bit(64) conversion against hand-computed
// values.
//
// A silent failure here would make duplicate detection fail open, so the
// expected values are written out explicitly rather than derived from the
// implementation.
func TestHashBitStringRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		hash uint64
		bits string
	}{
		{"zero", 0x0000000000000000, "0000000000000000000000000000000000000000000000000000000000000000"},
		{"all ones", 0xFFFFFFFFFFFFFFFF, "1111111111111111111111111111111111111111111111111111111111111111"},
		{"msb only", 0x8000000000000000, "1000000000000000000000000000000000000000000000000000000000000000"},
		{"lsb only", 0x0000000000000001, "0000000000000000000000000000000000000000000000000000000000000001"},
		{
			"sample",
			0xD1D52E2FD0D10ED1,
			"1101000111010101001011100010111111010000110100010000111011010001",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := PerceptualHashToBitString(c.hash)
			if got != c.bits {
				t.Errorf("PerceptualHashToBitString(%016X) = %q, want %q", c.hash, got, c.bits)
			}

			back, err := PerceptualHashFromBitString(got)
			if err != nil {
				t.Fatalf("PerceptualHashFromBitString: %v", err)
			}
			if back != c.hash {
				t.Errorf("round trip: %016X -> %q -> %016X", c.hash, got, back)
			}
		})
	}
}

// TestPerceptualHashFromBitStringRejectsBadInput covers the failure modes that
// would otherwise be silent.
func TestPerceptualHashFromBitStringRejectsBadInput(t *testing.T) {
	cases := map[string]string{
		"empty":      "",
		"too short":  "1010",
		"too long":   "00000000000000000000000000000000000000000000000000000000000000000",
		"bad symbol": "110100011101010100101110001011111101000011010001000011101101000x",
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := PerceptualHashFromBitString(input); err == nil {
				t.Errorf("input %q should have been rejected", input)
			}
		})
	}
}

// TestPerceptualHashFromBytes covers the alternative decoding path.
func TestPerceptualHashFromBytes(t *testing.T) {
	// 8 raw bytes, big endian.
	got, err := PerceptualHashFromBytes([]byte{0xD1, 0xD5, 0x2E, 0x2F, 0xD0, 0xD1, 0x0E, 0xD1})
	if err != nil {
		t.Fatalf("PerceptualHashFromBytes: %v", err)
	}
	if got != 0xD1D52E2FD0D10ED1 {
		t.Errorf("got %016X, want D1D52E2FD0D10ED1", got)
	}

	// 64 bit-characters are also accepted.
	bits := "1101000111010101001011100010111111010000110100010000111011010001"
	got, err = PerceptualHashFromBytes([]byte(bits))
	if err != nil {
		t.Fatalf("PerceptualHashFromBytes with bit string: %v", err)
	}
	if got != 0xD1D52E2FD0D10ED1 {
		t.Errorf("got %016X, want D1D52E2FD0D10ED1", got)
	}

	if _, err := PerceptualHashFromBytes([]byte{1, 2, 3}); err == nil {
		t.Error("a 3-byte input should be rejected")
	}
}

// TestHammingDistance covers the duplicate detection metric.
func TestHammingDistance(t *testing.T) {
	cases := []struct {
		a, b uint64
		want int
	}{
		{0, 0, 0},
		{0xFFFFFFFFFFFFFFFF, 0xFFFFFFFFFFFFFFFF, 0},
		{0, 0xFFFFFFFFFFFFFFFF, 64},
		{1, 0, 1},
		{0x0F0F0F0F0F0F0F0F, 0, 32},
		// The real case from the spike corpus: two favicons that the C# service
		// considered duplicates at distance 2.
		{0x92B32F9E943A92A6, 0x92B32F9C94BA92A6, 2},
	}
	for _, c := range cases {
		if got := HammingDistance(c.a, c.b); got != c.want {
			t.Errorf("HammingDistance(%016X, %016X) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

// TestDuplicateThresholdClassification checks the boundary the upload handler
// relies on: distance <= 4 is a duplicate, 5 is not.
func TestDuplicateThresholdClassification(t *testing.T) {
	base := uint64(0)

	for distance := 0; distance <= 8; distance++ {
		// Build a hash exactly `distance` bits away from base.
		candidate := uint64(0)
		for i := 0; i < distance; i++ {
			candidate |= 1 << uint(63-i)
		}

		isDuplicate := HammingDistance(candidate, base) <= DuplicateThreshold
		wantDuplicate := distance <= DuplicateThreshold

		if isDuplicate != wantDuplicate {
			t.Errorf("distance %d: duplicate = %v, want %v", distance, isDuplicate, wantDuplicate)
		}
	}
}

// TestParseReviewableType covers the wire values the frontend sends.
func TestParseReviewableType(t *testing.T) {
	for input, want := range map[string]ReviewableType{
		"Image":  TargetImage,
		"Tag":    TargetTag,
		"Artist": TargetArtist,
	} {
		got, err := ParseReviewableType(input)
		if err != nil {
			t.Errorf("ParseReviewableType(%q): %v", input, err)
			continue
		}
		if got != want {
			t.Errorf("ParseReviewableType(%q) = %v, want %v", input, got, want)
		}
	}

	if _, err := ParseReviewableType("image"); err == nil {
		t.Error("lowercase \"image\" should be rejected; the contract is exact")
	}
	if _, err := ParseReviewableType("Album"); err == nil {
		t.Error("an unknown type should be rejected")
	}
}

// TestParseBannerVariant pins the wire names and the rejection of unknown ones.
func TestParseBannerVariant(t *testing.T) {
	// JsonStringEnumConverter resolves enum names case-insensitively, so the
	// lower-case spellings the frontend uses must parse to the same member.
	for input, want := range map[string]BannerVariant{
		"Info":     VariantInfo,
		"Success":  VariantSuccess,
		"Warning":  VariantWarning,
		"Danger":   Danger,
		"info":     VariantInfo,
		"success":  VariantSuccess,
		"warning":  VariantWarning,
		"danger":   Danger,
		"WARNING":  VariantWarning,
		" Warning": VariantWarning,
	} {
		got, ok := ParseBannerVariant(input)
		if !ok {
			t.Errorf("ParseBannerVariant(%q) rejected a known variant", input)
			continue
		}
		if got != want {
			t.Errorf("ParseBannerVariant(%q) = %v, want %v", input, got, want)
		}
	}

	// The C# validator marks this field IsInEnum, so an unknown style is a
	// 400 rather than a silent fall back to Info.
	for _, input := range []string{"Nonsense", "", "0", "infoo"} {
		if got, ok := ParseBannerVariant(input); ok {
			t.Errorf("ParseBannerVariant(%q) = %v, want it rejected", input, got)
		}
	}
}

// TestBannerVariantWireNames pins the strings the frontend matches against.
func TestBannerVariantWireNames(t *testing.T) {
	cases := map[BannerVariant]string{
		VariantInfo:    "Info",
		VariantSuccess: "Success",
		VariantWarning: "Warning",
		Danger:         "Danger",
	}
	for variant, want := range cases {
		if got := variant.String(); got != want {
			t.Errorf("BannerVariant(%d).String() = %q, want %q", int(variant), got, want)
		}
	}
}
