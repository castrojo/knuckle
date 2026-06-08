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

// run contains the main logic, extracted for testability.
// Returns 0 on success, 1 when model is missing entries, 2 on fetch failure.
func run(stdout, stderr io.Writer) int {
	fmt.Fprintln(stdout, "nvidia_check — verifying model.go NvidiaDriverOptions against Flatcar docs")
	fmt.Fprintln(stdout, "──────────────────────────────────────────────────────────────────────────")
	fmt.Fprintln(stdout)

	// ── Fetch Flatcar docs ────────────────────────────────────────────────
	fmt.Fprintln(stdout, "Fetching Flatcar NVIDIA docs...")
	docContent, err := fetchNvidiaDocs()
	if err != nil {
		fmt.Fprintf(stderr, "ERROR: Could not fetch Flatcar NVIDIA docs from GitHub.\n")
		fmt.Fprintf(stderr, "  Check network or try: curl -sf '%s'\n", docsURL)
		return 2
	}

	// Extract all nvidia-drivers-* patterns from the docs.
	docSeries := extractDriverSeries(docContent)
	sort.Strings(docSeries)

	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Driver series mentioned in Flatcar NVIDIA docs:")
	if len(docSeries) == 0 {
		fmt.Fprintln(stdout, "  (none found — docs may have changed structure)")
	} else {
		for _, series := range docSeries {
			id := strings.TrimPrefix(series, "nvidia-drivers-")
			fmt.Fprintf(stdout, "  %s  (%s)\n", id, series)
		}
	}

	// ── Extract model.go entries ──────────────────────────────────────────
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Driver series in model.go NvidiaDriverOptions:")
	modelIDs := model.DriverSeriesMap()
	var modelIDKeys []string
	for k := range modelIDs {
		modelIDKeys = append(modelIDKeys, k)
	}
	sort.Strings(modelIDKeys)

	modelIDSet := make(map[string]bool)
	for _, id := range modelIDKeys {
		modelIDSet[id] = true
		fmt.Fprintf(stdout, "  %s  (nvidia-drivers-%s)\n", id, id)
	}

	// ── Compare ───────────────────────────────────────────────────────────
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "──────────────────────────────────────────────────────────────────────────")

	missing, extra := compareDriverSeries(docSeries, modelIDKeys)

	for _, id := range missing {
		fmt.Fprintf(stdout, "⚠ MISSING IN MODEL: %s — mentioned in Flatcar docs but not in model.go\n", id)
	}
	for _, id := range extra {
		fmt.Fprintf(stdout, "  NOTE: %s is in model.go but not mentioned in current Flatcar docs\n", id)
		fmt.Fprintln(stdout, "        (This is normal — docs typically only show the recommended series)")
	}

	fmt.Fprintln(stdout)
	if len(missing) > 0 {
		fmt.Fprintln(stdout, "ACTION REQUIRED: Update internal/model/model.go NvidiaDriverOptions")
		fmt.Fprintln(stdout)
		fmt.Fprintln(stdout, "  1. Add the missing series to NvidiaDriverOptions in internal/model/model.go")
		fmt.Fprintln(stdout, "  2. Set Recommended: true on the newest open-source series")
		fmt.Fprintln(stdout, "  3. Update DefaultNvidiaDriverSeries to the newest recommended series")
		fmt.Fprintln(stdout, "  4. Update Description field with GPU compatibility information")
		fmt.Fprintln(stdout, "  5. Update the NVIDIA section in docs/SYSEXTS.md")
		fmt.Fprintln(stdout, "  6. Run: just ci")
		return 1
	}

	fmt.Fprintln(stdout, "✓ model.go NvidiaDriverOptions appears consistent with Flatcar docs.")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Note: The Flatcar docs may only show a single example series.")
	fmt.Fprintln(stdout, "For authoritative driver series availability, check:")
	fmt.Fprintln(stdout, "  https://www.flatcar.org/docs/latest/setup/customization/using-nvidia/")
	fmt.Fprintln(stdout, "  https://github.com/flatcar/flatcar-website")
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
