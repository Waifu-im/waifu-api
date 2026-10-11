package httpapi_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"waifuapi/internal/config"
)

// Coverage for GET /users, which had almost none.
//
// It is an administrator endpoint that exposes per-user counters and the
// blacklist reason, so the things worth testing are who may call it, how the
// two filters behave, and the pagination contract. Those are exactly the parts
// that were unexercised.

func seedUsers(t *testing.T, h *harness, roles ...config.Role) []int64 {
	t.Helper()
	ctx := context.Background()

	ids := make([]int64, 0, len(roles))
	for i, role := range roles {
		id := int64(1000 + i)
		_, err := h.db.Pool().Exec(ctx, `
            INSERT INTO "Users" ("Id", "DiscordId", "Name", "Role")
            VALUES ($1, $2, $3, $4)
            ON CONFLICT ("Id") DO NOTHING`,
			id, fmt.Sprintf("discord-seed-%d", i), fmt.Sprintf("seeded-%d", i), int(role))
		if err != nil {
			t.Fatalf("seed user %d: %v", id, err)
		}
		ids = append(ids, id)
	}
	return ids
}

// TestListUsersRequiresAdmin is the authorisation check. The endpoint exposes
// other people's Discord ids and blacklist reasons, so a non-admin reaching it
// would be a disclosure bug rather than a cosmetic one.
func TestListUsersRequiresAdmin(t *testing.T) {
	h := newHarness(t)
	seedUsers(t, h, config.RoleUser)

	roles := []config.Role{
		config.RoleUser,
		config.RoleTrustedUser,
		config.RoleModerator,
	}
	for _, role := range roles {
		t.Run(role.String(), func(t *testing.T) {
			token := h.issueToken(2, role)
			resp := h.request(http.MethodGet, "/users", token, nil, "")
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusForbidden {
				t.Errorf("status = %d, want 403 for %s", resp.StatusCode, role)
			}
		})
	}

	t.Run("admin is allowed", func(t *testing.T) {
		token := h.issueToken(2, config.RoleAdmin)
		resp := h.request(http.MethodGet, "/users", token, nil, "")
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want 200 for Admin: %s", resp.StatusCode, readAll(t, resp))
		}
	})
}

func TestListUsersRequiresAuthentication(t *testing.T) {
	h := newHarness(t)
	resp := h.request(http.MethodGet, "/users", "", nil, "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 without a token", resp.StatusCode)
	}
}

// TestListUsersReturnsPaginatedEnvelope checks the envelope shape, which the
// frontend types as PaginatedList<UserDto>.
func TestListUsersReturnsPaginatedEnvelope(t *testing.T) {
	h := newHarness(t)
	seedUsers(t, h, config.RoleUser, config.RoleUser, config.RoleUser)

	token := h.issueToken(2, config.RoleAdmin)
	resp := h.request(http.MethodGet, "/users", token, nil, "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d: %s", resp.StatusCode, readAll(t, resp))
	}
	payload := readAll(t, resp)

	for _, field := range []string{"items", "totalCount", "pageNumber", "totalPages"} {
		if !jsonHas(payload, field) {
			t.Errorf("envelope is missing %q", field)
		}
	}

	// The harness pre-creates 100 users, so the default page of 30 cannot be
	// assumed to contain anything seeded here. Assert on the count instead, and
	// use includedIds below to assert on identity.
	if total := jsonIntOrZero(t, payload, "totalCount"); total < 3 {
		t.Errorf("totalCount = %d, want at least the seeded users", total)
	}
}

// TestListUsersNameFilter covers the search box.
func TestListUsersNameFilter(t *testing.T) {
	h := newHarness(t)
	seedUsers(t, h, config.RoleUser, config.RoleUser, config.RoleUser)

	token := h.issueToken(2, config.RoleAdmin)

	t.Run("narrowing", func(t *testing.T) {
		resp := h.request(http.MethodGet, "/users?name=seeded-1", token, nil, "")
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d: %s", resp.StatusCode, readAll(t, resp))
		}
		payload := readAll(t, resp)

		// A name filter is applied before paging, so the match is total here.
		if total := jsonIntOrZero(t, payload, "totalCount"); total != 1 {
			t.Errorf("totalCount = %d, want exactly the one matching user", total)
		}
		if !jsonContains(payload, `"seeded-1"`) {
			t.Error("filtered response does not include seeded-1")
		}
		if jsonContains(payload, `"seeded-0"`) {
			t.Error("filtered response includes seeded-0")
		}
	})

	t.Run("blank matches everything", func(t *testing.T) {
		resp := h.request(http.MethodGet, "/users?name=", token, nil, "")
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d: %s", resp.StatusCode, readAll(t, resp))
		}
		// No filter, so every user is eligible; the default page is 30.
		if total := jsonIntOrZero(t, readAll(t, resp), "totalCount"); total < 3 {
			t.Errorf("totalCount = %d, want every user", total)
		}
	})
}

