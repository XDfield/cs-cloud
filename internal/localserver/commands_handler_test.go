package localserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleCommands(t *testing.T) {
	opencode := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/command" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		cmds := []Command{
			{Name: "init", Description: "Initialize", Source: "command"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(cmds)
	}))
	defer opencode.Close()

	// We cannot easily inject a mock AgentManager here because Server.manager
	// is created internally. Instead, test fetchOpenCodeCommands directly and
	// test handleCommands integration at a higher level later.
	cmds, err := fetchOpenCodeCommands(opencode.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cmds) != 1 {
		t.Fatalf("expected 1 command, got %d", len(cmds))
	}
	if cmds[0].Name != "init" {
		t.Fatalf("expected init, got %s", cmds[0].Name)
	}
}

func TestHandleCommandsOpencodeUnavailable(t *testing.T) {
	// Point to a non-existent endpoint
	cmds, err := fetchOpenCodeCommands("http://127.0.0.1:1")
	if err == nil {
		t.Fatal("expected error when opencode is unavailable")
	}
	if cmds != nil {
		t.Fatal("expected nil commands on error")
	}
}

func TestHandleCommandsOpencodeErrorStatus(t *testing.T) {
	opencode := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer opencode.Close()

	_, err := fetchOpenCodeCommands(opencode.URL)
	if err == nil {
		t.Fatal("expected error for non-200 status")
	}
}
