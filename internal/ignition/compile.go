// Package ignition generates Butane YAML configs and compiles them to Ignition JSON.
package ignition

import (
"fmt"

"github.com/coreos/butane/config"
"github.com/coreos/butane/config/common"
"github.com/coreos/vcontext/report"
)

// translateFunc is the signature of config.TranslateBytes, extracted for testability.
type translateFunc func([]byte, common.TranslateBytesOptions) ([]byte, report.Report, error)

// CompileToIgnition compiles Butane YAML to Ignition JSON using the coreos/butane
// Go library. This eliminates the need for the butane CLI binary, which is not
// available on Flatcar Container Linux.
func CompileToIgnition(butaneYAML string) (string, error) {
return compileToIgnitionWith(config.TranslateBytes, butaneYAML)
}

// compileToIgnitionWith is the internal implementation that accepts a translator
// function, enabling tests to exercise all branches including the defensive
// report.IsFatal() check.
func compileToIgnitionWith(translate translateFunc, butaneYAML string) (string, error) {
options := common.TranslateBytesOptions{
Raw:    true,
Pretty: false,
}

ignitionJSON, rpt, err := translate([]byte(butaneYAML), options)
if err != nil {
return "", fmt.Errorf("butane compilation failed: %w\n%s", err, rpt.String())
}

// Defensive: check for fatal report entries even when err is nil.
// Current butane library always returns err when IsFatal(), but this
// guards against future library behavior changes.
if rpt.IsFatal() {
return "", fmt.Errorf("butane compilation had fatal errors: %s", rpt.String())
}

return string(ignitionJSON), nil
}
