package bakery

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestSHA256SoftFail_MissingSHA256SUMS verifies that entries are returned WITHOUT
// a hash when SHA256SUMS fetch fails (soft fail). This documents that SHA256 is
// NOT mandatory — a MITM or CDN failure silently degrades to unverified downloads.
func TestSHA256SoftFail_MissingSHA256SUMS(t *testing.T) {
	payload := `[{"tag_name":"docker-v24.0.7","body":"Docker","assets":[
		{"name":"docker-24.0.7-x86-64.raw","browser_download_url":"https://BASEURL/docker.raw"},
		{"name":"SHA256SUMS","browser_download_url":"https://BASEURL/SHA256SUMS"}
	]}]`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/SHA256SUMS" {
			// Simulate CDN 503 or MITM dropping the SHA256SUMS file
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
			return
		}
		catalog := strings.ReplaceAll(payload, "https://BASEURL", "http://"+r.Host)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(catalog))
	}))
	defer srv.Close()

	client := NewHTTPClientWithURL(srv.URL)
	entries, err := client.FetchCatalog(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	// FINDING: Entry returned with empty Sha256 — NO error to caller.
	// The sysext will be placed into Ignition WITHOUT verification hash.
	if entries[0].Sha256 != "" {
		t.Errorf("expected empty Sha256 on fetch failure, got %q", entries[0].Sha256)
	}
}

// TestSHA256SoftFail_MalformedDigestFile verifies behavior when SHA256SUMS content
// doesn't contain the expected filename — hash is empty (no error).
func TestSHA256SoftFail_MalformedDigestFile(t *testing.T) {
	payload := `[{"tag_name":"docker-v24.0.7","body":"Docker","assets":[
		{"name":"docker-24.0.7-x86-64.raw","browser_download_url":"https://BASEURL/docker-24.0.7-x86-64.raw"},
		{"name":"SHA256SUMS","browser_download_url":"https://BASEURL/SHA256SUMS"}
	]}]`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/SHA256SUMS":
			// Malformed: garbage content, no valid hash lines
			_, _ = w.Write([]byte("this is not a valid sha256sums file\n\n\n"))
		default:
			catalog := strings.ReplaceAll(payload, "https://BASEURL", "http://"+r.Host)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(catalog))
		}
	}))
	defer srv.Close()

	client := NewHTTPClientWithURL(srv.URL)
	entries, err := client.FetchCatalog(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	// FINDING: Malformed digest file silently produces empty hash.
	if entries[0].Sha256 != "" {
		t.Errorf("expected empty Sha256 for malformed digest file, got %q", entries[0].Sha256)
	}
}

// TestSHA256SoftFail_WrongFilenameInDigest verifies that a SHA256SUMS file
// that doesn't list the target filename results in empty hash (no match).
func TestSHA256SoftFail_WrongFilenameInDigest(t *testing.T) {
	payload := `[{"tag_name":"docker-v24.0.7","body":"Docker","assets":[
		{"name":"docker-24.0.7-x86-64.raw","browser_download_url":"https://BASEURL/docker-24.0.7-x86-64.raw"},
		{"name":"SHA256SUMS","browser_download_url":"https://BASEURL/SHA256SUMS"}
	]}]`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/SHA256SUMS":
			// Valid format but filename doesn't match the target asset
			_, _ = w.Write([]byte("abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890  totally-different-file.raw\n"))
		default:
			catalog := strings.ReplaceAll(payload, "https://BASEURL", "http://"+r.Host)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(catalog))
		}
	}))
	defer srv.Close()

	client := NewHTTPClientWithURL(srv.URL)
	entries, err := client.FetchCatalog(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Sha256 != "" {
		t.Errorf("expected empty Sha256 when filename not in SUMS, got %q", entries[0].Sha256)
	}
}

