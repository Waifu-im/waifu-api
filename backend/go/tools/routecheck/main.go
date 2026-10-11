// Command routecheck compares the routes this service registers against the
// endpoints the React frontend actually calls.
//
// It exists because the route table cannot be reliably derived from the C#
// controllers by eye. Several frontend calls differ from what the controllers
// suggest: the API keys live under /auth/api-keys rather than /api-keys, album
// items are addressed as /images rather than /items, and the user role change is
// a PUT rather than a PATCH. Missing any of those produced a service that passed
// its own tests while the application could not reach it.
//
// Run:
//
//	go run ./tools/routecheck -frontend ../frontend/src
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// callPattern matches `api.<method>('<path>')` in TypeScript.
//
// The path may be a template literal with interpolations, which are reduced to a
// :param placeholder so paths can be compared structurally.
var callPattern = regexp.MustCompile(
	"api\\.(get|post|put|patch|delete)(?:<[^>]*>)?\\(\\s*[`'\"]([^`'\"]+)[`'\"]")

// interpolation matches ${...} inside a template literal.
var interpolation = regexp.MustCompile(`\$\{[^}]*\}`)

// dynamicPrefix matches the leading ${apiUserPath} segment specifically.
//
// This deliberately names the variable rather than matching any ${...}. A
// generic hook such as useResource interpolates its own ${endpoint} parameter,
// and treating that as apiUserPath invented routes against /users that no page
// ever calls — reported as missing, which was worse than not checking.
var dynamicPrefix = regexp.MustCompile(`^\$\{apiUserPath\}`)

// unresolvedPrefix marks a leading interpolation that is neither a known
// constant nor a path segment the tool can resolve, such as the ${endpoint}
// parameter of a generic resource hook.
//
// These are not missing routes. Whether the hook's update or delete is ever
// invoked is a call-graph question this tool cannot answer from text alone, so
// they are reported separately instead of failing the run.
var unresolvedPrefix = regexp.MustCompile(`^\$\{[^}]+\}`)

// route is one discovered endpoint.
type route struct {
	Method string
	Path   string
	Source string
}

// normalise converts a frontend URL into a comparable route pattern.
func normalise(raw string) string {
	// Strip the query string; routes match on the path alone.
	if index := strings.Index(raw, "?"); index >= 0 {
		raw = raw[:index]
	}

	// A leading ${apiUserPath} is one of /users/me or /users/{id}. Both are
	// served by the same wildcard, so it is rewritten to /users/{userId}.
	raw = dynamicPrefix.ReplaceAllString(raw, "/users/{userId}")

	// Any other leading interpolation is a value supplied by a caller, so the
	// concrete path is not knowable here. It is tagged rather than guessed at.
	raw = unresolvedPrefix.ReplaceAllString(raw, "{unresolved}")

	// Both sides of the comparison use different wildcard syntax: the frontend
	// interpolates ${...} while this codebase registers {name} segments. Both are
	// reduced to the same token so they compare equal.
	raw = interpolation.ReplaceAllString(raw, "{wildcard}")

	// Collapse an empty leading segment produced by the rewrite above.
	raw = strings.ReplaceAll(raw, "//", "/")
	if !strings.HasPrefix(raw, "/") {
		raw = "/" + raw
	}
	return raw
}

