package ignition

import (
"errors"
"strings"
"testing"

"github.com/coreos/butane/config/common"
"github.com/coreos/vcontext/path"
"github.com/coreos/vcontext/report"
)

// TestCompileToIgnitionWith_FatalReport exercises the report.IsFatal() branch
// which is unreachable with the current butane library (TranslateBytes returns
// err when report is fatal) but exists as defensive code against future library
// behavior changes.
func TestCompileToIgnitionWith_FatalReport(t *testing.T) {
mockTranslator := func(_ []byte, _ common.TranslateBytesOptions) ([]byte, report.Report, error) {
r := report.Report{}
r.AddOnError(path.ContextPath{}, errors.New("simulated fatal report entry"))
return []byte(`{"ignition":{"version":"3.4.0"}}`), r, nil
}

_, err := compileToIgnitionWith(mockTranslator, "anything")
if err == nil {
t.Fatal("expected error when report.IsFatal() is true")
}
if !strings.Contains(err.Error(), "fatal errors") {
t.Errorf("expected 'fatal errors' in message, got: %v", err)
}
}

// TestCompileToIgnitionWith_TranslateError exercises the err != nil branch.
func TestCompileToIgnitionWith_TranslateError(t *testing.T) {
mockTranslator := func(_ []byte, _ common.TranslateBytesOptions) ([]byte, report.Report, error) {
return nil, report.Report{}, errors.New("translate failed")
}

_, err := compileToIgnitionWith(mockTranslator, "anything")
if err == nil {
t.Fatal("expected error when translate returns error")
}
if !strings.Contains(err.Error(), "butane compilation failed") {
t.Errorf("expected 'butane compilation failed' in message, got: %v", err)
}
}

// TestCompileToIgnitionWith_Success verifies the happy path through the
// testable internal function.
func TestCompileToIgnitionWith_Success(t *testing.T) {
mockTranslator := func(_ []byte, _ common.TranslateBytesOptions) ([]byte, report.Report, error) {
return []byte(`{"ignition":{"version":"3.4.0"}}`), report.Report{}, nil
}

got, err := compileToIgnitionWith(mockTranslator, "anything")
if err != nil {
t.Fatalf("unexpected error: %v", err)
}
if !strings.Contains(got, "3.4.0") {
t.Errorf("expected ignition version in output, got: %s", got)
}
}
