package httpapi_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"waifuapi/internal/config"
)

// These cover the endpoints added after the upload workflow: OAuth login,
// API keys, albums, reports and user administration.

// TestCallbackRejectsMissingCode checks validation before any Discord call.
func TestCallbackRejectsMissingCode(t *testing.T) {
	h := newHarness(t)

	resp := h.request("POST", "/auth/discord", "",
		bytesReader(`{}`), "application/json")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

// TestMeRequiresAuthentication guards the profile endpoint.
func TestMeRequiresAuthentication(t *testing.T) {
	h := newHarness(t)

	resp := h.request("GET", "/users/me", "", nil, "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

// TestMeReturnsTheCaller checks the profile reflects the token.
func TestMeReturnsTheCaller(t *testing.T) {
	h := newHarness(t)

	token := h.issueToken(20, config.RoleTrustedUser)
	resp := h.request("GET", "/users/me", token, nil, "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", resp.StatusCode, readAll(t, resp))
	}

	body := readAll(t, resp)
	if got := jsonString(t, body, "name"); got != "user-20" {
		t.Errorf("name = %q, want user-20", got)
	}

	// The role comes from the DATABASE, not from the token, so a promotion takes
	// effect on the next request rather than waiting for the token to expire.
	// The seeded role for user 20 is User, so that is what must come back.
	if got := jsonString(t, body, "role"); got != "User" {
		t.Errorf("role = %q, want User (the stored role, not the token role)", got)
	}
}

// TestPermissionsReflectsConfiguration checks the banner permission is derived
// server-side, so changing Permissions__BannerManagementMinRole takes effect
// without a frontend deploy.
func TestPermissionsReflectsConfiguration(t *testing.T) {
	h := newHarness(t)

	// A user cannot manage the banner under the default Moderator minimum.
	user := h.issueToken(21, config.RoleUser)
	resp := h.request("GET", "/users/me/permissions", user, nil, "")
	defer resp.Body.Close()

	if jsonBool(t, readAll(t, resp), "canManageBanner") {
		t.Error("canManageBanner should be false for a user")
	}

	// A moderator can.
	mod := h.issueToken(22, config.RoleModerator)
	resp2 := h.request("GET", "/users/me/permissions", mod, nil, "")
	defer resp2.Body.Close()

	if !jsonBool(t, readAll(t, resp2), "canManageBanner") {
		t.Error("canManageBanner should be true for a moderator")
	}
}

// TestAPIKeyLifecycle covers creation, listing and deletion.
//
// The plaintext must be returned exactly once, at creation.
func TestAPIKeyLifecycle(t *testing.T) {
	h := newHarness(t)

	token := h.issueToken(23, config.RoleUser)

	// AuthController.CreateApiKey uses CreatedAtAction, so this is 201 with a
	// Location header, not 200.
	created := h.request("POST", "/auth/api-keys", token,
		bytesReader(`{"description":"laptop"}`), "application/json")
	if created.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d: %s", created.StatusCode, readAll(t, created))
	}
	if created.Header.Get("Location") == "" {
		t.Error("CreatedAtAction sets a Location header; it is missing")
	}

	createdBody := readAll(t, created)
	plaintext := jsonString(t, createdBody, "key")
	if plaintext == "" {
		t.Fatal("the plaintext key must be returned once at creation")
	}
	keyID, err := jsonInt(t, createdBody, "id")
	if err != nil {
		t.Fatalf("no key id: %v", err)
	}

	// Listing must never include the plaintext.
	list := h.request("GET", "/auth/api-keys", token, nil, "")
	defer list.Body.Close()
	if containsBytes(readAll(t, list), plaintext) {
		t.Error("the plaintext key leaked into the key listing")
	}

	// The stored key must be a hash, not the plaintext.
	assertKeyHashed(t, h, plaintext)

	// Deletion.
	deleted := h.request("DELETE", "/auth/api-keys/"+itoa(keyID), token, nil, "")
	defer deleted.Body.Close()
	if deleted.StatusCode != http.StatusNoContent {
		t.Errorf("delete status = %d, want 204", deleted.StatusCode)
	}

	after := h.request("GET", "/auth/api-keys", token, nil, "")
	defer after.Body.Close()
	if containsBytes(readAll(t, after), `"id":`+itoa(keyID)) {
		t.Error("the deleted key is still listed")
	}
}

// assertKeyHashed checks the stored form is not the plaintext.
func assertKeyHashed(t *testing.T, h *harness, plaintext string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var stored string
	err := h.db.Pool().QueryRow(ctx, `SELECT "KeyHash" FROM "ApiKeys" LIMIT 1`).Scan(&stored)
	if err != nil {
		t.Fatalf("read stored hash: %v", err)
	}

	if stored == plaintext {
		t.Error("the API key was stored in plaintext")
	}
	if len(stored) != 64 {
		t.Errorf("stored hash length = %d, want 64 hex characters of SHA-256", len(stored))
	}
}

// TestAPIKeyDeleteIsOwnerScoped checks another user cannot delete a key.
func TestAPIKeyDeleteIsOwnerScoped(t *testing.T) {
	h := newHarness(t)

	owner := h.issueToken(24, config.RoleUser)
	created := h.request("POST", "/auth/api-keys", owner,
		bytesReader(`{"description":"owned"}`), "application/json")
	keyID, err := jsonInt(t, readAll(t, created), "id")
	if err != nil {
		t.Fatalf("no key id: %v", err)
	}

	// A different user attempts the delete.
	intruder := h.issueToken(25, config.RoleUser)
	resp := h.request("DELETE", "/auth/api-keys/"+itoa(keyID), intruder, nil, "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404 for a key owned by someone else", resp.StatusCode)
	}

	// It must still exist.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var count int
	if err := h.db.Pool().QueryRow(ctx,
		`SELECT COUNT(*) FROM "ApiKeys" WHERE "Id" = $1`, keyID).Scan(&count); err != nil {
		t.Fatalf("count keys: %v", err)
	}
	if count != 1 {
		t.Error("another user was able to delete the key")
	}
}

// TestCreateAPIKeyRequiresDescription checks validation.
func TestCreateAPIKeyRequiresDescription(t *testing.T) {
	h := newHarness(t)

	token := h.issueToken(26, config.RoleUser)
	resp := h.request("POST", "/auth/api-keys", token,
		bytesReader(`{"description":"  "}`), "application/json")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", resp.StatusCode)
	}
}

// TestCreateAPIKeyRejectsNegativeExpiry checks a typo cannot silently create a
// permanent key.
func TestCreateAPIKeyRejectsNegativeExpiry(t *testing.T) {
	h := newHarness(t)

	token := h.issueToken(27, config.RoleUser)
	resp := h.request("POST", "/auth/api-keys", token,
		bytesReader(`{"description":"x","expiresInDays":-30}`), "application/json")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}
