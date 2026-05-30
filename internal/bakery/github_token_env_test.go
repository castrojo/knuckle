package bakery

import (
	"testing"
)

// TestGithubTokenFromEnv verifies all three branches of githubTokenFromEnv:
//   - GITHUB_TOKEN set → returned verbatim (trimmed)
//   - GITHUB_TOKEN empty, GH_TOKEN set → GH_TOKEN returned (trimmed)
//   - Neither set → empty string returned
//
// These tests exercise the auth-credential path used by NewHTTPClient and
// NewHTTPClientWithURL; previously 0% covered.
func TestGithubTokenFromEnv_GithubToken(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "ghp_testtoken123")
	t.Setenv("GH_TOKEN", "")

	got := githubTokenFromEnv()
	if got != "ghp_testtoken123" {
		t.Errorf("githubTokenFromEnv() = %q, want %q", got, "ghp_testtoken123")
	}
}

func TestGithubTokenFromEnv_FallbackGHToken(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "ghs_fallback456")

	got := githubTokenFromEnv()
	if got != "ghs_fallback456" {
		t.Errorf("githubTokenFromEnv() = %q, want %q", got, "ghs_fallback456")
	}
}

func TestGithubTokenFromEnv_NeitherSet(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	got := githubTokenFromEnv()
	if got != "" {
		t.Errorf("githubTokenFromEnv() = %q, want empty string", got)
	}
}

func TestGithubTokenFromEnv_GithubTokenTakesPriority(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "primary_token")
	t.Setenv("GH_TOKEN", "fallback_token")

	got := githubTokenFromEnv()
	if got != "primary_token" {
		t.Errorf("githubTokenFromEnv() = %q, want %q (GITHUB_TOKEN should take priority)", got, "primary_token")
	}
}

func TestGithubTokenFromEnv_TrimsWhitespace(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "  ghp_padded  ")
	t.Setenv("GH_TOKEN", "")

	got := githubTokenFromEnv()
	if got != "ghp_padded" {
		t.Errorf("githubTokenFromEnv() = %q, want trimmed %q", got, "ghp_padded")
	}
}

func TestGithubTokenFromEnv_GHTokenTrimsWhitespace(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "\tghs_spaced\t")

	got := githubTokenFromEnv()
	if got != "ghs_spaced" {
		t.Errorf("githubTokenFromEnv() = %q, want trimmed %q", got, "ghs_spaced")
	}
}
