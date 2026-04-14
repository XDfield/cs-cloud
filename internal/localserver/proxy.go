package localserver

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

type route struct {
	method  string
	prefix  string
	rewrite func(r *http.Request) string
}

var proxyRoutes = []route{
	{http.MethodPost, "/conversations", rewriteTo("/session")},
	{http.MethodGet, "/conversations", rewriteTo("/session")},
	{http.MethodGet, "/conversations/status", rewriteTo("/session/status")},
	{http.MethodGet, "/conversations/{id}", rewriteSessionID("/session/")},
	{http.MethodPatch, "/conversations/{id}", rewriteSessionID("/session/")},
	{http.MethodDelete, "/conversations/{id}", rewriteSessionID("/session/")},
	{http.MethodPost, "/conversations/{id}/prompt", rewriteSessionID("/session/")},
	{http.MethodPost, "/conversations/{id}/prompt/async", rewriteSessionIDWithSuffix("/session/", "/prompt_async")},
	{http.MethodPost, "/conversations/{id}/abort", rewriteSessionIDWithSuffix("/session/", "/abort")},
	{http.MethodGet, "/conversations/{id}/messages", rewriteSessionIDWithSuffix("/session/", "/message")},
	{http.MethodGet, "/conversations/{id}/todo", rewriteSessionIDWithSuffix("/session/", "/todo")},
	{http.MethodGet, "/conversations/{id}/diff", rewriteSessionIDWithSuffix("/session/", "/diff")},
	{http.MethodPost, "/conversations/{id}/shell", rewriteSessionIDWithSuffix("/session/", "/shell")},
	{http.MethodPost, "/conversations/{id}/command", rewriteSessionIDWithSuffix("/session/", "/command")},
	{http.MethodGet, "/permissions", rewriteTo("/permission")},
	{http.MethodPost, "/permissions/{id}/reply", rewritePermReply},
	{http.MethodGet, "/questions", rewriteTo("/question")},
	{http.MethodPost, "/questions/{id}/reply", rewriteQuestionAction("/reply")},
	{http.MethodPost, "/questions/{id}/reject", rewriteQuestionAction("/reject")},
	{http.MethodGet, "/events", rewriteTo("/event")},
	{http.MethodGet, "/agents/models", rewriteTo("/provider/capabilities")},
	{http.MethodGet, "/agents/session-modes", rewriteTo("/agent")},
	{http.MethodGet, "/agents/commands", rewriteTo("/command")},
	{http.MethodGet, "/agents/mcp", rewriteTo("/mcp")},
	{http.MethodGet, "/agents/lsp", rewriteTo("/lsp")},
}

func rewriteTo(target string) func(*http.Request) string {
	return func(_ *http.Request) string { return target }
}

func rewriteSessionID(prefix string) func(*http.Request) string {
	return func(r *http.Request) string {
		id := r.PathValue("id")
		return prefix + id
	}
}

func rewriteSessionIDWithSuffix(prefix, suffix string) func(*http.Request) string {
	return func(r *http.Request) string {
		id := r.PathValue("id")
		return prefix + id + suffix
	}
}

func rewritePermReply(r *http.Request) string {
	id := r.PathValue("id")
	return "/permission/" + id + "/reply"
}

func rewriteQuestionAction(suffix string) func(*http.Request) string {
	return func(r *http.Request) string {
		id := r.PathValue("id")
		return "/question/" + id + suffix
	}
}

func (s *Server) makeReverseProxy(target string) http.Handler {
	targetURL, err := url.Parse(target)
	if err != nil {
		log.Fatalf("invalid proxy target: %s", target)
	}
	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	proxy.ErrorLog = log.New(log.Writer(), "[proxy] ", log.LstdFlags)
	return proxy
}

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

	var rewriteFunc func(*http.Request) string
	for _, rt := range proxyRoutes {
		if r.Method != rt.method {
			continue
		}
		cleanPath := strings.TrimPrefix(r.URL.Path, "/api/v1")
		if matchRoute(cleanPath, rt.prefix) {
			rewriteFunc = rt.rewrite
			break
		}
	}

	if rewriteFunc == nil {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "no proxy route for "+r.URL.Path)
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.URL.Path = rewriteFunc(r)
		req.URL.RawPath = ""
		req.Host = targetURL.Host
	}

	proxy.ServeHTTP(w, r)
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