// discover walks the frontend source and returns every called route.
func discover(root string) ([]route, error) {
	var routes []route

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".ts") && !strings.HasSuffix(path, ".tsx") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		source := strings.TrimPrefix(path, root+string(filepath.Separator))
		for _, match := range callPattern.FindAllStringSubmatch(string(content), -1) {
			routes = append(routes, route{
				Method: strings.ToUpper(match[1]),
				Path:   normalise(match[2]),
				Source: source,
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return routes, nil
}

// handleFuncCall locates a mux.HandleFunc invocation in source.
var handleFuncCall = regexp.MustCompile(`HandleFunc\(`)

// stringLiteral extracts a double-quoted literal.
var stringLiteral = regexp.MustCompile(`"([^"]*)"`)

// registered reads the route table straight out of the source.
//
// The argument is parsed rather than pattern-matched because it is an
// expression, not a single string: routes are written as
// `mux.HandleFunc("GET "+base+"tags", h)`. A naive regex captures the empty
// string between the quotes, which then normalises to "/" and matches every
// route in the frontend, so the whole check silently passes.
func registered(root string) (map[string]bool, error) {
	registeredRoutes := make(map[string]bool)

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		source := string(content)

		for _, location := range handleFuncCall.FindAllStringIndex(source, -1) {
			argument, ok := firstArgument(source, location[1])
			if !ok {
				continue
			}

			method, routePath, ok := parseRouteArgument(argument)
			if !ok || routePath == "" {
				continue
			}

			// A route registered without a method prefix, such as
			// mux.HandleFunc(base+"healthz", h), answers every method. It is
			// recorded under a wildcard so it satisfies any call on that path.
			if method == "" {
				method = "*"
			}
			registeredRoutes[method+" "+routePath] = true
		}
		return nil
	})
	return registeredRoutes, err
}

// firstArgument extracts the text of the call's first argument, honouring
// nesting so a call inside a call is not truncated.
func firstArgument(source string, start int) (string, bool) {
	// The scan begins just after "HandleFunc(", so the opening parenthesis has
	// already been consumed. Starting at zero would mean the terminating comma
	// is never seen at depth 1, and the argument would run on to the end of the
	// file, swallowing the handler name and every string literal after it.
	depth := 1
	inString := false
	var builder strings.Builder

	for i := start; i < len(source); i++ {
		char := source[i]

		if inString {
			builder.WriteByte(char)
			if char == '"' && (i == 0 || source[i-1] != '\\') {
				inString = false
			}
			continue
		}

		switch char {
		case '"':
			inString = true
			builder.WriteByte(char)
		case '(':
			depth++
			if depth == 1 {
				continue
			}
			builder.WriteByte(char)
		case ')':
			depth--
			if depth == 0 {
				return builder.String(), true
			}
			builder.WriteByte(char)
		case ',':
			if depth == 1 {
				return builder.String(), true
			}
			builder.WriteByte(char)
		default:
			builder.WriteByte(char)
		}
	}
	return "", false
}

// parseRouteArgument splits a route expression into its method and path.
//
// Two shapes appear in this codebase:
//
//	"GET "+base+"tags"          -> GET, /tags
//	base+"healthz"             -> no method, /healthz
func parseRouteArgument(argument string) (method, path string, ok bool) {
	literals := stringLiteral.FindAllStringSubmatch(argument, -1)
	if len(literals) == 0 {
		return "", "", false
	}

	first := literals[0][1]
	if strings.HasPrefix(first, "GET ") || strings.HasPrefix(first, "POST ") ||
		strings.HasPrefix(first, "PUT ") || strings.HasPrefix(first, "PATCH ") ||
		strings.HasPrefix(first, "DELETE ") {

		parts := strings.SplitN(first, " ", 2)
		method = parts[0]
		if len(parts) == 2 {
			path = parts[1]
		}

		// The remaining literals are path segments appended after base.
		for _, literal := range literals[1:] {
			if literal[1] == "base" {
				continue
			}
			path += literal[1]
		}
	} else {
		for _, literal := range literals {
			if literal[1] == "base" {
				continue
			}
			path += literal[1]
		}
	}

	// Registered wildcards are {name}; rename them so both sides agree.
	path = renameWildcards(path)

	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return method, path, true
}

// renameWildcards rewrites every {name} segment to a common token.
func renameWildcards(path string) string {
	segments := strings.Split(path, "/")
	for i, segment := range segments {
		if strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}") {
			segments[i] = "{wildcard}"
		}
	}
	return strings.Join(segments, "/")
}

