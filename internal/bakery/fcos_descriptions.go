package bakery

// Support tier constants for FCOS community extensions.
// These describe the origin and maintenance level of each extension in
// fedora-sysexts/community, not a quality certification.
const (
	// TierFCOSCommunity marks extensions from the fedora-sysexts/community catalog,
	// which publishes daily automated builds from official Fedora RPMs.
	TierFCOSCommunity = "FCOS Community"
)

// fcosCatalog is the curated metadata map for common fedora-sysexts/community extensions.
// When the catalog contains an extension not listed here, LookupFCOS returns ok=false
// and the TUI will display the raw GitHub release body text as the description.
var fcosCatalog = map[string]ExtensionMeta{
	"docker-ce": {
		Category:    "Container Runtime",
		SupportTier: TierFCOSCommunity,
		Short:       "Docker CE — community edition container engine from the official Fedora RPM",
		Long:        "Docker CE provides the full Docker container engine (daemon, CLI, containerd) installed from the official Fedora RPM package. Runs docker.socket and docker.service at boot. Use with docker-compose or docker-buildx sysexts for additional tooling.",
		Caveats:     nil,
	},
	"tailscale": {
		Category:    "Networking",
		SupportTier: TierFCOSCommunity,
		Short:       "Tailscale — zero-config WireGuard mesh VPN for secure node-to-node networking",
		Long:        "Tailscale creates an encrypted WireGuard mesh network between your nodes with no manual key exchange or firewall rules required. Ships tailscale, tailscaled, and a service unit that auto-starts at boot. Authenticate nodes with tailscale up after provisioning.",
		Caveats:     nil,
	},
	"vscode": {
		Category:    "Developer Tools",
		SupportTier: TierFCOSCommunity,
		Short:       "Visual Studio Code — Microsoft's open-source code editor (Fedora RPM build)",
		Long:        "Visual Studio Code is a lightweight but powerful source code editor with built-in support for debugging, syntax highlighting, and Git. Installed from the official Microsoft VS Code RPM repository. Suitable for developer workstations running FCOS.",
		Caveats:     []string{"VS Code is a desktop application — useful only when a graphical session or VS Code Server (Remote SSH) is configured."},
	},
	"vscodium": {
		Category:    "Developer Tools",
		SupportTier: TierFCOSCommunity,
		Short:       "VSCodium — VS Code without Microsoft telemetry or proprietary branding",
		Long:        "VSCodium is a community-maintained binary distribution of VS Code built from the open-source core without Microsoft telemetry, tracking, or proprietary extensions marketplace defaults. It uses the Open VSX Registry instead. Drop-in replacement for VS Code.",
		Caveats:     []string{"Uses Open VSX Registry by default — some Microsoft-only extensions may not be available."},
	},
	"kubernetes": {
		Category:    "Orchestration",
		SupportTier: TierFCOSCommunity,
		Short:       "Kubernetes — kubelet, kubeadm, and kubectl for FCOS cluster nodes",
		Long:        "Ships kubelet, kubeadm, and kubectl for deploying and managing Kubernetes clusters on FCOS. Supports both control-plane and worker node configurations. Use kubeadm init/join to bootstrap the cluster after provisioning.",
		Caveats:     nil,
	},
	"cilium-cli": {
		Category:    "Networking",
		SupportTier: TierFCOSCommunity,
		Short:       "Cilium CLI — eBPF-based Kubernetes networking and observability tooling",
		Long:        "Ships the cilium CLI for managing Cilium in a running Kubernetes cluster. Cilium provides eBPF-powered networking, security policy enforcement, and observability. This sysext provides the CLI only — the Cilium control plane runs as pods in the cluster.",
		Caveats:     []string{"CLI only — requires a running Kubernetes cluster with Cilium installed."},
	},
	"1password-cli": {
		Category:    "Security",
		SupportTier: TierFCOSCommunity,
		Short:       "1Password CLI — command-line interface for 1Password secrets manager",
		Long:        "1Password CLI (op) enables scripting and automation of 1Password vault access from the terminal. Useful for secrets injection at provisioning time or in service account workflows. Authenticate with op signin before use.",
		Caveats:     nil,
	},
	"1password-gui": {
		Category:    "Security",
		SupportTier: TierFCOSCommunity,
		Short:       "1Password GUI — desktop password manager application",
		Long:        "The 1Password desktop application for Linux provides a full-featured graphical password manager. Suitable for developer workstations running FCOS with a graphical session.",
		Caveats:     []string{"Desktop application — requires a graphical session."},
	},
	"google-chrome": {
		Category:    "Developer Tools",
		SupportTier: TierFCOSCommunity,
		Short:       "Google Chrome — web browser from the official Google RPM repository",
		Long:        "Google Chrome is a fast, secure web browser installed from the official Google Linux RPM repository. Suitable for developer workstations running FCOS with a graphical session.",
		Caveats:     []string{"Desktop application — requires a graphical session."},
	},
	"microsoft-edge": {
		Category:    "Developer Tools",
		SupportTier: TierFCOSCommunity,
		Short:       "Microsoft Edge — Chromium-based browser from the official Microsoft RPM",
		Long:        "Microsoft Edge is a Chromium-based browser with built-in Microsoft account integration, enterprise policy support, and developer tools. Installed from the official Microsoft Edge RPM repository.",
		Caveats:     []string{"Desktop application — requires a graphical session."},
	},
	"bitwarden": {
		Category:    "Security",
		SupportTier: TierFCOSCommunity,
		Short:       "Bitwarden — open-source password manager desktop application",
		Long:        "Bitwarden is an open-source password manager with end-to-end encryption. The desktop application connects to Bitwarden's cloud service or a self-hosted Vaultwarden instance.",
		Caveats:     []string{"Desktop application — requires a graphical session."},
	},
	"netbird": {
		Category:    "Networking",
		SupportTier: TierFCOSCommunity,
		Short:       "NetBird — peer-to-peer WireGuard overlay network with a management plane",
		Long:        "NetBird creates an encrypted WireGuard overlay mesh network. Ships the netbird daemon and CLI. Connect to NetBird's SaaS management plane or a self-hosted NetBird server. Start the daemon and authenticate with netbird up --setup-key.",
		Caveats:     nil,
	},
	"littlesnitch": {
		Category:    "Security",
		SupportTier: TierFCOSCommunity,
		Short:       "Little Snitch — application firewall that monitors and controls network connections",
		Long:        "Little Snitch is an application-layer firewall that monitors outgoing network connections and lets you allow or deny them per application. Primarily for macOS; this FCOS sysext ships the Little Snitch Network Monitor for Linux.",
		Caveats:     nil,
	},
}

// LookupFCOS returns curated metadata for the named FCOS community extension.
// Returns the metadata and true if found; zero ExtensionMeta and false if not.
// Unknown extensions should be displayed with Category "Other" and the raw GitHub release body.
func LookupFCOS(name string) (ExtensionMeta, bool) {
	meta, ok := fcosCatalog[name]
	return meta, ok
}
