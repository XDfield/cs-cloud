package platform

import (
	"crypto/tls"
	"net/http"
	"os"
	"sync"
)

var (
	insecureOnce  sync.Once
	insecureClient *http.Client
)

func InsecureSkipTLSVerify() bool {
	return os.Getenv("COSTRICT_INSECURE_SKIP_TLS_VERIFY") == "true" ||
		os.Getenv("COSTRICT_INSECURE_SKIP_TLS_VERIFY") == "1"
}

func HTTPClient() *http.Client {
	if !InsecureSkipTLSVerify() {
		return http.DefaultClient
	}
	insecureOnce.Do(func() {
		insecureClient = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
		}
	})
	return insecureClient
}