// TestSHA256Injection_CraftedCatalogURL verifies that download URLs in the catalog
// are passed through as-is from the GitHub Releases API response. An attacker who
// controls the API response (unlikely with HTTPS, but e.g. a compromised token or
// GitHub Actions artifact poisoning) can inject arbitrary download URLs.
func TestSHA256Injection_CraftedCatalogURL(t *testing.T) {
	// Simulate an attacker serving malicious catalog with arbitrary download URLs
	evilPayload := `[{"tag_name":"docker-v24.0.7","body":"Docker","assets":[
		{"name":"docker-24.0.7-x86-64.raw","browser_download_url":"http://evil.attacker.example.com/backdoor.raw"},
		{"name":"SHA256SUMS","browser_download_url":"http://evil.attacker.example.com/SHA256SUMS"}
	]}]`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/SHA256SUMS":
			// Attacker controls the SHA256SUMS too — they provide hash of their backdoor
			_, _ = w.Write([]byte("deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef  backdoor.raw\n"))
		default:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(evilPayload))
		}
	}))
	defer srv.Close()

	client := NewHTTPClientWithURL(srv.URL)
	entries, err := client.FetchCatalog(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	// The catalog layer stores the URL as-is; HTTPS enforcement happens in
	// ignition.GenerateButane() when the URL is embedded into the Ignition config.
	if entries[0].URL != "http://evil.attacker.example.com/backdoor.raw" {
		t.Errorf("expected evil URL to pass through catalog fetch, got %q", entries[0].URL)
	}
	_ = entries[0].Sha256 // may be empty (unreachable evil server)
}

// TestSHA256_ValidHashExtracted verifies the happy path: hash is correctly
// extracted when SHA256SUMS is well-formed and contains the target filename.
func TestSHA256_ValidHashExtracted(t *testing.T) {
	const expectedHash = "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"
	payload := `[{"tag_name":"docker-v24.0.7","body":"Docker","assets":[
		{"name":"docker-24.0.7-x86-64.raw","browser_download_url":"https://BASEURL/docker-24.0.7-x86-64.raw"},
		{"name":"SHA256SUMS","browser_download_url":"https://BASEURL/SHA256SUMS"}
	]}]`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/SHA256SUMS":
			_, _ = w.Write([]byte(expectedHash + "  docker-24.0.7-x86-64.raw\notherhash  other-file.raw\n"))
		default:
			catalog := strings.ReplaceAll(payload, "https://BASEURL", "http://"+r.Host)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(catalog))
		}
	}))
	defer srv.Close()

	client := NewHTTPClientWithURL(srv.URL)
	entries, err := client.FetchCatalog(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Sha256 != expectedHash {
		t.Errorf("expected hash %q, got %q", expectedHash, entries[0].Sha256)
	}
}

// TestSHA256_PrefixedFilename verifies that SHA256SUMS with "./filename" prefix
// is handled correctly (strip prefix before comparison).
func TestSHA256_PrefixedFilename(t *testing.T) {
	const expectedHash = "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"
	payload := `[{"tag_name":"docker-v24.0.7","body":"Docker","assets":[
		{"name":"docker-24.0.7-x86-64.raw","browser_download_url":"https://BASEURL/docker-24.0.7-x86-64.raw"},
		{"name":"SHA256SUMS","browser_download_url":"https://BASEURL/SHA256SUMS"}
	]}]`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/SHA256SUMS":
			// Some tools produce "./filename" in SHA256SUMS
			_, _ = w.Write([]byte(expectedHash + "  ./docker-24.0.7-x86-64.raw\n"))
		default:
			catalog := strings.ReplaceAll(payload, "https://BASEURL", "http://"+r.Host)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(catalog))
		}
	}))
	defer srv.Close()

	client := NewHTTPClientWithURL(srv.URL)
	entries, err := client.FetchCatalog(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	// The code strips the path prefix before comparison — this should work
	if entries[0].Sha256 != expectedHash {
		t.Errorf("expected hash %q for ./prefixed filename, got %q", expectedHash, entries[0].Sha256)
	}
}

// TestVerifySHA512_EmptyDigestFile documents that an empty .DIGESTS file
// results in verification failure (returns false).
func TestVerifySHA512_EmptyDigestFile(t *testing.T) {
	if verifySHA512("any content", "", "test.txt") {
		t.Error("verifySHA512 should return false for empty digest body")
	}
}

// TestVerifySHA512_MissingHashSection documents that a .DIGESTS file without
// the "# SHA512 HASH" header results in verification failure.
func TestVerifySHA512_MissingHashSection(t *testing.T) {
	// No "# SHA512 HASH" marker — all lines are ignored
	digest := "309ecc489c12d6eb4cc40f50c902f2b4d0ed77ee511a7c7a9bcd3ca86d4cd86f989dd35bc5ff499670da34255b45b0cfd830e81f605dcf7dc5542e93ae9cd76f  test.txt\n"
	if verifySHA512("hello world", digest, "test.txt") {
		t.Error("should fail when # SHA512 HASH header is missing")
	}
}

