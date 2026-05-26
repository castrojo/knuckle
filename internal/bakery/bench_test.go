package bakery

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/projectbluefin/knuckle/internal/model"
)

var (
	benchmarkTagNameSink        string
	benchmarkTagVersionSink     string
	benchmarkLinkNextSink       string
	benchmarkLinkNextOKSink     bool
	benchmarkDescriptionSink    string
	benchmarkEntriesSink        []model.SysextEntry
	benchmarkChannelInfoSink    ChannelInfo
	benchmarkVerifySHA512Sink   bool
	benchmarkVersionTxtBody     = buildBenchmarkVersionTxt()
	benchmarkSBOMJSONBody       = buildBenchmarkSBOMJSON()
	benchmarkPackageListBody    = buildBenchmarkPackageList()
	benchmarkDigestContent      = buildBenchmarkDigestContent()
	benchmarkDigestFilename     = "flatcar_production_image_sbom.json"
	benchmarkDigestBody         = buildBenchmarkDigestBody(benchmarkDigestContent, benchmarkDigestFilename)
	benchmarkCatalogFirstPage   = buildBenchmarkCatalogPage(0, 24, false)
	benchmarkCatalogSecondPage  = buildBenchmarkCatalogPage(24, 24, true)
	benchmarkCatalogLinkHeader  = `<https://example.com/releases?page=2>; rel="next", <https://example.com/releases?page=2>; rel="last"`
	benchmarkParseTagNameInputs = []string{
		"docker-v28.0.4",
		"containerd-v2.1.5",
		"intel_gpu-2025.04.01",
		"observability-stack-v1.12.3",
	}
	benchmarkLinkHeader  = `<https://api.github.com/repos/flatcar/sysext-bakery/releases?per_page=100&page=1>; rel="prev", <https://api.github.com/repos/flatcar/sysext-bakery/releases?per_page=100&page=3>; rel="next", <https://api.github.com/repos/flatcar/sysext-bakery/releases?per_page=100&page=6>; rel="last"`
	benchmarkDescription = "  Docker sysext for Flatcar clusters with metrics, logging, and runtime integrations enabled by default for multi-node rollouts.\nIncludes detailed release notes that should be ignored by the truncation helper.  "
)

type benchmarkRoundTripper func(*http.Request) (*http.Response, error)

func (fn benchmarkRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func BenchmarkParseTagName(b *testing.B) {
	inputs := benchmarkParseTagNameInputs
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		name, version := ParseTagName(inputs[i%len(inputs)])
		benchmarkTagNameSink = name
		benchmarkTagVersionSink = version
	}
}

func BenchmarkParseLinkNext(b *testing.B) {
	header := benchmarkLinkHeader
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		benchmarkLinkNextSink, benchmarkLinkNextOKSink = parseLinkNext(header)
	}
}

func BenchmarkTruncateDescription(b *testing.B) {
	description := benchmarkDescription
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		benchmarkDescriptionSink = truncateDescription(description, 80)
	}
}

func BenchmarkFetchCatalogArch(b *testing.B) {
	client := &HTTPClient{
		CatalogURL: "https://example.com/releases?page=1",
		HTTP: &http.Client{Transport: benchmarkRoundTripper(func(req *http.Request) (*http.Response, error) {
			body := benchmarkCatalogFirstPage
			header := make(http.Header)
			if req.URL.Query().Get("page") == "2" {
				body = benchmarkCatalogSecondPage
			} else {
				header.Set("Link", benchmarkCatalogLinkHeader)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     header,
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		})},
	}

	ctx := context.Background()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		entries, err := client.FetchCatalogArch(ctx, "amd64")
		if err != nil {
			b.Fatalf("FetchCatalogArch failed: %v", err)
		}
		benchmarkEntriesSink = entries
	}
}

func BenchmarkParseVersionTxt(b *testing.B) {
	body := benchmarkVersionTxtBody
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		info := &ChannelInfo{}
		parseVersionTxt(body, info)
		benchmarkChannelInfoSink = *info
	}
}

func BenchmarkParseSBOMJSON(b *testing.B) {
	body := benchmarkSBOMJSONBody
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		info := &ChannelInfo{}
		parseSBOMJSON(body, info)
		benchmarkChannelInfoSink = *info
	}
}

func BenchmarkParsePackageList(b *testing.B) {
	body := benchmarkPackageListBody
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		info := &ChannelInfo{}
		parsePackageList(body, info)
		benchmarkChannelInfoSink = *info
	}
}

func BenchmarkVerifySHA512(b *testing.B) {
	content := benchmarkDigestContent
	digest := benchmarkDigestBody
	filename := benchmarkDigestFilename
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		benchmarkVerifySHA512Sink = verifySHA512(content, digest, filename)
	}
}

