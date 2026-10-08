// CDN client: manifest and chunk fetches against one host. URLs follow
// SteamKit2's Client: manifest `depot/{id}/manifest/{gid}/5/{code}`
// (code omitted when zero) and chunk `depot/{id}/chunk/{hex}`; the scheme
// is http or https per the server entry, and a CDN auth token (from
// ContentServerDirectory.GetCDNAuthToken) goes in as the query string.
// Timeouts mirror SteamKit's: 10 s per request, 60 s for the body.
package depot

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Host is one download source. BaseURL is the origin
// (e.g. https://cache1-par1.steamcontent.com or a fake CDN's
// http://127.0.0.1:port); Name identifies it for tokens and stats;
// Token is the optional CDN auth token for this host.
type Host struct {
	Name    string
	BaseURL string
	Token   string
}

// Client fetches manifests and chunks. One Client serves all hosts; the
// scheduler keeps one Transport per host for keep-alive HTTP/1.1.
type Client struct {
	http *http.Client
}

// NewClient builds a client with keep-alive and the Steam-like timeouts.
func NewClient() *Client {
	tr := &http.Transport{
		MaxIdleConns:        64,
		MaxIdleConnsPerHost: 8,
		IdleConnTimeout:     90 * time.Second,
	}
	return &Client{http: &http.Client{Transport: tr}}
}

func (c *Client) url(h Host, path string) string {
	base := strings.TrimSuffix(h.BaseURL, "/")
	u := base + "/" + strings.TrimPrefix(path, "/")
	if h.Token != "" {
		u += "?" + h.Token
	}
	return u
}

func (c *Client) get(ctx context.Context, url string) ([]byte, error) {
	return c.getWithTimeouts(ctx, url, 10*time.Second, 60*time.Second)
}

const maxBody = 64 << 20 // chunks are ~1 MiB; manifests stay small

func (c *Client) getWithTimeouts(ctx context.Context, url string, headerTimeout, bodyTimeout time.Duration) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, headerTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, &HostError{Status: resp.StatusCode, Msg: fmt.Sprintf("host answered %d", resp.StatusCode)}
	}
	// Read the body with a timeout that cannot leak: on expiry close the
	// body to unblock the reader, then wait for it before returning.
	// Limit to maxBody+1 so over-long bodies error instead of truncating.
	type res struct {
		data []byte
		err  error
	}
	done := make(chan res, 1)
	go func() {
		limited := io.LimitReader(resp.Body, maxBody+1)
		data, err := io.ReadAll(limited)
		done <- res{data: data, err: err}
	}()
	timer := time.NewTimer(bodyTimeout)
	defer timer.Stop()
	select {
	case <-timer.C:
		resp.Body.Close() // unblock the reader; it then exits
		r := <-done
		_ = r
		return nil, &HostError{Timeout: true, Msg: "body timed out"}
	case r := <-done:
		if r.err != nil {
			return nil, r.err
		}
		if len(r.data) > maxBody {
			return nil, fmt.Errorf("body exceeds %d bytes", maxBody)
		}
		return r.data, nil
	}
}

// HostError marks failures for the scheduler's health tracking: timeouts,
// 5xx and bad checksums penalise and rest a host; its queued chunks move
// to others.
type HostError struct {
	Status  int
	Timeout bool
	Msg     string
}

func (e *HostError) Error() string { return e.Msg }

// DownloadManifest fetches one manifest file's bytes.
func (c *Client) DownloadManifest(ctx context.Context, h Host, depotID uint32, manifestID uint64, requestCode uint64) ([]byte, error) {
	path := fmt.Sprintf("depot/%d/manifest/%d/%d", depotID, manifestID, ManifestVersion)
	if requestCode != 0 {
		path += fmt.Sprintf("/%d", requestCode)
	}
	return c.get(ctx, c.url(h, path))
}

// DownloadChunk fetches one encrypted chunk's bytes. ChunkID is the raw
// SHA-1; the hex form goes in the URL.
func (c *Client) DownloadChunk(ctx context.Context, h Host, depotID uint32, chunkID []byte) ([]byte, error) {
	var hex [40]byte
	const digits = "0123456789abcdef"
	for i, b := range chunkID {
		hex[2*i] = digits[b>>4]
		hex[2*i+1] = digits[b&0xF]
	}
	return c.get(ctx, c.url(h, fmt.Sprintf("depot/%d/chunk/%s", depotID, hex[:])))
}
