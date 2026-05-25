// catalog_check verifies that internal/bakery/descriptions.go covers all
// extensions currently published in the live Flatcar Sysext Bakery.
//
// Usage:
//
//	go run ./scripts/catalog_check/           # informational report
//	go run ./scripts/catalog_check/ --strict  # exit 1 if any gaps found
//
// Run before cutting a release to catch new bakery extensions that need
// curated descriptions. Not part of `just ci` (requires network); run
// manually or via `just catalog-check`.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/projectbluefin/knuckle/internal/bakery"
	"github.com/projectbluefin/knuckle/internal/model"
)

// CatalogFetcher abstracts catalog retrieval so tests can inject a mock.
type CatalogFetcher interface {
	FetchCatalogArch(ctx context.Context, arch string) ([]model.SysextEntry, error)
}

// errStrictViolation is returned by run when --strict is set and entries are missing.
var errStrictViolation = errors.New("strict mode: missing curated descriptions")

func main() {
	strict := flag.Bool("strict", false, "exit 1 if any extensions are missing curated descriptions")
	arch := flag.String("arch", runtime.GOARCH, "architecture to query (default: host arch)")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	client := bakery.NewHTTPClient()

	if err := run(ctx, os.Stdout, os.Stderr, client, *arch, *strict); err != nil {
		if errors.Is(err, errStrictViolation) {
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "\nERROR: %v\n", err)
		os.Exit(2)
	}
}

// run executes the catalog check logic, writing output to w and errors to errW.
// It returns errStrictViolation if strict is true and entries are missing,
// or a wrapped error if the catalog fetch fails.
func run(ctx context.Context, w io.Writer, errW io.Writer, fetcher CatalogFetcher, arch string, strict bool) error {
	fmt.Fprintln(w, "catalog_check — verifying descriptions.go against live Flatcar Bakery")
	fmt.Fprintln(w, strings.Repeat("─", 70))
	fmt.Fprintln(w)

	fmt.Fprintf(w, "Fetching live bakery catalog (%s)... ", arch)
	entries, err := fetcher.FetchCatalogArch(ctx, arch)
	if err != nil {
		return fmt.Errorf("fetching catalog: %w", err)
	}
	fmt.Fprintf(w, "%d extensions found\n\n", len(entries))

	// Sort by name for stable output.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})

	covered, missing := checkCatalog(entries)

	for _, e := range entries {
		if meta, ok := bakery.Lookup(e.Name); ok {
			fmt.Fprintf(w, "  ok       %-22s  v%-12s  %s · %s\n",
				e.Name, e.Version, meta.SupportTier, meta.Category)
		} else {
			fmt.Fprintf(w, "  MISSING  %-22s  v%s\n", e.Name, e.Version)
		}
	}

	fmt.Fprintln(w)
	fmt.Fprintf(w, "Result: %d covered, %d missing\n", covered, len(missing))

	if len(missing) == 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "✓ All live bakery extensions have curated descriptions.")
		fmt.Fprintln(w, "  No action needed.")
		return nil
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "─── Missing entries ────────────────────────────────────────────────────")
	fmt.Fprintf(w, "Add the following to extensionCatalog in internal/bakery/descriptions.go:\n\n")

	for _, m := range missing {
		fmt.Fprintf(w, "// %s v%s — source: %s\n", m.Name, m.Version, m.URL)
		fmt.Fprintf(w, `"%s": {
	Category:    "TODO", // e.g. "Container Runtime", "Networking", "Orchestration"
	SupportTier: bakery.TierMaintained, // or TierIntegrated, TierExperimental
	Short:       "TODO: one-line description (~80 chars)",
	Long:        "TODO: 3–5 sentence description shown in the detail panel.",
	Caveats:     nil,
},
`, m.Name)
		fmt.Fprintln(w)
	}

	fmt.Fprintln(w, "─── Checklist ──────────────────────────────────────────────────────────")
	fmt.Fprintln(w, "1. Add the entry/entries above to internal/bakery/descriptions.go")
	fmt.Fprintf(w, "2. Add %-22q to allKnownExtensions in internal/bakery/descriptions_test.go\n", missing[0].Name)
	fmt.Fprintln(w, "3. Add a row to docs/SYSEXTS.md under the appropriate category")
	fmt.Fprintln(w, "4. Run: just ci")
	fmt.Fprintln(w)

	if strict {
		fmt.Fprintf(errW, "FAIL: %d extension(s) missing curated descriptions (--strict)\n", len(missing))
		return errStrictViolation
	}

	fmt.Fprintf(w, "⚠ %d extension(s) are missing curated descriptions.\n", len(missing))
	fmt.Fprintln(w, "  Run 'just catalog-check-strict' to enforce as a hard gate.")
	return nil
}