// patternMatches reports whether a registered route serves a called path.
//
// Go's ServeMux matches on segment count, with a trailing wildcard absorbing the
// rest, so the comparison has to model that rather than being a string equality.
func patternMatches(registeredPattern, calledPath string) bool {
	if registeredPattern == calledPath {
		return true
	}

	// "/" is not a catch-all. Registering a bare slash means the base path
	// resolved to nothing, which is a parsing failure rather than a route that
	// serves everything.
	if registeredPattern == "/" || registeredPattern == "" {
		return false
	}

	// Only a trailing wildcard may absorb extra segments. Without this rule
	// "GET /images" would wrongly satisfy "GET /images/{id}", because a
	// prefix comparison stops early.
	if strings.HasSuffix(registeredPattern, "/") && strings.HasPrefix(calledPath, registeredPattern) {
		return true
	}

	registeredSegments := strings.Split(strings.Trim(registeredPattern, "/"), "/")
	calledSegments := strings.Split(strings.Trim(calledPath, "/"), "/")

	// Segment counts must otherwise agree exactly.
	if len(registeredSegments) != len(calledSegments) {
		return false
	}

	for i, segment := range registeredSegments {
		if strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}") {
			// A wildcard matches any single segment, including the literal "me"
			// that the frontend uses in place of a numeric user id.
			continue
		}

		// The called path may itself be a wildcard, because the frontend builds
		// it from a variable. Reports.tsx dispatches
		// PATCH /reports/${id}/${action} where action is one of a fixed set, and
		// the service registers the expanded forms. The question this tool asks
		// is whether some registered route could serve the call, so a literal is
		// accepted here: if the caller passes "validate", that route serves it.
		if strings.HasPrefix(calledSegments[i], "{") && strings.HasSuffix(calledSegments[i], "}") {
			continue
		}

		if segment != calledSegments[i] {
			return false
		}
	}
	return true
}

func main() {
	frontendDir := flag.String("frontend", "../frontend/src", "frontend source directory")
	backendDir := flag.String("backend", "internal/httpapi", "directory holding the route registrations")
	flag.Parse()

	called, err := discover(*frontendDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "discover frontend routes: %v\n", err)
		os.Exit(1)
	}

	registeredRoutes, err := registered(*backendDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read registered routes: %v\n", err)
		os.Exit(1)
	}

	// Collapse duplicates so the report lists each route once.
	unique := make(map[string]route)
	for _, r := range called {
		key := r.Method + " " + r.Path
		if _, seen := unique[key]; !seen {
			unique[key] = r
		}
	}

	keys := make([]string, 0, len(unique))
	for key := range unique {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var missing []string
	var unresolved []string
	covered := 0

	for _, key := range keys {
		method := strings.SplitN(key, " ", 2)[0]
		path := strings.SplitN(key, " ", 2)[1]

		found := false
		for registered := range registeredRoutes {
			registeredMethod := strings.TrimSuffix(strings.SplitN(registered, " ", 2)[0], "")
			registeredPath := strings.TrimPrefix(registered, registeredMethod+" ")

			if registeredMethod != "*" && registeredMethod != method {
				continue
			}
			if patternMatches(registeredPath, path) {
				found = true
				break
			}
		}

		if strings.Contains(path, "{unresolved}") {
			// Not a proven gap. The path is assembled from a value supplied by
			// the caller, so whether it resolves to something the service serves
			// cannot be decided from the text, and the helper holding it may never
			// be invoked at all.
			unresolved = append(unresolved, key)
			continue
		}

		if found {
			covered++
		} else {
			missing = append(missing, key)
		}
	}

	fmt.Printf("frontend calls %d distinct routes; %d are served, %d are missing, %d unresolved\n\n",
		len(keys), covered, len(missing), len(unresolved))

	if len(missing) > 0 {
		fmt.Println("MISSING (the frontend calls these and would receive 404):")
		for _, key := range missing {
			fmt.Printf("  %-8s %s   [%s]\n",
				strings.SplitN(key, " ", 2)[0], strings.SplitN(key, " ", 2)[1],
				unique[key].Source)
		}
		os.Exit(1)
	}

	if len(unresolved) > 0 {
		fmt.Println("UNRESOLVED (path built from a caller-supplied value; check by hand):")
		for _, key := range unresolved {
			fmt.Printf("  %-8s %s   [%s]\n",
				strings.SplitN(key, " ", 2)[0], strings.SplitN(key, " ", 2)[1],
				unique[key].Source)
		}
		fmt.Println()
	}

	if len(missing) == 0 {
		fmt.Println("every route the frontend can be shown to call is served")
	}
}