// TestVerifySHA512_MissingDigestFile_ViaChannel documents that a missing .DIGESTS
// file silently skips verification — no error propagated.
func TestVerifySHA512_MissingDigestFile_ViaChannel(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/version.txt", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("FLATCAR_VERSION=4593.2.0\n"))
	})
	mux.HandleFunc("/flatcar_production_image_sbom.json", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"packages":[{"name":"sys-kernel/coreos-kernel","versionInfo":"6.12.81"}]}`))
	})
	mux.HandleFunc("/flatcar_production_image_sbom.json.DIGESTS", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	})
	mux.HandleFunc("/flatcar_production_image_packages.txt", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", 404)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	info, err := fetchChannelInfoFromURLs(context.Background(), "stable",
		srv.URL+"/version.txt",
		srv.URL+"/flatcar_production_image_packages.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// FINDING: Missing .DIGESTS is silently skipped. DigestVerified=false but no error.
	if info.DigestVerified {
		t.Error("DigestVerified should be false when .DIGESTS is 404")
	}
	if info.SignedDigest {
		t.Error("SignedDigest should be false when .DIGESTS is 404")
	}
	if !info.SBOMVerified {
		t.Error("SBOMVerified should be true even when .DIGESTS is missing")
	}
}

// TestGPGSignature_MissingAscFile documents that a missing .DIGESTS.asc file
// silently skips GPG verification — SignedDigest stays false.
func TestGPGSignature_MissingAscFile(t *testing.T) {
	content := `{"packages":[{"name":"sys-kernel/coreos-kernel","versionInfo":"6.12.81"}]}`
	mux := http.NewServeMux()
	mux.HandleFunc("/version.txt", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("FLATCAR_VERSION=4593.2.0\n"))
	})
	mux.HandleFunc("/flatcar_production_image_sbom.json", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(content))
	})
	mux.HandleFunc("/flatcar_production_image_sbom.json.DIGESTS", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("# SHA512 HASH\ndeadbeef  flatcar_production_image_sbom.json\n"))
	})
	mux.HandleFunc("/flatcar_production_image_sbom.json.DIGESTS.asc", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	})
	mux.HandleFunc("/flatcar_production_image_packages.txt", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", 404)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	info, err := fetchChannelInfoFromURLs(context.Background(), "stable",
		srv.URL+"/version.txt",
		srv.URL+"/flatcar_production_image_packages.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// FINDING: GPG signature is entirely optional — missing .asc is silent.
	if info.SignedDigest {
		t.Error("SignedDigest should be false when .DIGESTS.asc returns 404")
	}
}

// TestSHA256HashValidation verifies that malformed SHA256 hashes are rejected.
// fetchSHA256ForAsset returns ("", err) for non-hex-64 strings; the entry
// is still included in the catalog but with an empty Sha256 (soft fail).
func TestSHA256HashValidation(t *testing.T) {
	payload := `[{"tag_name":"docker-v24.0.7","body":"Docker","assets":[
		{"name":"docker-24.0.7-x86-64.raw","browser_download_url":"https://BASEURL/docker-24.0.7-x86-64.raw"},
		{"name":"SHA256SUMS","browser_download_url":"https://BASEURL/SHA256SUMS"}
	]}]`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/SHA256SUMS":
			// Invalid hash: too short, not hex
			_, _ = w.Write([]byte("not-a-valid-hash!!!  docker-24.0.7-x86-64.raw\n"))
		default:
			catalog := strings.ReplaceAll(payload, "https://BASEURL", "http://"+r.Host)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(catalog))
		}
	}))
	defer srv.Close()

	client := NewHTTPClientWithURL(srv.URL)
	entries, err := client.FetchCatalog(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	// Invalid hash is rejected; entry is still present but Sha256 is empty (soft fail).
	if entries[0].Sha256 != "" {
		t.Errorf("expected empty Sha256 for invalid hash, got %q", entries[0].Sha256)
	}
}
