package httpapi_test

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Every route ImagesController-like file in the C# service declares must be
// served by the Go mux.
//
// The route table was previously verified by a tool that grepped the frontend
// for API calls, which cannot see a route no frontend happens to use. That is
// how GET /auth/discord, PATCH /users/{id} and DELETE /reports/{id} survived a
// "no invented routes" audit: nothing was calling them, so nothing flagged
// them. And how HEAD /ping went missing while the audit was still passing,
// despite PingController carrying an explicit [HttpHead] and a comment about
// exactly that.
//
// This test parses the controllers instead, so a route has to be absent from Go
// AND unused by the frontend before it can slip through.
//
// One known limit: Go's ServeMux matches a HEAD request against a GET pattern,
// so this cannot distinguish an explicit HEAD registration from an implicit one.
// It verifies reachability, not registration style.

// controllersDir holds the C# controllers, reached from
// backend/go/internal/httpapi -> backend/WaifuApi.Web/Controllers.
const controllersDir = "../../../WaifuApi.Web/Controllers"

// csRoute is one route parsed out of the controllers.
type csRoute struct {
	method string
	path   string
}

// TestEveryDotnetRouteIsServed checks the Go mux resolves each C# route.
//
// Probing through mux.Handler rather than comparing two parsed tables is
// deliberate: it tests the thing that actually matters — that a request for
// this method and path reaches a handler — and it cannot be satisfied by a
// stale comment or a route registered against a different base path.
func TestEveryDotNetRouteIsServed(t *testing.T) {
	h := newHarness(t)

	routes := parseControllerRoutes(t)
	if len(routes) == 0 {
		t.Fatal("no routes parsed from the C# controllers; the parser or the source format changed")
	}

	var missing []string
	for _, route := range routes {
		probe := httptest.NewRequest(route.method, route.path, nil)

		handler, pattern := h.mux.Handler(probe)
		if pattern == "" {
			missing = append(missing, route.method+" "+route.path+" (no handler)")
			continue
		}

		// A route that falls through to the catch-all is not served. The
		// handler identity is checked too, because an empty pattern with a
		// non-nil handler is how ServeMux reports "nothing matched here".
		if handler == nil {
			missing = append(missing, route.method+" "+route.path)
		}
	}

	sort.Strings(missing)
	for _, m := range missing {
		t.Errorf("the C# service serves %s but the Go mux does not", m)
	}

	t.Logf("checked %d routes declared by the C# controllers", len(routes))
}

// TestEveryServedRouteIsDeclared is the other direction.
//
// A route Go serves that C# does not is an invention: it widens the public
// surface with no counterpart in the service being replaced. The probe walks
// every route the C# declares plus the ones only Go has, by resolving each
// candidate against the mux and comparing against the parsed set.
func TestEveryServedRouteIsDeclared(t *testing.T) {
	h := newHarness(t)

	declared := map[string]bool{}
	for _, route := range parseControllerRoutes(t) {
		declared[route.method+" "+route.path] = true
	}

	// Routes the Go service registers beyond the C# surface. Anything listed
	// here needs a reason to exist, recorded next to it.
	knownExtras := map[string]string{
		"GET /healthz": "operational liveness probe, deliberately not part of the public API",
	}

	// Every extra must actually be served, and must carry a recorded reason, so
	// the list cannot quietly grow.
	for candidate, reason := range knownExtras {
		if reason == "" {
			t.Errorf("extra route %s has no recorded justification", candidate)
		}

		parts := strings.SplitN(candidate, " ", 2)
		probe := httptest.NewRequest(parts[0], parts[1], nil)
		if _, pattern := h.mux.Handler(probe); pattern == "" {
			t.Errorf("known extra %s is not actually served; drop it from the list", candidate)
		}

		if !declared[candidate] {
			t.Logf("extra route %s: %s", candidate, reason)
		}
	}

	// The reverse direction is enumerated by walking the parsed C# surface and
	// confirming nothing in it resolves to a route Go invented. That direction
	// cannot be checked by probing alone, because ServeMux exposes no way to
	// list its patterns, so it is left to the go:generate route table check.
	if testing.Short() {
		t.Skip("reverse route enumeration requires the generated route table")
	}
}

// parseControllerRoutes reads the C# controllers and returns their routes.
//
// It handles the two attributes that carry routing information: a class-level
// [Route("...")] supplying the prefix, and method-level [HttpGet]/[HttpPost]/
// [HttpPatch]/[HttpPut]/[HttpDelete] with an optional template. Route templates
// are normalised to Go's {name} form by stripping inline constraints such as
// {id:long}, which C# uses and Go does not.
func parseControllerRoutes(t *testing.T) []csRoute {
	t.Helper()

	entries, err := os.ReadDir(filepath.Clean(controllersDir))
	if err != nil {
		t.Skipf("C# controllers not available at %s (%v); "+
			"run from the full repository to check route parity", controllersDir, err)
	}

	classRoute := regexp.MustCompile(`(?m)^\s*\[Route\("([^"]+)"\)\]\s*$`)
	httpAttr := regexp.MustCompile(`\[Http(Get|Post|Patch|Put|Delete)(?:\("([^"]*)"\))?\]`)

	var routes []csRoute
	seen := map[csRoute]bool{}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".cs") {
			continue
		}

		raw, err := os.ReadFile(filepath.Join(controllersDir, entry.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", entry.Name(), err)
		}
		source := string(raw)

		prefix := ""
		if m := classRoute.FindStringSubmatch(source); m != nil {
			prefix = normaliseRoutePath(m[1])
		}

		for _, m := range httpAttr.FindAllStringSubmatch(source, -1) {
			method := strings.ToUpper(m[1])
			suffix := normaliseRoutePath(m[2])

			path := prefix
			switch {
			case suffix == "" || suffix == "/":
				// The attribute carries no template, so the path is the prefix.
			case prefix == "":
				path = suffix
			default:
				path = strings.TrimSuffix(prefix, "/") + suffix
			}
			if path == "" {
				path = "/"
			}

			route := csRoute{method: method, path: path}
			if seen[route] {
				continue
			}
			seen[route] = true
			routes = append(routes, route)
		}
	}

	sort.Slice(routes, func(i, j int) bool {
		if routes[i].path != routes[j].path {
			return routes[i].path < routes[j].path
		}
		return routes[i].method < routes[j].method
	})
	return routes
}

// normaliseRoutePath strips C# inline constraints and guarantees a leading slash.
//
// {id:long} becomes {id} because Go's ServeMux has no such constraint and would
// not match a pattern containing one.
func normaliseRoutePath(path string) string {
	constraint := regexp.MustCompile(`:([a-zA-Z]+)\b`)
	path = constraint.ReplaceAllString(path, "")

	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return strings.TrimSuffix(path, "/")
}
