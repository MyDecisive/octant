package wrapper

import (
	"io"
	"net/http"
)

// HTTPClient is an interface for http.Client.
type HTTPClient interface {
	Get(url string) (*http.Response, error)
	Post(url, contentType string, body io.Reader) (*http.Response, error)
}
