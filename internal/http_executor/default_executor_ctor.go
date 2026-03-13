package http_executor

import (
	"net/http"
	"time"
)

// NewDefaultExecutor constructs a new HTTP executor with cookie jar enabled.
func NewDefaultExecutor() *DefaultExecutor {
	jar := NewManagedCookieJar()
	return &DefaultExecutor{
		jar: jar,
		client: &http.Client{
			Timeout: 60 * time.Second,
			Jar:     jar,
		},
	}
}
