package bakery

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

const mockVersionTxt = `FLATCAR_VERSION=4593.2.0
FLATCAR_BUILD_ID="2026-04-14-0806"
GROUP=stable
`

const mockPackageList = `app-misc/some-tool-1.2.3::portage-stable
sys-kernel/coreos-kernel-6.12.81::coreos-overlay
sys-apps/systemd-257.9::portage-stable
net-misc/curl-8.5.0::portage-stable
`

func TestFetchChannelInfo(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/version.txt", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(mockVersionTxt))
	})
	mux.HandleFunc("/packages.txt", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(mockPackageList))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	info, err := fetchChannelInfoFromURLs(context.Background(), "stable", srv.URL+"/version.txt", srv.URL+"/packages.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if info.Channel != "stable" {
		t.Errorf("channel: got %q, want %q", info.Channel, "stable")
	}
	if info.Version != "4593.2.0" {
		t.Errorf("version: got %q, want %q", info.Version, "4593.2.0")
	}
	if info.BuildDate != "2026-04-14" {
		t.Errorf("build date: got %q, want %q", info.BuildDate, "2026-04-14")
	}
	if info.Kernel != "6.12.81" {
		t.Errorf("kernel: got %q, want %q", info.Kernel, "6.12.81")
	}
	if info.Systemd != "257.9" {
		t.Errorf("systemd: got %q, want %q", info.Systemd, "257.9")
	}
}

func TestFetchAllChannels(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/version.txt", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(mockVersionTxt))
	})
	mux.HandleFunc("/packages.txt", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(mockPackageList))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	results, err := fetchAllChannelsWithURLFn(context.Background(), func(channel string) (string, string) {
		return srv.URL + "/version.txt", srv.URL + "/packages.txt"
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	expected := []string{"stable", "beta", "alpha"}
	for i, ch := range expected {
		if results[i].Channel != ch {
			t.Errorf("results[%d].Channel: got %q, want %q", i, results[i].Channel, ch)
		}
		if results[i].Version != "4593.2.0" {
			t.Errorf("results[%d].Version: got %q, want %q", i, results[i].Version, "4593.2.0")
		}
	}
}

func TestFetchChannelInfoHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := fetchChannelInfoFromURLs(context.Background(), "stable", srv.URL+"/version.txt", srv.URL+"/packages.txt")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestParseVersionTxt(t *testing.T) {
	info := &ChannelInfo{}
	parseVersionTxt(mockVersionTxt, info)

	if info.Version != "4593.2.0" {
		t.Errorf("version: got %q, want %q", info.Version, "4593.2.0")
	}
	if info.BuildDate != "2026-04-14" {
		t.Errorf("build date: got %q, want %q", info.BuildDate, "2026-04-14")
	}
}

func TestParsePackageList(t *testing.T) {
	info := &ChannelInfo{}
	parsePackageList(mockPackageList, info)

	if info.Kernel != "6.12.81" {
		t.Errorf("kernel: got %q, want %q", info.Kernel, "6.12.81")
	}
	if info.Systemd != "257.9" {
		t.Errorf("systemd: got %q, want %q", info.Systemd, "257.9")
	}
}
