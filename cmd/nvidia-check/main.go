package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
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

func main() {
	fmt.Println("nvidia_check — verifying model.go NvidiaDriverOptions against Flatcar docs")
	fmt.Println("──────────────────────────────────────────────────────────────────────────")
	fmt.Println()

	// ── Fetch Flatcar docs ────────────────────────────────────────────────
	fmt.Println("Fetching Flatcar NVIDIA docs...")
	docContent, err := fetchNvidiaDocs()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Could not fetch Flatcar NVIDIA docs from GitHub.\n")
		fmt.Fprintf(os.Stderr, "  Check network or try: curl -sf '%s'\n", docsURL)
		os.Exit(2)
	}

	// Extract all nvidia-drivers-* patterns from the docs.
	docSeries := extractDriverSeries(docContent)
	sort.Strings(docSeries)

	fmt.Println()
	fmt.Println("Driver series mentioned in Flatcar NVIDIA docs:")
	if len(docSeries) == 0 {
		fmt.Println("  (none found — docs may have changed structure)")
	} else {
		for _, series := range docSeries {
			id := strings.TrimPrefix(series, "nvidia-drivers-")
			fmt.Printf("  %s  (%s)\n", id, series)
		}
	}

	// ── Extract model.go entries ──────────────────────────────────────────
	fmt.Println()
	fmt.Println("Driver series in model.go NvidiaDriverOptions:")
	modelIDs := model.DriverSeriesMap()
	var modelIDKeys []string
	for k := range modelIDs {
		modelIDKeys = append(modelIDKeys, k)
	}
	sort.Strings(modelIDKeys)

	modelIDSet := make(map[string]bool)
	for _, id := range modelIDKeys {
		modelIDSet[id] = true
		fmt.Printf("  %s  (nvidia-drivers-%s)\n", id, id)
	}

	// ── Compare ───────────────────────────────────────────────────────────
	fmt.Println()
	fmt.Println("──────────────────────────────────────────────────────────────────────────")

	missing, extra := compareDriverSeries(docSeries, modelIDKeys)

	for _, id := range missing {
		fmt.Printf("⚠ MISSING IN MODEL: %s — mentioned in Flatcar docs but not in model.go\n", id)
	}
	for _, id := range extra {
		fmt.Printf("  NOTE: %s is in model.go but not mentioned in current Flatcar docs\n", id)
		fmt.Println("        (This is normal — docs typically only show the recommended series)")
	}

	fmt.Println()
	if len(missing) > 0 {
		fmt.Println("ACTION REQUIRED: Update internal/model/model.go NvidiaDriverOptions")
		fmt.Println()
		fmt.Println("  1. Add the missing series to NvidiaDriverOptions in internal/model/model.go")
		fmt.Println("  2. Set Recommended: true on the newest open-source series")
		fmt.Println("  3. Update DefaultNvidiaDriverSeries to the newest recommended series")
		fmt.Println("  4. Update Description field with GPU compatibility information")
		fmt.Println("  5. Update the NVIDIA section in docs/SYSEXTS.md")
		fmt.Println("  6. Run: just ci")
		os.Exit(1)
	} else {
		fmt.Println("✓ model.go NvidiaDriverOptions appears consistent with Flatcar docs.")
		fmt.Println()
		fmt.Println("Note: The Flatcar docs may only show a single example series.")
		fmt.Println("For authoritative driver series availability, check:")
		fmt.Println("  https://www.flatcar.org/docs/latest/setup/customization/using-nvidia/")
		fmt.Println("  https://github.com/flatcar/flatcar-website")
	}
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
