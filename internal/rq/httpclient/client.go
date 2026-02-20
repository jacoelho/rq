package httpclient

import (
	"crypto/tls"
	"net"
	"net/http"
	"time"
)

// New creates a tuned HTTP client for rq execution.
func New(tlsConfig *tls.Config, timeout time.Duration) *http.Client {
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	transport := &http.Transport{
		Proxy:                  http.ProxyFromEnvironment,
		DialContext:            dialer.DialContext,
		TLSClientConfig:        tlsConfig,
		TLSHandshakeTimeout:    10 * time.Second,
		ResponseHeaderTimeout:  10 * time.Second,
		ExpectContinueTimeout:  1 * time.Second,
		IdleConnTimeout:        60 * time.Second,
		MaxIdleConns:           100,
		MaxIdleConnsPerHost:    10,
		MaxConnsPerHost:        50,
		MaxResponseHeaderBytes: 1 << 20, // 1 MiB
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
}
