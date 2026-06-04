package bakery

// FCOS community sysext support tier constants.
const (
	// TierFCOSCommunity marks extensions from the fedora-sysexts/community catalog.
	TierFCOSCommunity = "FCOS Community"
)

// fcosCatalog is the curated metadata catalog for fedora-sysexts/community extensions.
// When the community catalog contains an extension not listed here, FCOSLookup returns
// ok=false and the caller falls back to the raw GitHub release body as description.
var fcosCatalog = map[string]ExtensionMeta{
	"tailscale": {
		Category:    "Networking",
		SupportTier: TierFCOSCommunity,
		Short:       "Tailscale — zero-config WireGuard mesh VPN for FCOS nodes",
		Long:        "Tailscale creates an encrypted WireGuard mesh network between your nodes with no manual key exchange or firewall rules. Ships tailscale and tailscaled binaries with a systemd service unit. Authenticate with tailscale up after provisioning.",
		Caveats:     nil,
	},
	"docker-ce": {
		Category:    "Container Runtime",
		SupportTier: TierFCOSCommunity,
		Short:       "Docker CE — container runtime and CLI for FCOS",
		Long:        "Docker CE ships the Docker daemon, CLI, and containerd for Fedora CoreOS. Provides the familiar docker build/run/compose workflow. Compatible with Podman-based FCOS deployments as an alternative container runtime.",
		Caveats:     []string{"Conflicts with the Podman socket if both are running; disable one before starting the other."},
	},
	"vscode": {
		Category:    "Developer Tools",
		SupportTier: TierFCOSCommunity,
		Short:       "Visual Studio Code — Microsoft's editor with extensions support",
		Long:        "Visual Studio Code is a lightweight but powerful source code editor from Microsoft. Ships the code binary and supporting files for FCOS. Useful for development workflows directly on CoreOS nodes.",
		Caveats:     nil,
	},
	"vscodium": {
		Category:    "Developer Tools",
		SupportTier: TierFCOSCommunity,
		Short:       "VSCodium — community-built VS Code without Microsoft telemetry",
		Long:        "VSCodium is a community-maintained binary distribution of VS Code compiled without Microsoft branding or telemetry. Ships the codium binary and supporting files for FCOS.",
		Caveats:     nil,
	},
	"glab": {
		Category:    "Developer Tools",
		SupportTier: TierFCOSCommunity,
		Short:       "glab — GitLab CLI for managing MRs, issues, and pipelines",
		Long:        "glab is the official GitLab CLI tool. Manage merge requests, issues, CI/CD pipelines, and repositories directly from the terminal on FCOS.",
		Caveats:     nil,
	},
	"cilium-cli": {
		Category:    "Networking",
		SupportTier: TierFCOSCommunity,
		Short:       "Cilium CLI — install and manage Cilium CNI in Kubernetes clusters",
		Long:        "The Cilium CLI is used to install, configure, and troubleshoot Cilium, a powerful eBPF-based networking and security plugin for Kubernetes. Useful for bootstrapping Cilium on FCOS-based Kubernetes nodes.",
		Caveats:     nil,
	},
	"1password-gui": {
		Category:    "Security",
		SupportTier: TierFCOSCommunity,
		Short:       "1Password GUI — desktop 1Password client for credential management",
		Long:        "1Password is a password manager and secure credential vault. Ships the 1password GUI application for FCOS desktop or graphical deployments.",
		Caveats:     nil,
	},
	"1password-cli": {
		Category:    "Security",
		SupportTier: TierFCOSCommunity,
		Short:       "1Password CLI — terminal-based 1Password credential access",
		Long:        "The 1Password CLI (op) provides terminal access to your 1Password vaults and supports secret injection into shell environments. Useful for scripted FCOS workflows that need credential retrieval.",
		Caveats:     nil,
	},
	"kubernetes": {
		Category:    "Container Runtime",
		SupportTier: TierFCOSCommunity,
		Short:       "Kubernetes — container orchestration tools (kubectl, kubeadm, kubelet)",
		Long:        "Ships the core Kubernetes binaries — kubectl, kubeadm, and kubelet — for FCOS nodes. Enables FCOS hosts to join or bootstrap Kubernetes clusters. Pair with Ignition provisioning for unattended node setup.",
		Caveats:     []string{"Requires additional CNI configuration before cluster bring-up. Pin the version to match your cluster control-plane."},
	},
	"bitwarden": {
		Category:    "Security",
		SupportTier: TierFCOSCommunity,
		Short:       "Bitwarden — open-source password manager desktop client",
		Long:        "Bitwarden is a free and open-source password manager. Ships the Bitwarden desktop application for FCOS graphical deployments. Stores credentials in your self-hosted or cloud Bitwarden vault.",
		Caveats:     nil,
	},
}

// FCOSLookup returns the curated ExtensionMeta for an FCOS community sysext by name.
// Returns (ExtensionMeta{}, false) when the extension is not in the curated catalog.
func FCOSLookup(name string) (ExtensionMeta, bool) {
	meta, ok := fcosCatalog[name]
	return meta, ok
}
