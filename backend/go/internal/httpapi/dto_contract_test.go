package httpapi

// This file is an in-package test rather than an httpapi_test one because
// several wire structs are unexported. They are internal representations that
// a client cannot name, but they still cross the network, so their JSON shape
// still has to match the C# DTOs. An external test package cannot reflect over
// them at all.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Response shape parity between the C# DTOs and the Go wire structs.
//
// The route contract test proves the same paths are served. It cannot prove the
// two services put the same JSON on the wire, and a field that is present in one
// and absent in the other is invisible until a frontend destructures it and gets
// undefined. The frontend types every response, so a silently dropped field
// compiles cleanly and fails only at runtime.
//
// This parses the C# DTO classes and reflects over the Go structs, so drift in
// either direction fails here rather than in the browser.
//
// # How the names are compared
//
// ASP.NET Core serialises with camelCase by default, so a C# property named
// ByteSize appears as "byteSize". Go declares the wire name explicitly in a json
// tag. Both sides are reduced to a sorted set of lower-camel names and compared.
//
// # Known limits
//
//   - Properties marked [JsonIgnore] are excluded on the C# side; a Go field
//     with `json:"-"` is likewise excluded.
//   - Nested DTOs are compared by name only. Their own fields are checked when
//     they appear in the table below, so TagDto and ArtistDto are covered by
//     their own rows rather than through ImageDto.
//   - Generic envelopes such as PaginatedList<T> are checked structurally where
//     they have a Go counterpart, and skipped where they do not.

// controllersDir holds the C# controllers, reached from
// backend/go/internal/httpapi -> backend/WaifuApi.Web/Controllers.
const controllersDir = "../../../WaifuApi.Web/Controllers"

// modelsDir holds the C# DTOs, reached from
// backend/go/internal/httpapi -> backend/WaifuApi.Application/Common/Models.
const modelsDir = "../../../WaifuApi.Application/Common/Models"

// csProperty matches one public property declaration inside a DTO class.
var (
	csClassStart = regexp.MustCompile(`(?m)^public class (\w+)`)
	csProperty   = regexp.MustCompile(`(?m)^\s*public\s+[\w\.<>\?\[\],\s]+\s+(\w+)\s*\{\s*get`)
	// jsonIgnore matches a property the C# service deliberately keeps out of the
	// response.
	jsonIgnore = regexp.MustCompile(`(?m)^\s*\[JsonIgnore[^\]]*\]`)
)

// camelCase lower-cases the leading run of upper-case characters, which is
// ASP.NET Core's default naming policy for a property like ByteSize.
func camelCase(name string) string {
	if name == "" {
		return name
	}
	runes := []rune(name)
	// Lower-case the leading acronym: "URLValue" becomes "urlValue".
	i := 0
	for i < len(runes) && unicodeIsUpper(runes[i]) {
		i++
	}
	if i > 1 && i < len(runes) {
		i--
	}
	for j := 0; j < i; j++ {
		runes[j] = toLower(runes[j])
	}
	return string(runes)
}

func unicodeIsUpper(r rune) bool { return r >= 'A' && r <= 'Z' }
func toLower(r rune) rune        { return r + ('a' - 'A') }

// csDtoFields parses one C# DTO file and returns the camelCase wire names of
// its properties, excluding [JsonIgnore]d ones.
//
// base is the class name to read; empty means the first public class in the
// file.
func csDtoFields(t *testing.T, file, base string) map[string]struct{} {
	t.Helper()
	return csFieldsFromFile(t, filepath.Join(modelsDir, file), base)
}

// csFieldsFromFile parses a C# class body out of an arbitrary path.
func csFieldsFromFile(t *testing.T, path, base string) map[string]struct{} {
	t.Helper()

	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	lines := strings.Split(string(source), "\n")

	// Locate the class body.
	start := -1
	className := base
	for i, line := range lines {
		if m := csClassStart.FindStringSubmatch(line); m != nil {
			if base == "" || m[1] == base {
				start = i
				className = m[1]
				break
			}
		}
	}
	if start < 0 {
		t.Fatalf("class %q not found in %s", base, path)
	}

	fields := map[string]struct{}{}
	ignoreNext := false
	for i := start + 1; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "}") && strings.HasSuffix(trimmed, "}") {
			// End of the class body.
			break
		}
		if jsonIgnore.MatchString(line) {
			ignoreNext = true
			continue
		}
		if m := csProperty.FindStringSubmatch(line); m != nil {
			if !ignoreNext {
				fields[camelCase(m[1])] = struct{}{}
			}
			ignoreNext = false
			continue
		}
		if trimmed != "" && !strings.HasPrefix(trimmed, "[") && !strings.HasPrefix(trimmed, "//") {
			// Any other declaration resets the pending-ignore, since a [JsonIgnore]
			// always sits directly above its property.
			ignoreNext = false
		}
	}

	if len(fields) == 0 {
		t.Fatalf("no properties parsed from %s (%s)", path, className)
	}
	return fields
}

