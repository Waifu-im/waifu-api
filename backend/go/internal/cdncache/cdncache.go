// Package cdncache invalidates the Cloudflare edge cache for specific objects.
//
// # Why it exists
//
// Images are served through Cdn__BaseUrl, which points at a Cloudflare Worker
// rather than at storage. That means deleting an object from S3 or replacing its
// bytes does not change what visitors see: the edge keeps serving the copy it
// cached until the TTL expires. The C# service has always had this problem too,
// and the same workaround — CloudflareCacheService — is what makes a delete or a
// replacement take effect immediately.
//
// # It is deliberately optional
//
// With no Cloudflare__ZoneId or Cloudflare__ApiToken configured, every method is
// a no-op. That is not a fallback for a broken configuration; it is the normal
// state for a self-hosted instance that does not use the edge at all, and
// requiring the credentials would make the service unstartable for those
// deployments.
package cdncache

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// maxURLsPerPurge is Cloudflare's per-request ceiling.
const maxURLsPerPurge = 30

// Config describes the edge and how image variants are addressed.
//
// The transform settings mirror the frontend's cfImage helper. Both sides have to
// agree on the exact variant URL, because the purge is keyed on the cached URL:
// a purge for a differently-spelled variant misses the cache entry and the stale
// image stays.
type Config struct {
	ZoneID  string
	Token   string
	BaseURL string

	TransformEnabled bool
	TransformPath    string
	WidthParam       string
	CardWidth        int
	PreviewWidth     int
}

// IsConfigured reports whether purging is possible at all.
func (c Config) IsConfigured() bool {
	return c.ZoneID != "" && c.Token != "" && c.BaseURL != ""
}

// Purger invalidates cached URLs.
//
// The HTTP client is injected rather than constructed so the caller controls
// timeouts, and so tests can substitute a stub without reaching the network.
type Purger struct {
	config Config
	client *http.Client
	log    Logger
}

// Logger is the small logging surface this package needs.
//
// It is an interface so the package does not depend on the logging package, which
// keeps it testable with a two-line fake.
type Logger interface {
	Warn(msg string, err error, args ...any)
}

// New builds a Purger.
func New(config Config, client *http.Client, log Logger) *Purger {
	if client == nil {
		// Cloudflare's own purge API is fast, but a hung connection would tie up
		// the request goroutine of whoever triggered the deletion. Ten seconds is
		// long enough to succeed and short enough not to be noticed.
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &Purger{config: config, client: client, log: log}
}

// PurgeImage invalidates one image's original and its resized variants.
//
// extensions may hold more than one value when a replacement changed the file
// extension: the old object is then a second thing that needs purging.
func (p *Purger) PurgeImage(ctx context.Context, imageID int64, extensions []string) {
	names := make([]string, 0, len(extensions))
	for _, extension := range extensions {
		if extension == "" {
			continue
		}
		names = append(names, strconv.FormatInt(imageID, 10)+extension)
	}
	p.PurgeFiles(ctx, names)
}

// PurgeFiles invalidates the given object names.
//
// Nothing happens when Cloudflare is not configured, or when the name list is
// empty after filtering.
func (p *Purger) PurgeFiles(ctx context.Context, names []string) {
	if p == nil || !p.config.IsConfigured() {
		return
	}

	urls := p.buildURLs(names)
	if len(urls) == 0 {
		return
	}

	// Cloudflare accepts at most 30 URLs per call, so a bulk cleanup is split.
	for start := 0; start < len(urls); start += maxURLsPerPurge {
		end := start + maxURLsPerPurge
		if end > len(urls) {
			end = len(urls)
		}
		p.sendPurge(ctx, urls[start:end])
	}
}

// buildURLs expands object names into the exact URLs the edge may have cached.
//
// The original comes first and always; variants are only produced when image
// transforms are enabled, because otherwise those URLs were never cached and
// purging them would be a wasted call.
func (p *Purger) buildURLs(names []string) []string {
	seen := make(map[string]bool)
	var urls []string

	add := func(candidate string) {
		if candidate == "" || seen[candidate] {
			return
		}
		seen[candidate] = true
		urls = append(urls, candidate)
	}

	for _, name := range names {
		if name == "" {
			continue
		}
		original := strings.TrimRight(p.config.BaseURL, "/") + "/" + name
		add(original)

		if !p.config.TransformEnabled || p.config.TransformPath == "" {
			continue
		}
		for _, width := range p.widths() {
			add(p.resizeURL(original, width))
		}
	}
	return urls
}

// widths returns the configured variant widths, de-duplicated and in a stable
// order so the generated purge request is deterministic.
func (p *Purger) widths() []int {
	seen := map[int]bool{}
	var out []int
	for _, width := range []int{p.config.CardWidth, p.config.PreviewWidth} {
		if width <= 0 || seen[width] {
			continue
		}
		seen[width] = true
		out = append(out, width)
	}
	return out
}

// resizeURL builds a transformed variant URL.
//
// The separator logic matches the frontend's cfImage helper: the transform path
// may already end in a separator or already contain its own "k=v" pair, and the
// naive concatenation produces a URL that does not match the cached key.
func (p *Purger) resizeURL(original string, width int) string {
	parsed, err := url.Parse(original)
	if err != nil {
		return ""
	}

	path := p.config.TransformPath
	last := path[len(path)-1:]

	separator := ""
	switch {
	case last == "," || last == "/":
		// The path already terminates itself.
	case strings.Contains(path, "="):
		separator = ","
	default:
		separator = "/"
	}

	param := p.config.WidthParam
	if param == "" {
		param = "width"
	}

	return parsed.Scheme + "://" + parsed.Host +
		path + separator + param + "=" + strconv.Itoa(width) + parsed.Path
}

// sendPurge issues one purge request.
//
// Every failure is logged and swallowed. A purge that fails leaves a stale image
// at the edge until the TTL expires, which is a cosmetic problem; failing the
// deletion that triggered it would report a rollback that did not happen.
func (p *Purger) sendPurge(ctx context.Context, urls []string) {
	body, err := json.Marshal(map[string][]string{"files": urls})
	if err != nil {
		p.log.Warn("encode cloudflare purge request", err)
		return
	}

	endpoint := "https://api.cloudflare.com/client/v4/zones/" + p.config.ZoneID + "/purge_cache"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		p.log.Warn("build cloudflare purge request", err)
		return
	}
	request.Header.Set("Authorization", "Bearer "+p.config.Token)
	request.Header.Set("Content-Type", "application/json")

	response, err := p.client.Do(request)
	if err != nil {
		p.log.Warn("cloudflare cache purge failed", err, "urls", len(urls))
		return
	}
	defer response.Body.Close()

	if response.StatusCode >= 300 {
		p.log.Warn("cloudflare cache purge rejected", nil,
			"status", response.Status, "urls", len(urls))
	}
}

// purgeResponse is the shape Cloudflare returns, used only for diagnostics.
type purgeResponse struct {
	Success bool `json:"success"`
	Errors  []struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"errors"`
}
