package bakery

// FCOS support tier constants for fedora-sysexts/community extensions.
const (
	FCOSTierCommunity = "Community"
)

// FCOSExtensionMeta holds curated display metadata for a fedora-sysexts/community extension.
type FCOSExtensionMeta struct {
	Category    string
	SupportTier string
	Short       string
	Long        string
	Caveats     []string
}

// fcosCatalog is the static curated catalog for fedora-sysexts/community extensions.
// When the catalog adds new extensions not present here, FCOSLookup returns ok=false
// and the raw GitHub release body is used as the description.
var fcosCatalog = map[string]FCOSExtensionMeta{
	"docker-ce": {
		Category:    "Container Runtime",
		SupportTier: FCOSTierCommunity,
		Short:       "Docker CE — community container engine for Fedora CoreOS",
		Long:        "Docker Community Edition provides the Docker daemon, CLI, containerd, and runc packaged as a systemd-sysext for Fedora CoreOS. Built from official Fedora RPMs. Starts via socket activation by default.",
	},
	"tailscale": {
		Category:    "Networking",
		SupportTier: FCOSTierCommunity,
		Short:       "Tailscale — zero-config WireGuard mesh VPN for FCOS nodes",
		Long:        "Tailscale creates an encrypted WireGuard mesh network between nodes. Ships tailscale and tailscaled binaries as a systemd-sysext. Authenticate with tailscale up after provisioning.",
	},
	"vscode": {
		Category:    "Development",
		SupportTier: FCOSTierCommunity,
		Short:       "Visual Studio Code — Microsoft's open-source code editor",
		Long:        "Visual Studio Code packaged as a systemd-sysext for Fedora CoreOS. Built from official Microsoft RPMs. Provides the code CLI and desktop application.",
	},
	"vscodium": {
		Category:    "Development",
		SupportTier: FCOSTierCommunity,
		Short:       "VSCodium — free/libre build of VS Code without Microsoft telemetry",
		Long:        "VSCodium is a community-driven, freely-licensed build of Visual Studio Code with Microsoft telemetry removed. Packaged as a systemd-sysext for Fedora CoreOS.",
	},
	"cilium-cli": {
		Category:    "Networking",
		SupportTier: FCOSTierCommunity,
		Short:       "Cilium CLI — eBPF-based Kubernetes networking and observability",
		Long:        "The Cilium CLI for managing eBPF-powered Kubernetes networking, security policies, and observability. Packaged as a systemd-sysext for Fedora CoreOS.",
	},
	"1password-gui": {
		Category:    "Security",
		SupportTier: FCOSTierCommunity,
		Short:       "1Password — password manager desktop application",
		Long:        "1Password desktop application packaged as a systemd-sysext for Fedora CoreOS. Provides secure credential storage and autofill.",
	},
	"cloud-hypervisor": {
		Category:    "Virtualization",
		SupportTier: FCOSTierCommunity,
		Short:       "Cloud Hypervisor — lightweight VMM for modern cloud workloads",
		Long:        "Cloud Hypervisor is a lightweight Virtual Machine Monitor built on KVM. Designed for running modern cloud workloads with minimal overhead. Packaged as a systemd-sysext for Fedora CoreOS.",
	},
	"glab": {
		Category:    "Development",
		SupportTier: FCOSTierCommunity,
		Short:       "GLab — GitLab CLI for issues, MRs, pipelines, and more",
		Long:        "GLab is the official GitLab CLI tool for managing issues, merge requests, pipelines, and other GitLab resources from the terminal. Packaged as a systemd-sysext for Fedora CoreOS.",
	},
	"bitwarden": {
		Category:    "Security",
		SupportTier: FCOSTierCommunity,
		Short:       "Bitwarden — open-source password manager desktop application",
		Long:        "Bitwarden desktop application packaged as a systemd-sysext for Fedora CoreOS. Provides self-hostable secure credential management.",
	},
	"ghostty": {
		Category:    "Utilities",
		SupportTier: FCOSTierCommunity,
		Short:       "Ghostty — fast, feature-rich GPU-accelerated terminal emulator",
		Long:        "Ghostty is a GPU-accelerated terminal emulator focused on performance and correct terminal emulation. Packaged as a systemd-sysext for Fedora CoreOS.",
	},
	"google-chrome": {
		Category:    "Development",
		SupportTier: FCOSTierCommunity,
		Short:       "Google Chrome — Google's web browser",
		Long:        "Google Chrome web browser packaged as a systemd-sysext for Fedora CoreOS. Built from official Google RPMs.",
	},
	"microsoft-edge": {
		Category:    "Development",
		SupportTier: FCOSTierCommunity,
		Short:       "Microsoft Edge — Chromium-based web browser from Microsoft",
		Long:        "Microsoft Edge web browser packaged as a systemd-sysext for Fedora CoreOS. Built from official Microsoft RPMs.",
	},
}

// FCOSLookup returns curated metadata for the named FCOS extension.
// Returns the metadata and true if found; zero value and false if not in catalog.
func FCOSLookup(name string) (FCOSExtensionMeta, bool) {
	meta, ok := fcosCatalog[name]
	return meta, ok
}
