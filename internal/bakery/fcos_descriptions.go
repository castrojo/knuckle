package bakery

// FCOS support tier constants for fedora-sysexts/community extensions.
const (
	// FCOSTierCommunity marks extensions maintained by the fedora-sysexts community.
	FCOSTierCommunity = "Community"
)

// fcosCatalog is the static curated catalog of common fedora-sysexts/community
// extensions. When the catalog contains extensions not present here, FCOSLookup
// returns ok=false and the TUI shows the raw GitHub release body as description.
var fcosCatalog = map[string]ExtensionMeta{
	"docker-ce": {
		Category:    "Container Runtime",
		SupportTier: FCOSTierCommunity,
		Short:       "Docker CE — full Docker engine for FCOS (daemon, CLI, containerd, runc)",
		Long:        "Ships the complete Docker Community Edition engine from official upstream packages. Includes the daemon, CLI, containerd, and runc. Packaged as a Fedora sysext for FCOS nodes.",
	},
	"tailscale": {
		Category:    "Networking",
		SupportTier: FCOSTierCommunity,
		Short:       "Tailscale — zero-config WireGuard mesh VPN for secure node-to-node networking",
		Long:        "Tailscale creates an encrypted WireGuard mesh network with no manual key exchange or firewall rules. Ships tailscale and tailscaled binaries. Authenticate nodes with tailscale up after provisioning.",
	},
	"vscode": {
		Category:    "Development",
		SupportTier: FCOSTierCommunity,
		Short:       "Visual Studio Code — Microsoft's open-source code editor",
		Long:        "Visual Studio Code packaged as a Fedora sysext for FCOS. Provides the full VS Code editor with extension marketplace support.",
	},
	"vscodium": {
		Category:    "Development",
		SupportTier: FCOSTierCommunity,
		Short:       "VSCodium — VS Code without Microsoft telemetry and branding",
		Long:        "VSCodium is a community-driven build of VS Code with Microsoft telemetry and branding removed. Uses the Open VSX registry instead of the Microsoft marketplace.",
	},
	"cilium-cli": {
		Category:    "Networking",
		SupportTier: FCOSTierCommunity,
		Short:       "Cilium CLI — eBPF-based Kubernetes networking, security, and observability",
		Long:        "The Cilium CLI for managing eBPF-powered Kubernetes networking, security policy enforcement, and observability. Use with a Kubernetes cluster running Cilium.",
	},
	"1password-cli": {
		Category:    "Security",
		SupportTier: FCOSTierCommunity,
		Short:       "1Password CLI — access passwords, secrets, and credentials from the terminal",
		Long:        "The 1Password command-line tool for accessing vault items, injecting secrets into scripts, and managing credentials without a GUI.",
	},
	"google-chrome": {
		Category:    "Browser",
		SupportTier: FCOSTierCommunity,
		Short:       "Google Chrome — Google's web browser with auto-update and sync",
		Long:        "Google Chrome browser packaged as a Fedora sysext. Includes the full browser with sync, extensions, and DevTools.",
	},
	"microsoft-edge": {
		Category:    "Browser",
		SupportTier: FCOSTierCommunity,
		Short:       "Microsoft Edge — Chromium-based browser with Microsoft integration",
		Long:        "Microsoft Edge browser packaged as a Fedora sysext. Chromium-based with Microsoft account sync and enterprise management features.",
	},
	"netbird": {
		Category:    "Networking",
		SupportTier: FCOSTierCommunity,
		Short:       "NetBird — open-source WireGuard VPN with SSO and access control",
		Long:        "NetBird creates a WireGuard-based overlay network with single sign-on, access control lists, and a management dashboard. No port forwarding required.",
	},
	"netbird-ui": {
		Category:    "Networking",
		SupportTier: FCOSTierCommunity,
		Short:       "NetBird UI — system tray GUI for the NetBird VPN client",
		Long:        "The graphical system tray application for managing NetBird VPN connections. Requires the netbird sysext for the underlying daemon.",
		Caveats:     []string{"Requires the netbird sysext for the underlying VPN daemon."},
	},
	"glab": {
		Category:    "Development",
		SupportTier: FCOSTierCommunity,
		Short:       "GLab — GitLab CLI for issues, merge requests, and CI/CD pipelines",
		Long:        "The official GitLab CLI tool for managing repositories, issues, merge requests, and CI/CD pipelines from the terminal.",
	},
	"bitwarden": {
		Category:    "Security",
		SupportTier: FCOSTierCommunity,
		Short:       "Bitwarden — open-source password manager desktop application",
		Long:        "The Bitwarden desktop application for managing passwords, secure notes, and identity information. Syncs with the Bitwarden cloud or a self-hosted server.",
	},
	"nordvpn": {
		Category:    "Networking",
		SupportTier: FCOSTierCommunity,
		Short:       "NordVPN — commercial VPN client daemon",
		Long:        "NordVPN client daemon for connecting to the NordVPN service. Provides the CLI tool for login, server selection, and connection management.",
	},
	"nordvpn-gui": {
		Category:    "Networking",
		SupportTier: FCOSTierCommunity,
		Short:       "NordVPN GUI — graphical interface for the NordVPN client",
		Long:        "The graphical desktop application for NordVPN. Requires the nordvpn sysext for the underlying daemon.",
		Caveats:     []string{"Requires the nordvpn sysext for the underlying VPN daemon."},
	},
	"virtctl": {
		Category:    "Virtualization",
		SupportTier: FCOSTierCommunity,
		Short:       "virtctl — KubeVirt CLI for managing virtual machines on Kubernetes",
		Long:        "The KubeVirt command-line tool for creating, starting, stopping, and connecting to virtual machines running on Kubernetes clusters.",
	},
	"cloud-hypervisor": {
		Category:    "Virtualization",
		SupportTier: FCOSTierCommunity,
		Short:       "Cloud Hypervisor — lightweight, Rust-based VMM for modern cloud workloads",
		Long:        "Cloud Hypervisor is a Virtual Machine Monitor built in Rust, focused on running modern cloud workloads with minimal overhead. Supports virtio devices, vhost-user, and live migration.",
	},
	"openconnect": {
		Category:    "Networking",
		SupportTier: FCOSTierCommunity,
		Short:       "OpenConnect — open-source VPN client for Cisco AnyConnect and compatible servers",
		Long:        "OpenConnect is a VPN client supporting Cisco AnyConnect, Juniper/Pulse Secure, GlobalProtect, and other SSL VPN protocols.",
	},
	"1password-gui": {
		Category:    "Security",
		SupportTier: FCOSTierCommunity,
		Short:       "1Password — desktop password manager with browser integration",
		Long:        "The 1Password desktop application for accessing passwords, secure notes, and identity information. Integrates with browsers via the 1Password extension.",
	},
	"littlesnitch": {
		Category:    "Security",
		SupportTier: FCOSTierCommunity,
		Short:       "Little Snitch — network monitor and firewall with per-app rules",
		Long:        "Little Snitch monitors outgoing network connections and lets you allow or deny them on a per-application basis. Useful for auditing which processes communicate over the network.",
	},
	"twingate": {
		Category:    "Networking",
		SupportTier: FCOSTierCommunity,
		Short:       "Twingate — zero-trust network access replacing traditional VPNs",
		Long:        "Twingate provides zero-trust network access, replacing traditional VPNs with per-resource access controls. Connects to the Twingate cloud management console.",
	},
	"nvidia-driver-cuda": {
		Category:    "GPU / Accelerators",
		SupportTier: FCOSTierCommunity,
		Short:       "NVIDIA CUDA driver — GPU compute driver for FCOS",
		Long:        "NVIDIA CUDA driver packaged as a Fedora sysext. Provides GPU compute capabilities for workloads requiring CUDA acceleration.",
	},
	"openh264": {
		Category:    "Multimedia",
		SupportTier: FCOSTierCommunity,
		Short:       "OpenH264 — Cisco's open-source H.264 codec library",
		Long:        "OpenH264 provides H.264 video encoding and decoding. Used by Firefox and other applications for WebRTC video support.",
	},
}

// FCOSLookup returns curated metadata for a fedora-sysexts/community extension.
// Returns the metadata and true if found; zero ExtensionMeta and false if not in catalog.
func FCOSLookup(name string) (ExtensionMeta, bool) {
	meta, ok := fcosCatalog[name]
	return meta, ok
}
