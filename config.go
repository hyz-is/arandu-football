package football

import (
	"fmt"
	"net/http"
	"time"
)

const (
	// DefaultBaseURL is the service's address.
	DefaultBaseURL = "https://v3.football.api-sports.io"
	// DefaultTimeout bounds one request when Config.Timeout is empty.
	DefaultTimeout = 15 * time.Second
)

// Config is what [New] needs.
type Config struct {
	// APIKey is the account's key, sent as x-apisports-key. It belongs to the
	// account that holds the subscription.
	APIKey string
	// BaseURL is the service's address; empty means [DefaultBaseURL]. A test
	// points it at a local server.
	BaseURL string
	// HTTPClient makes the requests; nil means a client with Timeout.
	HTTPClient *http.Client
	// Timeout bounds one request; empty means [DefaultTimeout].
	Timeout time.Duration
}

// String formats the configuration without the key.
func (c Config) String() string {
	return fmt.Sprintf("football.Config{APIKey:%s BaseURL:%q Timeout:%s}", redact(c.APIKey), c.BaseURL, c.Timeout)
}

// GoString formats the configuration without the key, for %#v.
func (c Config) GoString() string { return c.String() }

func redact(key string) string {
	if key == "" {
		return `""`
	}
	return "[redacted]"
}