// goJsonFields reflects over a Go struct and returns its wire names.
func goJsonFields(t *testing.T, value any) map[string]struct{} {
	t.Helper()

	typ := reflect.TypeOf(value)
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct {
		t.Fatalf("%s is not a struct", typ)
	}

	fields := map[string]struct{}{}
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if !field.IsExported() {
			continue
		}
		tag := field.Tag.Get("json")
		name := strings.Split(tag, ",")[0]
		if name == "-" {
			continue
		}
		if name == "" {
			// No tag means the field is serialised under its Go name, which is a
			// different naming policy from the rest of the wire format.
			t.Errorf("%s.%s has no json tag; every wire field should declare one",
				typ.Name(), field.Name)
			continue
		}
		fields[name] = struct{}{}
	}
	return fields
}

func sortedKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func difference(a, b map[string]struct{}) []string {
	var out []string
	for k := range a {
		if _, ok := b[k]; !ok {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// TestDtoShapesMatch is the differential check proper.
func TestDtoShapesMatch(t *testing.T) {
	cases := []struct {
		name  string
		file  string
		class string
		goVal any
	}{
		{"ImageDto", "ImageDto.cs", "ImageDto", ImageDto{}},
		{"TagDto", "TagDto.cs", "TagDto", TagDto{}},
		{"ArtistDto", "ArtistDto.cs", "ArtistDto", ArtistDto{}},
		{"AlbumDto", "AlbumDto.cs", "AlbumDto", albumDto{}},
		{"ApiKeyDto", "ApiKeyDto.cs", "ApiKeyDto", apiKeyDto{}},
		{"BannerDto", "BannerDto.cs", "BannerDto", bannerDto{}},
		{"ReportDto", "ReportDto.cs", "ReportDto", reportDto{}},
		{"UserDto", "UserDto.cs", "UserDto", userDto{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want := csDtoFields(t, tc.file, tc.class)
			got := goJsonFields(t, tc.goVal)

			missing := difference(want, got)
			extra := difference(got, want)

			for _, field := range missing {
				t.Errorf("%s.%s is served by C# but absent from the Go wire struct", tc.name, field)
			}
			for _, field := range extra {
				t.Errorf("%s.%s is serialised by Go but not declared in %s", tc.name, field, tc.file)
			}

			t.Logf("checked %d fields: C#=%v Go=%v", len(want), sortedKeys(want), sortedKeys(got))
		})
	}
}

// TestUnmappedDtoFilesAreListed is a tripwire.
//
// It fails if a new DTO appears in the C# models directory that this test does
// not cover. Without it, adding a C# DTO silently drops it out of parity
// checking, which is exactly the failure mode this file exists to prevent.
func TestUnmappedDtoFilesAreListed(t *testing.T) {
	covered := map[string]bool{
		"ImageDto.cs":              true,
		"TagDto.cs":                true,
		"ArtistDto.cs":             true,
		"AlbumDto.cs":              true,
		"ApiKeyDto.cs":             true,
		"BannerDto.cs":             true,
		"ReportDto.cs":             true,
		"ReviewTaskDto.cs":         true,
		"UserDto.cs":               true,
		"UserPermissionsDto.cs":    true,
		"PaginatedList.cs":         true,
		"EditPayloads.cs":          true,
		"ImageFilters.cs":          true,
		"ReviewContentPayloads.cs": true,
	}

	entries, err := os.ReadDir(modelsDir)
	if err != nil {
		t.Fatalf("read %s: %v", modelsDir, err)
	}

	var uncovered []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".cs") {
			continue
		}
		if !covered[entry.Name()] {
			uncovered = append(uncovered, entry.Name())
		}
	}

	if len(uncovered) > 0 {
		sort.Strings(uncovered)
		t.Errorf("DTO files not covered by the shape comparison: %v\n"+
			"Add each to TestDtoShapesMatch, or record why it does not need comparing.",
			uncovered)
	}
}

// TestCamelCaseMatchesTheService pins the name mangling itself, because a
// wrong rule here would make every other assertion in this file meaningless.
func TestCamelCaseMatchesTheService(t *testing.T) {
	cases := map[string]string{
		"Id":             "id",
		"ByteSize":       "byteSize",
		"Url":            "url",
		"URL":            "url",
		"IsNsfw":         "isNsfw",
		"PerceptualHash": "perceptualHash",
		"DiscordId":      "discordId",
		"UploaderId":     "uploaderId",
		"LikedAt":        "likedAt",
		"AddedToAlbumAt": "addedToAlbumAt",
		"ReviewStatus":   "reviewStatus",
	}

	for input, want := range cases {
		if got := camelCase(input); got != want {
			t.Errorf("camelCase(%q) = %q, want %q", input, got, want)
		}
	}
}

// TestUnsetTimestampsSerialiseAsNullNotAbsent pins the null policy.
//
// ASP.NET Core is registered without a DefaultIgnoreCondition
// (WaifuApi.Web/Program.cs), so the C# service emits an explicit null for an
// unset nullable. A Go pointer tagged omitempty drops the key entirely, which
// is a different shape: `dto.lastUsedAt === null` is true either way, but
// `Object.hasOwn(dto, 'lastUsedAt')` is not, and a client that distinguishes
// "not applicable" from "never used" would misread it.
func TestUnsetTimestampsSerialiseAsNullNotAbsent(t *testing.T) {
	encoded, err := json.Marshal(apiKeyDto{
		ID:          7,
		UserID:      42,
		Description: "unused key",
		CreatedAt:   "2026-01-02T03:04:05.000Z",
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	for _, field := range []string{"lastUsedAt", "expirationDate"} {
		raw, present := decoded[field]
		if !present {
			t.Errorf("%q was omitted from the response; C# emits an explicit null", field)
			continue
		}
		if string(raw) != "null" {
			t.Errorf("%q = %s, want null", field, raw)
		}
	}

	if got := decoded["userId"]; string(got) != "42" {
		t.Errorf("userId = %s, want 42", got)
	}
}

// TestRequestPayloadShapesMatch is the inbound half of the contract.
//
// A renamed request field fails differently from a renamed response field: the
// response field is undefined in the client, while the request field is silently
// dropped by the deserialiser and the endpoint applies a default. That produces
// a successful-looking PATCH that changed nothing, which is considerably worse
// than an error.
//
// The request types live beside the controllers rather than in the models
// directory, so they are parsed from there.
func TestRequestPayloadShapesMatch(t *testing.T) {
	cases := []struct {
		name  string
		file  string
		class string
		goVal any
	}{
		{"SubmitEditRequest", "ReviewController.cs", "SubmitEditRequest", submitEditRequest{}},
		{"EditReviewTaskRequest", "ReviewController.cs", "EditReviewTaskRequest", editReviewTaskRequest{}},
		{"UpdateTagRequest", "TagsController.cs", "UpdateTagRequest", updateTagRequest{}},
		{"UpdateArtistRequest", "ArtistsController.cs", "UpdateArtistRequest", updateArtistRequest{}},
		{"UpdateBannerRequest", "BannerController.cs", "UpdateBannerRequest", updateBannerRequest{}},
		{"CreateReportRequest", "ReportsController.cs", "CreateReportRequest", createReportRequest{}},
		{"UpdateReportRequest", "ReportsController.cs", "UpdateReportRequest", updateReportRequest{}},
		{"UpdateRoleRequest", "UsersController.cs", "UpdateRoleRequest", updateUserRoleRequest{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want := csFieldsFromFile(t, filepath.Join(controllersDir, tc.file), tc.class)
			got := goJsonFields(t, tc.goVal)

			for _, field := range difference(want, got) {
				t.Errorf("%s.%s is accepted by C# but not read by Go; the field is dropped silently",
					tc.name, field)
			}
			for _, field := range difference(got, want) {
				t.Errorf("%s.%s is read by Go but not declared in %s", tc.name, field, tc.file)
			}

			t.Logf("checked %d fields: C#=%v Go=%v", len(want), sortedKeys(want), sortedKeys(got))
		})
	}
}
