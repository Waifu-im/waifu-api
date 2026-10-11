package storage

import (
	"strings"
	"testing"
)

// TestProviderSettings pins the per-provider behaviour.
//
// These defaults encode workarounds for real providers failing against the plain
// AWS defaults. Changing one without a reason would reintroduce a bug that is
// invisible until an upload starts failing in production.
func TestProviderSettings(t *testing.T) {
	cases := []struct {
		provider Provider
		want     providerSettings
	}{
		{
			provider: ProviderR2,
			// R2 needs the "auto" region and rejects the SDK's default
			// checksum headers.
			want: providerSettings{regionOverride: "auto", disableRequestChecksum: true},
		},
		{
			provider: ProviderMinIO,
			// MinIO resolves buckets by path, not by wildcard subdomain.
			want: providerSettings{forcePathStyle: true},
		},
		{
			provider: ProviderIDrive,
			want:     providerSettings{forcePathStyle: true},
		},
		{
			// AWS is the reference implementation and needs nothing.
			provider: ProviderAWS,
			want:     providerSettings{},
		},
		{
			provider: ProviderGeneric,
			want:     providerSettings{},
		},
		{
			// An unrecognised name falls through rather than failing, so a
			// typo cannot block startup.
			provider: Provider("nonsense"),
			want:     providerSettings{},
		},
	}

	for _, c := range cases {
		t.Run(string(c.provider), func(t *testing.T) {
			got := settingsFor(c.provider)
			if got.regionOverride != c.want.regionOverride {
				t.Errorf("regionOverride = %q, want %q", got.regionOverride, c.want.regionOverride)
			}
			if got.forcePathStyle != c.want.forcePathStyle {
				t.Errorf("forcePathStyle = %v, want %v", got.forcePathStyle, c.want.forcePathStyle)
			}
			if got.disableRequestChecksum != c.want.disableRequestChecksum {
				t.Errorf("disableRequestChecksum = %v, want %v",
					got.disableRequestChecksum, c.want.disableRequestChecksum)
			}
		})
	}
}

// TestProviderIsCaseInsensitive keeps configuration forgiving about casing.
func TestProviderIsCaseInsensitive(t *testing.T) {
	for _, name := range []string{"r2", "R2", "R2 "} {
		if !settingsFor(Provider(name)).disableRequestChecksum {
			t.Errorf("provider %q should resolve to the R2 settings", name)
		}
	}
}

// TestNormalizeEndpoint covers the accepted endpoint forms.
func TestNormalizeEndpoint(t *testing.T) {
	cases := map[string]string{
		"":                            "",
		"   ":                         "",
		"https://s3.fr-par.scw.cloud": "https://s3.fr-par.scw.cloud",
		"https://s3.example.com/":     "https://s3.example.com",
		"https://s3.example.com///":   "https://s3.example.com",
		// A bare host is assumed to be TLS, which every supported provider is.
		"s3.example.com":                     "https://s3.example.com",
		"accountid.r2.cloudflarestorage.com": "https://accountid.r2.cloudflarestorage.com",
		// http is preserved so a local MinIO over plain HTTP still works.
		"http://localhost:9000": "http://localhost:9000",
	}

	for input, want := range cases {
		if got := normalizeEndpoint(input); got != want {
			t.Errorf("normalizeEndpoint(%q) = %q, want %q", input, got, want)
		}
	}
}

// TestNewS3StoreRequiresEverySetting checks the fail-fast behaviour.
//
// A missing value must be reported at startup. Constructing a client with a
// half-empty configuration would fail later, on the first upload, which is a
// much worse time to discover it.
func TestNewS3StoreRequiresEverySetting(t *testing.T) {
	complete := Config{
		Provider:   ProviderGeneric,
		AccessKey:  "access",
		SecretKey:  "secret",
		ServiceURL: "https://s3.example.com",
		Region:     "us-east-1",
		BucketName: "bucket",
	}

	if _, err := NewS3Store(complete); err != nil {
		t.Fatalf("a complete configuration was rejected: %v", err)
	}

	cases := map[string]func(*Config){
		"S3__AccessKey":  func(c *Config) { c.AccessKey = "" },
		"S3__SecretKey":  func(c *Config) { c.SecretKey = "" },
		"S3__ServiceUrl": func(c *Config) { c.ServiceURL = "" },
		"S3__BucketName": func(c *Config) { c.BucketName = "" },
	}

	for name, remove := range cases {
		t.Run(name, func(t *testing.T) {
			cfg := complete
			remove(&cfg)

			_, err := NewS3Store(cfg)
			if err == nil {
				t.Fatal("an incomplete configuration should be rejected")
			}
			if !strings.Contains(err.Error(), name) {
				t.Errorf("error should name the missing variable %s, got: %v", name, err)
			}
		})
	}
}

// TestR2SuppliesItsOwnRegion covers the case where R2 has no configured region.
func TestR2SuppliesItsOwnRegion(t *testing.T) {
	_, err := NewS3Store(Config{
		Provider:   ProviderR2,
		AccessKey:  "access",
		SecretKey:  "secret",
		ServiceURL: "accountid.r2.cloudflarestorage.com",
		// Region deliberately empty: R2 uses "auto".
		BucketName: "images",
	})
	if err != nil {
		t.Fatalf("R2 without an explicit region should be accepted: %v", err)
	}
}

// TestNormalizeExtension protects the object key.
//
// The key format is part of the contract with the Cloudflare Worker, so a
// crafted filename must not be able to escape it or produce a key the Worker
// cannot serve.
func TestNormalizeExtension(t *testing.T) {
	cases := map[string]string{
		".jpg":     ".jpg",
		".JPG":     ".jpg",
		"jpg":      ".jpg",
		"  .png  ": ".png",
		"":         ".jpg",
		".":        ".jpg",
		"..":       ".jpg",
		// Path separators must never reach the key.
		"../../etc": ".jpg",
		"/abs/path": ".jpg",
		"a/b":       ".jpg",
		"a\\b":      ".jpg",
	}

	for input, want := range cases {
		if got := NormalizeExtension(input); got != want {
			t.Errorf("NormalizeExtension(%q) = %q, want %q", input, got, want)
		}
	}
}

// TestObjectKeyFormat pins the key layout the Worker reconstructs.
func TestObjectKeyFormat(t *testing.T) {
	if got := ObjectKey(42, ".jpg"); got != "42.jpg" {
		t.Errorf("ObjectKey(42, \".jpg\") = %q, want \"42.jpg\"", got)
	}
	if got := ObjectKey(1, ".webp"); got != "1.webp" {
		t.Errorf("ObjectKey(1, \".webp\") = %q, want \"1.webp\"", got)
	}
}

// TestSortStrings keeps error messages stable between runs.
func TestSortStrings(t *testing.T) {
	values := []string{"S3__Region", "S3__AccessKey", "S3__BucketName"}
	sortStrings(values)

	want := []string{"S3__AccessKey", "S3__BucketName", "S3__Region"}
	for i := range want {
		if values[i] != want[i] {
			t.Fatalf("sorted = %v, want %v", values, want)
		}
	}
}
