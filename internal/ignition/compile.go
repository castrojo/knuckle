// Package ignition generates Butane YAML configs and compiles them to Ignition JSON.
package ignition

import (
	"fmt"

	"github.com/coreos/butane/config"
	"github.com/coreos/butane/config/common"
	"github.com/coreos/vcontext/report"
)

var translateButane func([]byte, common.TranslateBytesOptions) ([]byte, report.Report, error) = config.TranslateBytes

// CompileToIgnition compiles Butane YAML to Ignition JSON using the coreos/butane
// Go library. This eliminates the need for the butane CLI binary, which is not
// available on Flatcar Container Linux.
func CompileToIgnition(butaneYAML string) (string, error) {
	options := common.TranslateBytesOptions{
		Raw:    true,
		Pretty: false,
	}

	ignitionJSON, report, err := translateButane([]byte(butaneYAML), options)
	if err != nil {
		return "", fmt.Errorf("butane compilation failed: %w\n%s", err, report.String())
	}

	if report.IsFatal() {
		return "", fmt.Errorf("butane compilation had fatal errors: %s", report.String())
	}

	return string(ignitionJSON), nil
}
