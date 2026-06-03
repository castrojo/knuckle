package ignition

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/projectbluefin/knuckle/internal/model"
)

// Builder assembles a Butane YAML document from named sections.
//
// Why this exists: the original Butane generator was a single multi-hundred-line
// raw string template. Every new conditional block (NVIDIA, Tailscale, swap…)
// had to be hand-indented into the central template, which made the surface
// fragile as features grew (#88).
//
// Builder lets each feature contribute a typed section. Sections are rendered
// in a stable order and joined into a single Butane document; the document is
// still compiled to Ignition by CompileToIgnition.
//
// The builder is intentionally narrow: it produces YAML strings, not Butane
// Go types. That keeps the dependency surface small (no need to track Butane
// Go-type API drift across releases) while still giving us a structured
// assembly point.
type Builder struct {
	cfg *model.InstallConfig

	// Each map slot is rendered in this order. Keep the slice sorted by the
	// order the YAML keys appear in a Butane document.
	storageFiles    []string // YAML fragments under storage.files
	storageLinks    []string // YAML fragments under storage.links
	systemdUnits    []string // YAML fragments under systemd.units
	passwdUsersYAML string   // entire passwd.users block (mutually exclusive contents)
}

// NewBuilder returns a Builder seeded with cfg.
func NewBuilder(cfg *model.InstallConfig) *Builder {
	return &Builder{cfg: cfg}
}

// AddStorageFile appends a YAML fragment that will be placed under
// `storage.files`. The fragment must be valid Butane YAML for a single file
// entry; the builder takes care of leading-dash alignment.
func (b *Builder) AddStorageFile(fragment string) {
	b.storageFiles = append(b.storageFiles, fragment)
}

// AddStorageLink appends a YAML fragment placed under `storage.links`.
func (b *Builder) AddStorageLink(fragment string) {
	b.storageLinks = append(b.storageLinks, fragment)
}

// AddSystemdUnit appends a YAML fragment placed under `systemd.units`.
func (b *Builder) AddSystemdUnit(fragment string) {
	b.systemdUnits = append(b.systemdUnits, fragment)
}

// SetPasswdUsers sets the rendered passwd.users block. Calling twice
// overwrites the previous value (the wizard only ever produces one).
func (b *Builder) SetPasswdUsers(fragment string) {
	b.passwdUsersYAML = fragment
}

// BuildWithVariant assembles the final Butane YAML document using the given
// variant header (e.g. "variant: fcos\nversion: 1.5.0\n").
func (b *Builder) BuildWithVariant(header string) string {
	var doc strings.Builder
	doc.WriteString(header)

	if len(b.storageFiles) > 0 || len(b.storageLinks) > 0 {
		doc.WriteString("storage:\n")
		if len(b.storageFiles) > 0 {
			doc.WriteString("  files:\n")
			for _, f := range b.storageFiles {
				doc.WriteString(indentFragment(f, 4))
			}
		}
		if len(b.storageLinks) > 0 {
			doc.WriteString("  links:\n")
			for _, l := range b.storageLinks {
				doc.WriteString(indentFragment(l, 4))
			}
		}
	}

	if len(b.systemdUnits) > 0 {
		doc.WriteString("systemd:\n  units:\n")
		for _, u := range b.systemdUnits {
			doc.WriteString(indentFragment(u, 4))
		}
	}

	if b.passwdUsersYAML != "" {
		doc.WriteString("passwd:\n  users:\n")
		doc.WriteString(indentFragment(b.passwdUsersYAML, 4))
	}

	return doc.String()
}

// Build assembles the final Butane YAML document with the Flatcar variant header.
func (b *Builder) Build() string {
	return b.BuildWithVariant("variant: flatcar\nversion: 1.1.0\n")
}

// BuildFCOS assembles the final Butane YAML document with the FCOS variant header.
func (b *Builder) BuildFCOS() string {
	return b.BuildWithVariant("variant: fcos\nversion: 1.5.0\n")
}

// indentFragment ensures every line of fragment is at least baseIndent spaces
// from column zero (preserves relative indentation between lines).
func indentFragment(fragment string, baseIndent int) string {
	pad := strings.Repeat(" ", baseIndent)
	lines := strings.Split(strings.TrimRight(fragment, "\n"), "\n")
	var out strings.Builder
	for _, l := range lines {
		if l == "" {
			out.WriteString("\n")
			continue
		}
		out.WriteString(pad)
		out.WriteString(l)
		out.WriteString("\n")
	}
	return out.String()
}

// renderTemplate is a small helper to evaluate a sub-template against data
// with the same FuncMap the main template uses.
func renderTemplate(name, body string, data any) (string, error) {
	tmpl, err := template.New(name).Funcs(builderFuncMap).Parse(body)
	if err != nil {
		return "", fmt.Errorf("parsing %s: %w", name, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("executing %s: %w", name, err)
	}
	return buf.String(), nil
}

// builderFuncMap is shared by every section template so YAML escaping is
// consistent across features.
var builderFuncMap = template.FuncMap{
	"yamlEscape": func(s string) string {
		s = strings.ReplaceAll(s, `\`, `\\`)
		s = strings.ReplaceAll(s, `"`, `\"`)
		s = strings.ReplaceAll(s, "\n", `\n`)
		s = strings.ReplaceAll(s, "\r", `\r`)
		s = strings.ReplaceAll(s, "\t", `\t`)
		return s
	},
}
