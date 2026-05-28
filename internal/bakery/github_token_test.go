package bakery

import (
	"testing"
)

// TestGithubTokenFromEnv covers the GH_TOKEN fallback branch (the GITHUB_TOKEN
// primary branch is already exercised by other tests that set GH_TOKEN only).
func TestGithubTokenFromEnv_GH_TOKEN_fallback(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "gh-fallback-token")

	got := githubTokenFromEnv()
	if got != "gh-fallback-token" {
		t.Errorf("githubTokenFromEnv() = %q, want %q", got, "gh-fallback-token")
	}
}

func TestGithubTokenFromEnv_GITHUB_TOKEN_preferred(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "primary-token")
	t.Setenv("GH_TOKEN", "should-not-be-used")

	got := githubTokenFromEnv()
	if got != "primary-token" {
		t.Errorf("githubTokenFromEnv() = %q, want GITHUB_TOKEN value", got)
	}
}

func TestGithubTokenFromEnv_both_empty(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	got := githubTokenFromEnv()
	if got != "" {
		t.Errorf("githubTokenFromEnv() = %q, want empty", got)
	}
}

func TestGithubTokenFromEnv_trims_whitespace(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "  token-with-spaces  ")
	t.Setenv("GH_TOKEN", "")

	got := githubTokenFromEnv()
	if got != "token-with-spaces" {
		t.Errorf("githubTokenFromEnv() = %q, want trimmed token", got)
	}
}

func TestGithubTokenFromEnv_GH_TOKEN_whitespace_only_fallback(t *testing.T) {
	// GITHUB_TOKEN is whitespace-only → treated as empty → falls back to GH_TOKEN
	t.Setenv("GITHUB_TOKEN", "   ")
	t.Setenv("GH_TOKEN", "real-token")

	got := githubTokenFromEnv()
	if got != "real-token" {
		t.Errorf("githubTokenFromEnv() with whitespace GITHUB_TOKEN = %q, want GH_TOKEN value", got)
	}
}

func TestNewHTTPClient_UsesGH_TOKEN(t *testing.T) {
	// Verify the client constructed by NewHTTPClient picks up the token.
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "integration-token")

	c := NewHTTPClient()
	if c.AuthToken != "integration-token" {
		t.Errorf("NewHTTPClient().AuthToken = %q, want GH_TOKEN value", c.AuthToken)
	}
}
