// Package mapdata contains clients for the public map data services used to
// plan routes: Nominatim for place search, the elevation tiles and
// Open-Meteo for weather. Routing, trails and points of interest use the
// OpenStreetMap data imported by the internal/routing package.
package mapdata

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/bs-tools/trailrunn.ing/internal/elevation"
)

// UserAgent identifies the application to the public map data services, as
// required by their usage policies.
const UserAgent = "trail-tools/0.1 (+https://github.com/bs-tools/trailrunn.ing)"

// Config configures the map data clients. Zero values use the public
// services.
type Config struct {
	// The Nominatim base URL, e.g. https://nominatim.openstreetmap.org.
	NominatimURL string
	// Looks up ground elevations. Defaults to the public elevation tiles.
	Elevation Elevation
	// The Open-Meteo forecast API URL.
	ForecastURL string
	// The HTTP client to use. Required.
	HTTPClient *http.Client
}

func (c Config) withDefaults() Config {
	if c.NominatimURL == "" {
		c.NominatimURL = "https://nominatim.openstreetmap.org"
	}
	if c.ForecastURL == "" {
		c.ForecastURL = "https://api.open-meteo.com/v1/forecast"
	}
	if c.Elevation == nil {
		c.Elevation = elevation.NewDEM(elevation.DefaultURL, elevation.DefaultCacheDir, c.HTTPClient)
	}
	return c
}

// Client is a client for the map data services.
type Client struct {
	conf      Config
	nominatim *throttle
}

// New creates a new Client. It panics if conf has no HTTPClient.
func New(conf Config) *Client {
	if conf.HTTPClient == nil {
		panic("mapdata: Config.HTTPClient is required")
	}
	return &Client{
		conf: conf.withDefaults(),
		// Nominatim's usage policy allows at most 1 request per second.
		nominatim: newThrottle(time.Second),
	}
}

const (
	// connectAttempts is how many times a request is tried when no
	// connection to the service could be made.
	connectAttempts = 3
	// connectTimeout bounds making a connection, so that the attempts
	// together take about as long as the 10 seconds a single TLS handshake
	// may by default. A handshake that works takes well under a second.
	connectTimeout = 4 * time.Second
)

// do performs a request and decodes a JSON response into out.
//
// The services are free and shared, and Open-Meteo at times leaves a new
// connection's TLS handshake unanswered, which fails the first requests
// after a start, before there's a connection to reuse. Such a request never
// reached the service, so it's tried again on a new connection. Requests
// that did reach it aren't: an error it answers with is its answer.
func (c *Client) do(req *http.Request, out any) error {
	req.Header.Set("User-Agent", UserAgent)
	var err error
	for range connectAttempts {
		var connected bool
		connected, err = c.attempt(req, out)
		if err == nil || connected || req.Context().Err() != nil {
			return err
		}
	}
	return err
}

// attempt performs a request once, and reports whether it got a connection
// to send the request on.
func (c *Client) attempt(req *http.Request, out any) (connected bool, err error) {
	// Canceled when connecting takes too long, and once the response is
	// read.
	ctx, cancel := context.WithCancel(req.Context())
	defer cancel()
	var gotConn, timedOut atomic.Bool
	timer := time.AfterFunc(connectTimeout, func() {
		timedOut.Store(true)
		cancel()
	})
	defer timer.Stop()
	ctx = httptrace.WithClientTrace(ctx, &httptrace.ClientTrace{
		GotConn: func(httptrace.GotConnInfo) {
			// Not connected in time if the timer already fired.
			if timer.Stop() {
				gotConn.Store(true)
			}
		},
	})
	resp, err := c.conf.HTTPClient.Do(req.WithContext(ctx))
	if err != nil {
		if timedOut.Load() && !gotConn.Load() {
			return false, fmt.Errorf("connecting to %s took longer than %s", req.URL.Host, connectTimeout)
		}
		return gotConn.Load(), err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return true, &StatusError{StatusCode: resp.StatusCode, Body: plainText(string(body))}
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return true, fmt.Errorf("failed to decode response: %w", err)
	}
	return true, nil
}

// htmlTag matches HTML and XML tags.
var htmlTag = regexp.MustCompile(`<[^>]*>`)

// plainText strips markup from an error response body and shortens it, so
// it can be shown to users and agents.
func plainText(body string) string {
	if strings.Contains(body, "<") {
		// Drop the head, which only has the title and styles.
		if _, after, ok := strings.Cut(body, "</head>"); ok {
			body = after
		}
		body = htmlTag.ReplaceAllString(body, " ")
	}
	body = strings.Join(strings.Fields(body), " ")
	const maxLen = 300
	if len(body) > maxLen {
		body = body[:maxLen] + "..."
	}
	return body
}

// StatusError is returned when a service responds with an unexpected status.
type StatusError struct {
	StatusCode int
	Body       string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("unexpected status %d: %s", e.StatusCode, e.Body)
}

// throttle enforces a minimum interval between operations.
type throttle struct {
	tokens chan struct{}
}

func newThrottle(interval time.Duration) *throttle {
	t := &throttle{tokens: make(chan struct{}, 1)}
	t.tokens <- struct{}{}
	go func() {
		ticker := time.NewTicker(interval)
		for range ticker.C {
			select {
			case t.tokens <- struct{}{}:
			default:
			}
		}
	}()
	return t
}

func (t *throttle) wait(ctx context.Context) error {
	select {
	case <-t.tokens:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
