package agent

import (
	"fmt"
	"strings"
)

// SlashCommand represents a slash command that can be exposed to cloud UI or TUI.
type SlashCommand struct {
	Name        string   `json:"name"`
	Aliases     []string `json:"aliases,omitempty"`
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
	Scope       string   `json:"scope,omitempty"`
	Category    string   `json:"category,omitempty"`
	Keybind     string   `json:"keybind,omitempty"`
	Source      string   `json:"source,omitempty"`
	Agent       string   `json:"agent,omitempty"`
	Model       string   `json:"model,omitempty"`
	Subtask     bool     `json:"subtask,omitempty"`
	Hints       []string `json:"hints,omitempty"`
}

const (
	ScopeShared    = "shared"
	ScopeTuiOnly   = "tui-only"
	ScopeCloudOnly = "cloud-only"
	ScopePrompt    = "prompt"
)

// BuiltinCommands is the source-of-truth list for TUI/Web UI commands.
// These are maintained in cs-cloud and merged with opencode prompt commands
// at runtime via BuildManifest.
var BuiltinCommands = []SlashCommand{
	// shared — available in both cloud UI and TUI
	{Name: "new", Aliases: []string{"clear"}, Title: "New session", Description: "Start a new session", Scope: ScopeShared, Category: "session", Keybind: "mod+shift+s"},
	{Name: "sessions", Aliases: []string{"resume", "continue"}, Title: "Switch session", Description: "Switch to another session", Scope: ScopeShared, Category: "session"},
	{Name: "workspaces", Title: "Manage workspaces", Description: "Manage or switch workspaces", Scope: ScopeShared, Category: "workspace"},
	{Name: "models", Title: "Switch model", Description: "Choose model for this session", Scope: ScopeShared, Category: "model", Keybind: "mod+'"},
	{Name: "agents", Title: "Switch agent", Description: "Cycle through available agents", Scope: ScopeShared, Category: "agent", Keybind: "mod+."},
	{Name: "mcps", Title: "Toggle MCPs", Description: "Enable or disable MCP servers", Scope: ScopeShared, Category: "mcp", Keybind: "mod+;"},
	{Name: "variants", Title: "Switch variant", Description: "Switch model variant", Scope: ScopeShared, Category: "model", Keybind: "shift+mod+d"},
	{Name: "connect", Title: "Connect provider", Description: "Connect to an AI provider", Scope: ScopeShared, Category: "provider"},
	{Name: "status", Title: "View status", Description: "View agent and provider status", Scope: ScopeShared, Category: "provider"},
	{Name: "credit", Title: "View credit", Description: "View usage and credit balance", Scope: ScopeShared, Category: "provider"},
	{Name: "themes", Title: "Switch theme", Description: "Change the UI theme", Scope: ScopeShared, Category: "ui"},
	{Name: "help", Title: "Help", Description: "Show keyboard shortcuts and commands", Scope: ScopeShared, Category: "ui"},
	{Name: "favorites", Aliases: []string{"fav"}, Title: "Manage favorites", Description: "Manage favorite skills", Scope: ScopeShared, Category: "skill"},
	{Name: "skills", Title: "Skills picker", Description: "Browse and select skills", Scope: ScopeShared, Category: "skill"},
	{Name: "share", Title: "Share session", Description: "Generate a shareable link for this session", Scope: ScopeShared, Category: "session", Keybind: "session_share"},
	{Name: "unshare", Title: "Unshare session", Description: "Revoke the share link for this session", Scope: ScopeShared, Category: "session", Keybind: "session_unshare"},
	{Name: "rename", Title: "Rename session", Description: "Rename the current session", Scope: ScopeShared, Category: "session", Keybind: "session_rename"},
	{Name: "timeline", Title: "Jump to message", Description: "Jump to a specific message in the timeline", Scope: ScopeShared, Category: "session", Keybind: "session_timeline"},
	{Name: "fork", Title: "Fork session", Description: "Fork the session from a message", Scope: ScopeShared, Category: "session", Keybind: "session_fork"},
	{Name: "compact", Aliases: []string{"summarize"}, Title: "Compact session", Description: "Summarize and compact the session history", Scope: ScopeShared, Category: "session", Keybind: "session_compact"},
	{Name: "undo", Title: "Undo", Description: "Undo the last message", Scope: ScopeShared, Category: "session"},
	{Name: "redo", Title: "Redo", Description: "Redo the previously undone message", Scope: ScopeShared, Category: "session"},
	{Name: "copy", Title: "Copy transcript", Description: "Copy session transcript to clipboard", Scope: ScopeShared, Category: "session"},
	{Name: "export", Title: "Export transcript", Description: "Export session transcript as markdown", Scope: ScopeShared, Category: "session"},
	{Name: "timestamps", Aliases: []string{"toggle-timestamps"}, Title: "Toggle timestamps", Description: "Show or hide message timestamps", Scope: ScopeShared, Category: "ui"},
	{Name: "thinking", Aliases: []string{"toggle-thinking"}, Title: "Toggle thinking", Description: "Show or hide thinking process", Scope: ScopeShared, Category: "ui"},

	// tui-only — not available in cloud UI
	{Name: "exit", Aliases: []string{"quit", "q"}, Title: "Exit", Description: "Exit the TUI application", Scope: ScopeTuiOnly, Category: "ui"},
	{Name: "editor", Title: "Open editor", Description: "Open the default external editor", Scope: ScopeTuiOnly, Category: "ui"},

	// cloud-only — not available in TUI
	{Name: "open", Title: "Open file", Description: "Open a file dialog", Scope: ScopeCloudOnly, Category: "file", Keybind: "mod+p"},
	{Name: "terminal", Title: "Toggle terminal", Description: "Show or hide the integrated terminal panel", Scope: ScopeCloudOnly, Category: "terminal", Keybind: "ctrl+`"},
}

// BuildManifest merges builtin UI commands with opencode prompt commands,
// then filters by the requested scopes.
func BuildManifest(includeScopes []string, opencodeCmds []SlashCommand) ([]SlashCommand, error) {
	scopeSet := make(map[string]struct{})
	for _, s := range includeScopes {
		scopeSet[s] = struct{}{}
	}

	result := make([]SlashCommand, 0, len(BuiltinCommands)+len(opencodeCmds))

	for _, c := range BuiltinCommands {
		if _, ok := scopeSet[c.Scope]; ok {
			result = append(result, c)
		}
	}

	for _, c := range opencodeCmds {
		// opencode commands are always prompt scope
		if _, ok := scopeSet[ScopePrompt]; !ok {
			continue
		}
		// ensure prompt commands carry the prompt scope
		c.Scope = ScopePrompt
		result = append(result, c)
	}

	// validate no duplicate names or aliases
	seen := make(map[string]struct{})
	for _, c := range result {
		if _, ok := seen[c.Name]; ok {
			return nil, fmt.Errorf("duplicate command name: %s", c.Name)
		}
		seen[c.Name] = struct{}{}
		for _, a := range c.Aliases {
			if _, ok := seen[a]; ok {
				return nil, fmt.Errorf("duplicate command alias: %s (command: %s)", a, c.Name)
			}
			seen[a] = struct{}{}
		}
	}

	return result, nil
}

// ParseIncludeScopes parses the ?include= query parameter.
// Defaults to [shared, prompt, cloud-only] when empty.
func ParseIncludeScopes(q string) []string {
	if q == "" {
		return []string{ScopeShared, ScopePrompt, ScopeCloudOnly}
	}
	parts := strings.Split(q, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}
