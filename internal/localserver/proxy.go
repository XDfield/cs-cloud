package localserver

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"cs-cloud/internal/logger"
)

func (s *Server) handleProxy(w http.ResponseWriter, r *http.Request) {
	endpoint := s.manager.Endpoint()
	if endpoint == "" {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "no agent backend available")
		return
	}

	targetURL, err := url.Parse(endpoint)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL", "invalid backend endpoint")
		return
	}

	backend := s.manager.DefaultBackend()
	d, ok := s.manager.GetDriver(backend)
	if !ok {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "no driver for backend: "+backend)
		return
	}

	var rewriteFunc func(map[string]string) string
	for _, rt := range d.ProxyRoutes() {
		if r.Method != rt.Method {
			continue
		}
		cleanPath := strings.TrimPrefix(r.URL.Path, "/api/v1")
		if matchRoute(cleanPath, rt.Prefix) {
			rewriteFunc = rt.Rewrite
			break
		}
	}

	if rewriteFunc == nil {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "no proxy route for "+r.URL.Path)
		return
	}

	pathValues := extractPathValues(r)
	target := rewriteFunc(pathValues)

	targetAddr := targetURL.Scheme + "://" + targetURL.Host + target
	logger.Info("proxy %s %s -> %s", r.Method, r.URL.Path, targetAddr)

	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	headerMap := d.HeaderMap()

	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.URL.Path = target
		req.URL.RawPath = ""
		req.Host = targetURL.Host
		for from, to := range headerMap {
			if v := req.Header.Get(from); v != "" {
				req.Header.Set(to, v)
			}
		}
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		logger.Error("proxy %s %s -> %s failed: %v", r.Method, r.URL.Path, targetAddr, err)
		http.Error(w, "bad gateway", http.StatusBadGateway)
	}

	proxy.ModifyResponse = func(resp *http.Response) error {
		logger.Info("proxy %s %s -> %s %d", r.Method, r.URL.Path, targetAddr, resp.StatusCode)
		return nil
	}

	proxy.ServeHTTP(w, r)
}

func extractPathValues(r *http.Request) map[string]string {
	vals := make(map[string]string)
	for _, key := range []string{"id"} {
		if v := r.PathValue(key); v != "" {
			vals[key] = v
		}
	}
	return vals
}

func matchRoute(path, pattern string) bool {
	patternParts := strings.Split(strings.Trim(pattern, "/"), "/")
	pathParts := strings.Split(strings.Trim(path, "/"), "/")

	if len(pathParts) < len(patternParts) {
		return false
	}

	for i, pp := range patternParts {
		if strings.HasPrefix(pp, "{") && strings.HasSuffix(pp, "}") {
			continue
		}
		if pp != pathParts[i] {
			return false
		}
	}

	if len(patternParts) > 0 && !strings.Contains(patternParts[len(patternParts)-1], "{") &&
		len(pathParts) > len(patternParts) {
		return false
	}

	return len(pathParts) == len(patternParts)
}
