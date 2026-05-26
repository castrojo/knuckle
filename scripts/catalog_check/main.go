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

var newCatalogFetcher = func() CatalogFetcher {
	return bakery.NewHTTPClient()
}

var exitFunc = os.Exit

func writef(w io.Writer, format string, a ...any) error {
	_, err := fmt.Fprintf(w, format, a...)
	if err != nil {
		return fmt.Errorf("writing report: %w", err)
	}
	return nil
}

func writeln(w io.Writer, a ...any) error {
	_, err := fmt.Fprintln(w, a...)
	if err != nil {
		return fmt.Errorf("writing report: %w", err)
	}
	return nil
}

func main() {
	exitFunc(mainWithArgs(os.Args[1:], os.Stdout, os.Stderr))
}

func mainWithArgs(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("catalog_check", flag.ContinueOnError)
	fs.SetOutput(stderr)

	strict := fs.Bool("strict", false, "exit 1 if any extensions are missing curated descriptions")
	arch := fs.String("arch", runtime.GOARCH, "architecture to query (default: host arch)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	if err := run(ctx, stdout, stderr, newCatalogFetcher(), *arch, *strict); err != nil {
		if errors.Is(err, errStrictViolation) {
			return 1
		}
		_, _ = fmt.Fprintf(stderr, "\nERROR: %v\n", err)
		return 2
	}

	return 0
}

// run executes the catalog check logic, writing output to w and errors to errW.
// It returns errStrictViolation if strict is true and entries are missing,
// or a wrapped error if the catalog fetch fails.
func run(ctx context.Context, w io.Writer, errW io.Writer, fetcher CatalogFetcher, arch string, strict bool) error {
	if err := writeln(w, "catalog_check — verifying descriptions.go against live Flatcar Bakery"); err != nil {
		return err
	}
	if err := writeln(w, strings.Repeat("─", 70)); err != nil {
		return err
	}
	if err := writeln(w); err != nil {
		return err
	}

	if err := writef(w, "Fetching live bakery catalog (%s)... ", arch); err != nil {
		return err
	}
	entries, err := fetcher.FetchCatalogArch(ctx, arch)
	if err != nil {
		return fmt.Errorf("fetching catalog: %w", err)
	}
	if err := writef(w, "%d extensions found\n\n", len(entries)); err != nil {
		return err
	}

	// Sort by name for stable output.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})

	covered, missing := checkCatalog(entries)

	for _, e := range entries {
		if meta, ok := bakery.Lookup(e.Name); ok {
			if err := writef(w, "  ok       %-22s  v%-12s  %s · %s\n",
				e.Name, e.Version, meta.SupportTier, meta.Category); err != nil {
				return err
			}
		} else {
			if err := writef(w, "  MISSING  %-22s  v%s\n", e.Name, e.Version); err != nil {
				return err
			}
		}
	}

	if err := writeln(w); err != nil {
		return err
	}
	if err := writef(w, "Result: %d covered, %d missing\n", covered, len(missing)); err != nil {
		return err
	}

	if len(missing) == 0 {
		if err := writeln(w); err != nil {
			return err
		}
		if err := writeln(w, "✓ All live bakery extensions have curated descriptions."); err != nil {
			return err
		}
		if err := writeln(w, "  No action needed."); err != nil {
			return err
		}
		return nil
	}

	if err := writeln(w); err != nil {
		return err
	}
	if err := writeln(w, "─── Missing entries ────────────────────────────────────────────────────"); err != nil {
		return err
	}
	if err := writef(w, "Add the following to extensionCatalog in internal/bakery/descriptions.go:\n\n"); err != nil {
		return err
	}

	for _, m := range missing {
		if err := writef(w, "// %s v%s — source: %s\n", m.Name, m.Version, m.URL); err != nil {
			return err
		}
		if err := writef(w, `"%s": {
	Category:    "TODO", // e.g. "Container Runtime", "Networking", "Orchestration"
	SupportTier: bakery.TierMaintained, // or TierIntegrated, TierExperimental
	Short:       "TODO: one-line description (~80 chars)",
	Long:        "TODO: 3–5 sentence description shown in the detail panel.",
	Caveats:     nil,
},
`, m.Name); err != nil {
			return err
		}
		if err := writeln(w); err != nil {
			return err
		}
	}

	if err := writeln(w, "─── Checklist ──────────────────────────────────────────────────────────"); err != nil {
		return err
	}
	if err := writeln(w, "1. Add the entry/entries above to internal/bakery/descriptions.go"); err != nil {
		return err
	}
	if err := writef(w, "2. Add %-22q to allKnownExtensions in internal/bakery/descriptions_test.go\n", missing[0].Name); err != nil {
		return err
	}
	if err := writeln(w, "3. Add a row to docs/SYSEXTS.md under the appropriate category"); err != nil {
		return err
	}
	if err := writeln(w, "4. Run: just ci"); err != nil {
		return err
	}
	if err := writeln(w); err != nil {
		return err
	}

	if strict {
		if err := writef(errW, "FAIL: %d extension(s) missing curated descriptions (--strict)\n", len(missing)); err != nil {
			return err
		}
		return errStrictViolation
	}

	if err := writef(w, "⚠ %d extension(s) are missing curated descriptions.\n", len(missing)); err != nil {
		return err
	}
	if err := writeln(w, "  Run 'just catalog-check-strict' to enforce as a hard gate."); err != nil {
		return err
	}
	return nil
}
