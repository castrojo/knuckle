package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/projectbluefin/knuckle/internal/model"
)

const (
	defaultDocsURL = "https://api.github.com/repos/flatcar/flatcar-website/contents/content/docs/latest/setup/customization/using-nvidia.md"
	modelGo        = "internal/model/model.go"
)

// docsURL is the GitHub API URL to fetch; overridable in tests.
var docsURL = defaultDocsURL

// ghContentResponse is the minimal GitHub API response for file contents.
type ghContentResponse struct {
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
}

func main() { os.Exit(run(os.Stdout, os.Stderr)) }

// fprintln writes to w discarding the error — safe for CLI stdout/stderr.
func fprintln(w io.Writer, a ...any) { _, _ = fmt.Fprintln(w, a...) }

// fprintf writes to w discarding the error — safe for CLI stdout/stderr.
func fprintf(w io.Writer, format string, a ...any) { _, _ = fmt.Fprintf(w, format, a...) }

// run contains the main logic, extracted for testability.
// Returns 0 on success, 1 when model is missing entries, 2 on fetch failure.
func run(stdout, stderr io.Writer) int {
	fprintln(stdout, "nvidia_check — verifying model.go NvidiaDriverOptions against Flatcar docs")
	fprintln(stdout, "──────────────────────────────────────────────────────────────────────────")
	fprintln(stdout)

	// ── Fetch Flatcar docs ────────────────────────────────────────────────
	fprintln(stdout, "Fetching Flatcar NVIDIA docs...")
	docContent, err := fetchNvidiaDocs()
	if err != nil {
		fprintf(stderr, "ERROR: Could not fetch Flatcar NVIDIA docs from GitHub.\n")
		fprintf(stderr, "  Check network or try: curl -sf '%s'\n", docsURL)
		return 2
	}

	// Extract all nvidia-drivers-* patterns from the docs.
	docSeries := extractDriverSeries(docContent)
	sort.Strings(docSeries)

	fprintln(stdout)
	fprintln(stdout, "Driver series mentioned in Flatcar NVIDIA docs:")
	if len(docSeries) == 0 {
		fprintln(stdout, "  (none found — docs may have changed structure)")
	} else {
		for _, series := range docSeries {
			id := strings.TrimPrefix(series, "nvidia-drivers-")
			fprintf(stdout, "  %s  (%s)\n", id, series)
		}
	}

	// ── Extract model.go entries ──────────────────────────────────────────
	fprintln(stdout)
	fprintln(stdout, "Driver series in model.go NvidiaDriverOptions:")
	modelIDs := model.DriverSeriesMap()
	var modelIDKeys []string
	for k := range modelIDs {
		modelIDKeys = append(modelIDKeys, k)
	}
	sort.Strings(modelIDKeys)

	modelIDSet := make(map[string]bool)
	for _, id := range modelIDKeys {
		modelIDSet[id] = true
		fprintf(stdout, "  %s  (nvidia-drivers-%s)\n", id, id)
	}

	// ── Compare ───────────────────────────────────────────────────────────
	fprintln(stdout)
	fprintln(stdout, "──────────────────────────────────────────────────────────────────────────")

	missing, extra := compareDriverSeries(docSeries, modelIDKeys)

	for _, id := range missing {
		fprintf(stdout, "⚠ MISSING IN MODEL: %s — mentioned in Flatcar docs but not in model.go\n", id)
	}
	for _, id := range extra {
		fprintf(stdout, "  NOTE: %s is in model.go but not mentioned in current Flatcar docs\n", id)
		fprintln(stdout, "        (This is normal — docs typically only show the recommended series)")
	}

	fprintln(stdout)
	if len(missing) > 0 {
		fprintln(stdout, "ACTION REQUIRED: Update internal/model/model.go NvidiaDriverOptions")
		fprintln(stdout)
		fprintln(stdout, "  1. Add the missing series to NvidiaDriverOptions in internal/model/model.go")
		fprintln(stdout, "  2. Set Recommended: true on the newest open-source series")
		fprintln(stdout, "  3. Update DefaultNvidiaDriverSeries to the newest recommended series")
		fprintln(stdout, "  4. Update Description field with GPU compatibility information")
		fprintln(stdout, "  5. Update the NVIDIA section in docs/SYSEXTS.md")
		fprintln(stdout, "  6. Run: just ci")
		return 1
	}

	fprintln(stdout, "✓ model.go NvidiaDriverOptions appears consistent with Flatcar docs.")
	fprintln(stdout)
	fprintln(stdout, "Note: The Flatcar docs may only show a single example series.")
	fprintln(stdout, "For authoritative driver series availability, check:")
	fprintln(stdout, "  https://www.flatcar.org/docs/latest/setup/customization/using-nvidia/")
	fprintln(stdout, "  https://github.com/flatcar/flatcar-website")
	return 0
}

// compareDriverSeries returns IDs missing from model and IDs extra in model.
// docSeries are full "nvidia-drivers-X" strings; modelIDs are short "X" keys.
func compareDriverSeries(docSeries, modelIDs []string) (missing, extra []string) {
	modelIDSet := make(map[string]bool, len(modelIDs))
	for _, id := range modelIDs {
		modelIDSet[id] = true
	}

	docSet := make(map[string]bool, len(docSeries))
	for _, series := range docSeries {
		docSet[series] = true
		id := strings.TrimPrefix(series, "nvidia-drivers-")
		if !modelIDSet[id] {
			missing = append(missing, id)
		}
	}

	for _, id := range modelIDs {
		if !docSet["nvidia-drivers-"+id] {
			extra = append(extra, id)
		}
	}
	return missing, extra
}

func fetchNvidiaDocs() (string, error) {
	resp, err := http.Get(docsURL)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var ghResp ghContentResponse
	if err := json.NewDecoder(resp.Body).Decode(&ghResp); err != nil {
		return "", err
	}

	if ghResp.Encoding != "base64" {
		return "", fmt.Errorf("unexpected encoding: %s", ghResp.Encoding)
	}

	data, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(ghResp.Content, "\n", ""))
	if err != nil {
		return "", err
	}

	return string(data), nil
}

var driverSeriesRE = regexp.MustCompile(`nvidia-drivers-[a-z0-9-]+`)

func extractDriverSeries(doc string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, match := range driverSeriesRE.FindAllString(doc, -1) {
		if !seen[match] {
			seen[match] = true
			result = append(result, match)
		}
	}
	return result
}