// TestListUsersIncludedIDs covers the picker resolution, which is deliberately
// not paginated: one page holds everything matched.
func TestListUsersIncludedIDs(t *testing.T) {
	h := newHarness(t)
	ids := seedUsers(t, h, config.RoleUser, config.RoleUser, config.RoleUser, config.RoleUser)

	token := h.issueToken(2, config.RoleAdmin)
	query := fmt.Sprintf("?includedIds=%d,%d", ids[0], ids[2])

	resp := h.request(http.MethodGet, "/users"+query, token, nil, "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d: %s", resp.StatusCode, readAll(t, resp))
	}
	payload := readAll(t, resp)

	if !jsonContains(payload, fmt.Sprintf("seeded-%d", 0)) {
		t.Error("response is missing the first requested user")
	}
	if !jsonContains(payload, fmt.Sprintf("seeded-%d", 2)) {
		t.Error("response is missing the third requested user")
	}
	if jsonContains(payload, `"seeded-1"`) {
		t.Error("response includes a user that was not requested")
	}
	if jsonContains(payload, `"seeded-3"`) {
		t.Error("response includes a user that was not requested")
	}

	// An includedIds lookup reports totalCount as the number actually returned,
	// so the sidebar can show "N selected" without a second request.
	if got := jsonIntOrZero(t, payload, "totalCount"); got != 2 {
		t.Errorf("totalCount = %d, want 2 for an exact lookup", got)
	}
}

// TestListUsersNormalisesPagination pins the query-parameter contract.
//
// PaginationUtils.Normalize in the C# service clamps rather than rejects, so
// page=0 and a negative pageSize are accepted and adjusted. A 400 here would be
// a divergence from the behaviour clients already rely on.
func TestListUsersNormalisesPagination(t *testing.T) {
	h := newHarness(t)
	token := h.issueToken(2, config.RoleAdmin)

	for _, query := range []string{"?page=0", "?pageSize=0", "?pageSize=-5", "?page=1&pageSize=5000"} {
		t.Run(query, func(t *testing.T) {
			resp := h.request(http.MethodGet, "/users"+query, token, nil, "")
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want 200 for %s: %s", resp.StatusCode, query, readAll(t, resp))
			}
			payload := readAll(t, resp)
			if pageNumber := jsonIntOrZero(t, payload, "pageNumber"); pageNumber < 1 {
				t.Errorf("pageNumber = %d, want a normalised value of at least 1", pageNumber)
			}
		})
	}
}

// TestListUsersIgnoresUnparseableIncludedIDs documents current behaviour: a
// non-numeric id is dropped rather than rejected, so the query degrades to the
// ordinary listing. Pinned so a future change to reject it is a deliberate edit.
func TestListUsersIgnoresUnparseableIncludedIDs(t *testing.T) {
	h := newHarness(t)
	token := h.issueToken(2, config.RoleAdmin)

	resp := h.request(http.MethodGet, "/users?includedIds=abc", token, nil, "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", resp.StatusCode, readAll(t, resp))
	}
}

// TestUserCountsAreReportedPerUser exercises the aggregate columns, which are
// derived in SQL and so are easy to get subtly wrong.
func TestUserCountsAreReportedPerUser(t *testing.T) {
	h := newHarness(t)
	userID := seedUsers(t, h, config.RoleUser)[0]

	// One accepted image, so uploadedImageCount should be 1 for that user.
	imageID := int64(4242)
	_, err := h.db.Pool().Exec(context.Background(), `
        INSERT INTO "Images" ("Id", "PerceptualHash", "Extension", "DominantColor",
                               "IsNsfw", "IsAnimated", "Width", "Height", "ByteSize",
                               "UploaderId", "ReviewStatus")
        VALUES ($1, B'0000000000000000000000000000000000000000000000000000000000000000',
                '.png', '#000000', false, false, 800, 1000, 1000, $2, 1)`,
		imageID, userID)
	if err != nil {
		t.Fatalf("seed image: %v", err)
	}

	token := h.issueToken(2, config.RoleAdmin)
	resp := h.request(http.MethodGet, fmt.Sprintf("/users?includedIds=%d", userID), token, nil, "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d: %s", resp.StatusCode, readAll(t, resp))
	}

	payload := readAll(t, resp)
	if got := jsonIntOrZero(t, payload, "uploadedImageCount"); got != 1 {
		t.Errorf("uploadedImageCount = %d, want 1", got)
	}

	// The counters live on each item rather than the envelope, and must always
	// be present even at zero: the frontend renders them unconditionally.
	for _, field := range []string{
		"uploadedImageCount", "albumImageCount",
		"jwtRequestCount", "apiKeyRequestCount", "requestCount",
	} {
		if !jsonItemHas(payload, field) {
			t.Errorf("user dto item is missing %q", field)
		}
	}
}

// jsonHas reports whether a top-level key is present in a response body.
// jsonItemHas reports whether a key is present on the first element of items.
func jsonItemHas(payload []byte, field string) bool {
	var envelope struct {
		Items []map[string]json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return false
	}
	if len(envelope.Items) == 0 {
		return false
	}
	_, ok := envelope.Items[0][field]
	return ok
}

func jsonHas(payload []byte, field string) bool {
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return false
	}
	_, ok := decoded[field]
	return ok
}

func jsonContains(payload []byte, needle string) bool {
	return strings.Contains(string(payload), needle)
}
