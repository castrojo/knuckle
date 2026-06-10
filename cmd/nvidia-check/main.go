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
	docsURL = "https://api.github.com/repos/flatcar/flatcar-website/contents/content/docs/latest/setup/customization/using-nvidia.md"
	modelGo = "internal/model/model.go"
)

// ghContentResponse is the minimal GitHub API response for file contents.
type ghContentResponse struct {
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
}

func main() {
	os.Exit(run(os.Stdout, fetchNvidiaDocs))
}

// run is the testable entry point. It writes output to w and uses fetch to
// retrieve the Flatcar NVIDIA documentation content.
// Returns 0 on success, 1 when model.go needs updating, 2 on fetch error.
func run(w io.Writer, fetch func() (string, error)) int {
	_, _ = fmt.Fprintln(w, "nvidia_check — verifying model.go NvidiaDriverOptions against Flatcar docs")
	_, _ = fmt.Fprintln(w, "──────────────────────────────────────────────────────────────────────────")
	_, _ = fmt.Fprintln(w)

	// ── Fetch Flatcar docs ────────────────────────────────────────────────
	_, _ = fmt.Fprintln(w, "Fetching Flatcar NVIDIA docs...")
	docContent, err := fetch()
	if err != nil {
		_, _ = fmt.Fprintf(w, "ERROR: Could not fetch Flatcar NVIDIA docs from GitHub.\n")
		_, _ = fmt.Fprintf(w, "  Check network or try: curl -sf '%s'\n", docsURL)
		return 2
	}

	// Extract all nvidia-drivers-* patterns from the docs.
	docSeries := extractDriverSeries(docContent)
	sort.Strings(docSeries)

	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "Driver series mentioned in Flatcar NVIDIA docs:")
	if len(docSeries) == 0 {
		_, _ = fmt.Fprintln(w, "  (none found — docs may have changed structure)")
	} else {
		for _, series := range docSeries {
			id := strings.TrimPrefix(series, "nvidia-drivers-")
			_, _ = fmt.Fprintf(w, "  %s  (%s)\n", id, series)
		}
	}

	// ── Extract model.go entries ──────────────────────────────────────────
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "Driver series in model.go NvidiaDriverOptions:")
	modelIDs := model.DriverSeriesMap()
	var modelIDKeys []string
	for k := range modelIDs {
		modelIDKeys = append(modelIDKeys, k)
	}
	sort.Strings(modelIDKeys)

	modelIDSet := make(map[string]bool)
	for _, id := range modelIDKeys {
		modelIDSet[id] = true
		_, _ = fmt.Fprintf(w, "  %s  (nvidia-drivers-%s)\n", id, id)
	}

	// ── Compare ───────────────────────────────────────────────────────────
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "──────────────────────────────────────────────────────────────────────────")

	docSet := make(map[string]bool)
	needsAdd := false
	for _, series := range docSeries {
		docSet[series] = true
		id := strings.TrimPrefix(series, "nvidia-drivers-")
		if !modelIDSet[id] {
			_, _ = fmt.Fprintf(w, "⚠ MISSING IN MODEL: %s — mentioned in Flatcar docs but not in model.go\n", id)
			needsAdd = true
		}
	}

	for _, id := range modelIDKeys {
		if !docSet["nvidia-drivers-"+id] {
			_, _ = fmt.Fprintf(w, "  NOTE: %s is in model.go but not mentioned in current Flatcar docs\n", id)
			_, _ = fmt.Fprintln(w, "        (This is normal — docs typically only show the recommended series)")
		}
	}

	_, _ = fmt.Fprintln(w)
	if needsAdd {
		_, _ = fmt.Fprintln(w, "ACTION REQUIRED: Update internal/model/model.go NvidiaDriverOptions")
		_, _ = fmt.Fprintln(w)
		_, _ = fmt.Fprintln(w, "  1. Add the missing series to NvidiaDriverOptions in internal/model/model.go")
		_, _ = fmt.Fprintln(w, "  2. Set Recommended: true on the newest open-source series")
		_, _ = fmt.Fprintln(w, "  3. Update DefaultNvidiaDriverSeries to the newest recommended series")
		_, _ = fmt.Fprintln(w, "  4. Update Description field with GPU compatibility information")
		_, _ = fmt.Fprintln(w, "  5. Update the NVIDIA section in docs/SYSEXTS.md")
		_, _ = fmt.Fprintln(w, "  6. Run: just ci")
		return 1
	}

	_, _ = fmt.Fprintln(w, "✓ model.go NvidiaDriverOptions appears consistent with Flatcar docs.")
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "Note: The Flatcar docs may only show a single example series.")
	_, _ = fmt.Fprintln(w, "For authoritative driver series availability, check:")
	_, _ = fmt.Fprintln(w, "  https://www.flatcar.org/docs/latest/setup/customization/using-nvidia/")
	_, _ = fmt.Fprintln(w, "  https://github.com/flatcar/flatcar-website")
	return 0
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
