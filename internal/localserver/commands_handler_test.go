package localserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"cs-cloud/internal/agent"
	agentcs "cs-cloud/internal/agent/cs"
	agentcsc "cs-cloud/internal/agent/csc"
)

func TestDriverFetchCommands(t *testing.T) {
	opencode := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/command" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		cmds := []agent.SlashCommand{
			{Name: "init", Description: "Initialize", Source: "command"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(cmds)
	}))
	defer opencode.Close()

	d := agentcs.NewDriver(agent.Command{})
	cmds, err := d.FetchCommands(opencode.URL)
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

func TestDriverFetchCommandsUnavailable(t *testing.T) {
	d := agentcs.NewDriver(agent.Command{})
	cmds, err := d.FetchCommands("http://127.0.0.1:1")
	if err == nil {
		t.Fatal("expected error when endpoint is unavailable")
	}
	if cmds != nil {
		t.Fatal("expected nil commands on error")
	}
}

func TestDriverFetchCommandsErrorStatus(t *testing.T) {
	opencode := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer opencode.Close()

	d := agentcs.NewDriver(agent.Command{})
	_, err := d.FetchCommands(opencode.URL)
	if err == nil {
		t.Fatal("expected error for non-200 status")
	}
}

func TestCscDriverFetchCommands(t *testing.T) {
	opencode := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/command" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		cmds := []agent.SlashCommand{
			{Name: "init", Description: "Initialize", Source: "command"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(cmds)
	}))
	defer opencode.Close()

	d := agentcsc.NewDriver(agent.Command{})
	cmds, err := d.FetchCommands(opencode.URL)
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