func buildBenchmarkVersionTxt() string {
	var body strings.Builder
	body.WriteString("FLATCAR_VERSION=4593.2.1\n")
	body.WriteString("FLATCAR_BUILD_ID=\"2026-04-14-0806\"\n")
	body.WriteString("GROUP=stable\n")
	for i := 0; i < 48; i++ {
		fmt.Fprintf(&body, "EXTRA_KEY_%02d=value-%02d\n", i, i)
	}
	return body.String()
}

func buildBenchmarkSBOMJSON() string {
	var body strings.Builder
	body.WriteString(`{"spdxVersion":"SPDX-2.3","packages":[`)
	for i := 0; i < 96; i++ {
		if i > 0 {
			body.WriteByte(',')
		}
		name := fmt.Sprintf("app-benchmark/pkg-%02d", i)
		version := fmt.Sprintf("%d.%d.%d", 1+i/24, i%12, i%7)
		switch i {
		case 7:
			name, version = "sys-kernel/coreos-kernel", "6.12.87"
		case 18:
			name, version = "sys-apps/systemd", "257.9"
		case 51:
			name, version = "sys-apps/ignition", "2.24.0-r1"
		case 72:
			name, version = "dev-db/etcd", "3.5.18"
		}
		fmt.Fprintf(&body, `{"name":"%s","versionInfo":"%s"}`, name, version)
	}
	body.WriteString("]}")
	return body.String()
}

func buildBenchmarkPackageList() string {
	var body strings.Builder
	for i := 0; i < 160; i++ {
		switch i {
		case 40:
			body.WriteString("sys-kernel/coreos-kernel-6.12.81::coreos-overlay\n")
		case 82:
			body.WriteString("sys-apps/systemd-257.9::portage-stable\n")
		case 109:
			body.WriteString("sys-apps/ignition-2.24.0-r1::coreos-overlay\n")
		case 147:
			body.WriteString("dev-db/etcd-3.5.18::portage-stable\n")
		default:
			fmt.Fprintf(&body, "app-benchmark/pkg-%03d-%d.%d.%d::portage-stable\n", i, 1+i/32, i%16, i%9)
		}
	}
	return body.String()
}

func buildBenchmarkDigestContent() string {
	return strings.Repeat("benchmark-sbom-payload-with-package-metadata\n", 64)
}

func buildBenchmarkDigestBody(content, expectedFilename string) string {
	hash := sha512.Sum512([]byte(content))
	computed := hex.EncodeToString(hash[:])

	var body strings.Builder
	body.WriteString("# MD5 HASH\n")
	for i := 0; i < 16; i++ {
		fmt.Fprintf(&body, "deadbeef%024x  other-file-%02d.txt\n", i, i)
	}
	body.WriteString("# SHA256 HASH\n")
	for i := 0; i < 16; i++ {
		fmt.Fprintf(&body, "%064x  flatcar-other-%02d.raw\n", i+1, i)
	}
	body.WriteString("# SHA512 HASH\n")
	for i := 0; i < 48; i++ {
		fmt.Fprintf(&body, "%0128x  flatcar-noise-%02d.raw\n", i+1, i)
	}
	fmt.Fprintf(&body, "%s  %s\n", computed, expectedFilename)
	body.WriteString("# SIGNATURE\n")
	body.WriteString("-----BEGIN PGP SIGNATURE-----\n")
	body.WriteString("benchmark\n")
	body.WriteString("-----END PGP SIGNATURE-----\n")
	return body.String()
}

func buildBenchmarkCatalogPage(start, count int, duplicateOlder bool) string {
	var body strings.Builder
	body.WriteByte('[')
	for i := 0; i < count; i++ {
		if i > 0 {
			body.WriteByte(',')
		}
		idx := start + i
		name := fmt.Sprintf("benchpkg%02d", idx)
		major := 1 + idx/12
		minor := idx % 12
		patch := (idx % 5) + 1
		if duplicateOlder && i < 4 {
			name = fmt.Sprintf("benchpkg%02d", i)
			major = 1
			minor = i
			patch = 0
		}
		description := fmt.Sprintf("Benchmark package %02d release for Flatcar nodes with runtime helpers and observability integrations.", idx)
		fmt.Fprintf(&body, `{"tag_name":"%s-v%d.%d.%d","body":"%s","assets":[{"name":"%s-%d.%d.%d-x86-64.raw","browser_download_url":"https://example.com/download/%s-v%d.%d.%d/%s-%d.%d.%d-x86-64.raw"},{"name":"%s-%d.%d.%d-arm64.raw","browser_download_url":"https://example.com/download/%s-v%d.%d.%d/%s-%d.%d.%d-arm64.raw"}]}`,
			name, major, minor, patch, description,
			name, major, minor, patch, name, major, minor, patch, name, major, minor, patch,
			name, major, minor, patch, name, major, minor, patch, name, major, minor, patch,
		)
	}
	body.WriteByte(']')
	return body.String()
}
