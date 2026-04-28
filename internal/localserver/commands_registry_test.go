package localserver

import (
	"slices"
	"testing"
)

func TestBuiltinCommandsNoDuplicates(t *testing.T) {
	seen := make(map[string]struct{})
	for _, c := range BuiltinCommands {
		if _, ok := seen[c.Name]; ok {
			t.Fatalf("duplicate command name: %s", c.Name)
		}
		seen[c.Name] = struct{}{}
		for _, a := range c.Aliases {
			if _, ok := seen[a]; ok {
				t.Fatalf("duplicate alias: %s (command: %s)", a, c.Name)
			}
			seen[a] = struct{}{}
		}
	}
}

func TestBuiltinCommandsHaveValidScope(t *testing.T) {
	validScopes := []string{ScopeShared, ScopeTuiOnly, ScopeCloudOnly}
	for _, c := range BuiltinCommands {
		if !slices.Contains(validScopes, c.Scope) {
			t.Fatalf("invalid scope %q for command %s", c.Scope, c.Name)
		}
	}
}

func TestBuildManifest(t *testing.T) {
	opencode := []Command{
		{Name: "init", Description: "Initialize", Source: "command"},
		{Name: "review", Description: "Review", Source: "command"},
	}

	// shared + prompt + cloud-only (default)
	manifest, err := BuildManifest([]string{ScopeShared, ScopePrompt, ScopeCloudOnly}, opencode)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// should contain all shared, cloud-only, and prompt commands
	sharedCount := 0
	cloudCount := 0
	promptCount := 0
	for _, c := range manifest {
		switch c.Scope {
		case ScopeShared:
			sharedCount++
		case ScopeCloudOnly:
			cloudCount++
		case ScopePrompt:
			promptCount++
		}
	}

	if sharedCount == 0 {
		t.Error("expected shared commands in manifest")
	}
	if cloudCount == 0 {
		t.Error("expected cloud-only commands in manifest")
	}
	if promptCount != len(opencode) {
		t.Errorf("expected %d prompt commands, got %d", len(opencode), promptCount)
	}

	// tui-only should NOT be included
	for _, c := range manifest {
		if c.Scope == ScopeTuiOnly {
			t.Errorf("tui-only command %s should not be in manifest", c.Name)
		}
	}
}

func TestBuildManifestDuplicateDetection(t *testing.T) {
	opencode := []Command{
		{Name: "models", Description: "conflict with builtin"},
	}
	_, err := BuildManifest([]string{ScopeShared, ScopePrompt}, opencode)
	if err == nil {
		t.Fatal("expected error for duplicate command name")
	}
}

func TestBuildManifestEmptyOpencode(t *testing.T) {
	manifest, err := BuildManifest([]string{ScopeShared}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, c := range manifest {
		if c.Scope != ScopeShared {
			t.Errorf("expected only shared commands, got %s", c.Scope)
		}
	}
}

func TestParseIncludeScopes(t *testing.T) {
	cases := []struct {
		input    string
		expected []string
	}{
		{"", []string{ScopeShared, ScopePrompt, ScopeCloudOnly}},
		{"shared,prompt", []string{"shared", "prompt"}},
		{"shared, prompt , cloud-only", []string{"shared", "prompt", "cloud-only"}},
	}

	for _, tc := range cases {
		got := parseIncludeScopes(tc.input)
		if len(got) != len(tc.expected) {
			t.Fatalf("parseIncludeScopes(%q): expected %v, got %v", tc.input, tc.expected, got)
		}
		for i := range got {
			if got[i] != tc.expected[i] {
				t.Fatalf("parseIncludeScopes(%q): expected %v, got %v", tc.input, tc.expected, got)
			}
		}
	}
}
