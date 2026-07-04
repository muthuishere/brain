package ingest

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"time"
)

// Fetch retrieves a URL's body and content type, identifying itself as brain.
func Fetch(rawURL string) (body []byte, contentType string, err error) {
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", "brain")

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, "", fmt.Errorf("fetch %s: status %d", rawURL, resp.StatusCode)
	}
	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}
	return body, resp.Header.Get("Content-Type"), nil
}

// CrawlResult is one fetched page during a Crawl.
type CrawlResult struct {
	URL         string
	Body        []byte
	ContentType string
}

// CrawlOpts bounds a Crawl's breadth-first walk.
type CrawlOpts struct {
	MaxPages int // default 50
	MaxDepth int // default 3
}

var hrefRe = regexp.MustCompile(`(?i)<a[^>]+href=["']([^"'#]+)`)

type queueItem struct {
	url   string
	depth int
}

// Crawl walks same-host links breadth-first from seedURL, up to MaxPages pages
// and MaxDepth link-hops deep. Fetch failures on individual pages are
// swallowed — a crawl never aborts because one page failed.
func Crawl(seedURL string, opts CrawlOpts) []CrawlResult {
	maxPages := opts.MaxPages
	if maxPages <= 0 {
		maxPages = 50
	}
	maxDepth := opts.MaxDepth
	if maxDepth <= 0 {
		maxDepth = 3
	}

	seed, err := url.Parse(seedURL)
	if err != nil {
		return nil
	}
	host := seed.Host

	seen := map[string]bool{seedURL: true}
	queue := []queueItem{{url: seedURL, depth: 0}}
	var results []CrawlResult

	for len(queue) > 0 && len(results) < maxPages {
		item := queue[0]
		queue = queue[1:]

		body, contentType, err := Fetch(item.url)
		if err != nil {
			continue // per-page failures don't abort the crawl
		}
		results = append(results, CrawlResult{URL: item.url, Body: body, ContentType: contentType})

		if item.depth >= maxDepth {
			continue
		}
		for _, link := range extractLinks(item.url, body) {
			linkURL, err := url.Parse(link)
			if err != nil || linkURL.Host != host {
				continue
			}
			linkURL.Fragment = ""
			normalized := linkURL.String()
			if seen[normalized] {
				continue
			}
			seen[normalized] = true
			queue = append(queue, queueItem{url: normalized, depth: item.depth + 1})
		}
	}
	return results
}

// extractLinks pulls absolute, fragment-stripped <a href> targets from HTML,
// resolving relative links against base.
func extractLinks(base string, html []byte) []string {
	baseURL, err := url.Parse(base)
	if err != nil {
		return nil
	}
	var links []string
	for _, m := range hrefRe.FindAllSubmatch(html, -1) {
		ref, err := url.Parse(string(m[1]))
		if err != nil {
			continue
		}
		links = append(links, baseURL.ResolveReference(ref).String())
	}
	return links
}
