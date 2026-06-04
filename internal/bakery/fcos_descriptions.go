package bakery

// FCOS support tier constants describe integration and maintenance level for
// fedora-sysexts/community extensions.
const (
	// TierFCOSCommunity marks extensions maintained by the fedora-sysexts
	// community project, built daily from upstream sources.
	TierFCOSCommunity = "FCOS Community"
)

// fcosCatalog is the curated metadata catalog for fedora-sysexts/community
// extensions.  Descriptions are written for FCOS users on bare-metal nodes.
// When a name is not found here, FCOSLookup returns ok=false and the caller
// falls back to the raw GitHub release body.
var fcosCatalog = map[string]ExtensionMeta{
	"docker-ce": {
		Category:    "Container Runtime",
		SupportTier: TierFCOSCommunity,
		Short:       "Docker CE — container engine with daemon, CLI, and BuildKit for FCOS nodes",
		Long:        "Installs the full Docker CE engine (dockerd, docker CLI, containerd, runc) from the official Docker upstream RPMs, repackaged as a sysext for FCOS. Docker starts via socket activation and integrates with systemd on first boot. Pair with docker-buildx or docker-compose sysexts for a complete build environment.",
		Caveats:     []string{"Replaces the default container runtime. Verify compatibility with your workloads before deploying to production."},
	},
	"tailscale": {
		Category:    "Networking",
		SupportTier: TierFCOSCommunity,
		Short:       "Tailscale — zero-config WireGuard mesh VPN for secure node-to-node networking",
		Long:        "Tailscale creates an encrypted WireGuard mesh network between your FCOS nodes with no manual key exchange or firewall configuration required. Ships tailscale and tailscaled with a service unit that auto-starts at boot. Authenticate nodes with tailscale up after provisioning.",
		Caveats:     nil,
	},
	"vscode": {
		Category:    "Developer Tools",
		SupportTier: TierFCOSCommunity,
		Short:       "Visual Studio Code — lightweight source editor from Microsoft, for remote-SSH workflows",
		Long:        "Installs the VS Code server binary, enabling remote development via the VS Code Remote - SSH extension. Useful when FCOS is used as a development or build host. Ships the code CLI and server binary; does not start a GUI on the node.",
		Caveats:     []string{"Requires a display server or Remote - SSH connection from a desktop VS Code instance."},
	},
	"vscodium": {
		Category:    "Developer Tools",
		SupportTier: TierFCOSCommunity,
		Short:       "VSCodium — open-source VS Code build without Microsoft telemetry",
		Long:        "VSCodium is the community-compiled build of VS Code with Microsoft proprietary components and telemetry removed. Ships the codium binary and remote server for SSH-based development workflows on FCOS nodes. Same feature set as VS Code for most use cases.",
		Caveats:     nil,
	},
	"cilium-cli": {
		Category:    "Networking",
		SupportTier: TierFCOSCommunity,
		Short:       "Cilium CLI — manage eBPF-based Kubernetes networking and security policies",
		Long:        "Ships the cilium CLI for installing, inspecting, and troubleshooting Cilium in a Kubernetes cluster. Does not ship the Cilium agent itself — this is a management tool for interacting with an existing Cilium installation. Pair with a Kubernetes distribution such as kubeadm.",
		Caveats:     nil,
	},
	"1password-gui": {
		Category:    "Security",
		SupportTier: TierFCOSCommunity,
		Short:       "1Password — GUI password manager and secrets integration for FCOS workstations",
		Long:        "Installs the 1Password GUI application and CLI for managing credentials and secrets on FCOS-based workstation or developer nodes. Also provides the op CLI for scripted secrets access. Requires a graphical session or remote desktop to use the GUI; op works in headless environments.",
		Caveats:     []string{"GUI requires a display server (Wayland or X11). The op CLI is available without a display."},
	},
	"kubernetes": {
		Category:    "Orchestration",
		SupportTier: TierFCOSCommunity,
		Short:       "Kubernetes — kubelet, kubeadm, and kubectl for FCOS cluster nodes",
		Long:        "Ships kubelet, kubeadm, and kubectl from the official Kubernetes RPM repositories, repackaged as a sysext for FCOS. Use kubeadm init on control-plane nodes and kubeadm join on workers. FCOS's immutable base image pairs well with Kubernetes' declarative workload model.",
		Caveats:     []string{"Update within the same minor version only (e.g. v1.32.x → v1.32.y). Cross-minor upgrades require manual coordination."},
	},
	"netbird": {
		Category:    "Networking",
		SupportTier: TierFCOSCommunity,
		Short:       "NetBird — peer-to-peer WireGuard overlay network with centralized management",
		Long:        "NetBird creates a peer-to-peer WireGuard overlay network managed through a control plane. Ships the netbird daemon and CLI with a service unit that starts at boot. Configure via /etc/netbird/ or environment variables. Suitable for multi-site or hybrid-cloud FCOS deployments.",
		Caveats:     nil,
	},
	"cloud-hypervisor": {
		Category:    "Virtualization",
		SupportTier: TierFCOSCommunity,
		Short:       "Cloud Hypervisor — lightweight KVM-based VMM for cloud-native virtual machines",
		Long:        "Cloud Hypervisor is a minimal Virtual Machine Monitor built on Linux KVM, designed for running cloud workloads with low overhead. Ships the cloud-hypervisor binary. No default service unit is included — configure and launch VMs via Butane units or post-provisioning scripts.",
		Caveats:     []string{"Requires hardware virtualization support (KVM). FCOS must be running on a bare-metal or nested-virt-capable host."},
	},
	"glab": {
		Category:    "Developer Tools",
		SupportTier: TierFCOSCommunity,
		Short:       "glab — GitLab CLI for managing MRs, issues, pipelines, and repositories",
		Long:        "glab is the official GitLab CLI tool for interacting with GitLab repositories, merge requests, issues, CI/CD pipelines, and more from the command line. Useful on FCOS CI runner nodes or developer workstations. No daemon — purely a CLI binary.",
		Caveats:     nil,
	},
}

// FCOSLookup returns curated metadata for the named FCOS sysext extension.
// Returns the metadata and true if found; zero ExtensionMeta and false otherwise.
// Unknown extensions should be displayed with the raw GitHub release body as
// description, grouped under "Other" in the TUI.
func FCOSLookup(name string) (ExtensionMeta, bool) {
	meta, ok := fcosCatalog[name]
	return meta, ok
}
