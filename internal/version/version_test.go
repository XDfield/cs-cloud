package version

import "testing"

func TestGet(t *testing.T) {
	Version = "v1.2.3"
	if got := Get(); got != "v1.2.3" {
		t.Errorf("Get() = %q, want %q", got, "v1.2.3")
	}
}

func TestFullString(t *testing.T) {
	Version = "v1.2.3"
	Commit = "abc1234"
	BuildTime = "2026-01-01"
	got := FullString()
	if got == "" {
		t.Error("FullString() should not be empty")
	}
}

func TestUserAgent(t *testing.T) {
	Version = "v1.2.3"
	if got := UserAgent(); got != "cs-cloud/v1.2.3" {
		t.Errorf("UserAgent() = %q, want %q", got, "cs-cloud/v1.2.3")
	}
}

func TestDefaults(t *testing.T) {
	if Version == "" {
		t.Error("Version should have a default value")
	}
}
