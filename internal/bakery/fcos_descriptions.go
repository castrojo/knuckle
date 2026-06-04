package bakery

// FCOS support tier constants for fedora-sysexts/community extensions.
const (
	// FCOSTierCommunity marks extensions maintained by the fedora-sysexts community.
	FCOSTierCommunity = "Community"
)

// fcosCatalog is the static curated catalog of common fedora-sysexts/community extensions.
// When an extension is not present here, FCOSLookup returns ok=false and the raw
// GitHub release body is used as description.
var fcosCatalog = map[string]ExtensionMeta{
	"docker-ce": {
		Category:    "Container Runtime",
		SupportTier: FCOSTierCommunity,
		Short:       "Docker CE — full Docker engine for Fedora CoreOS container workloads",
		Long:        "Docker Community Edition ships the Docker daemon, CLI, containerd, and runc from official upstream packages. Provides the standard Docker experience on FCOS nodes for running containers, building images, and orchestrating with Compose.",
		Caveats:     nil,
	},
	"tailscale": {
		Category:    "Networking",
		SupportTier: FCOSTierCommunity,
		Short:       "Tailscale — zero-config WireGuard mesh VPN for secure node-to-node networking",
		Long:        "Tailscale creates an encrypted WireGuard mesh network between your nodes with no manual key exchange or firewall rules required. Ships tailscale and tailscaled binaries.",
		Caveats:     nil,
	},
	"vscode": {
		Category:    "Development",
		SupportTier: FCOSTierCommunity,
		Short:       "Visual Studio Code — Microsoft's code editor as a sysext layer",
		Long:        "Ships Visual Studio Code as a system extension for Fedora CoreOS. Provides a full-featured code editor accessible on the node.",
		Caveats:     nil,
	},
	"vscodium": {
		Category:    "Development",
		SupportTier: FCOSTierCommunity,
		Short:       "VSCodium — free/libre build of VS Code without Microsoft telemetry",
		Long:        "VSCodium is an open-source build of Visual Studio Code stripped of Microsoft telemetry and branding. Ships as a sysext layer for Fedora CoreOS.",
		Caveats:     nil,
	},
	"google-chrome": {
		Category:    "Desktop",
		SupportTier: FCOSTierCommunity,
		Short:       "Google Chrome — web browser as a sysext layer for immutable desktops",
		Long:        "Ships Google Chrome as a system extension for Fedora CoreOS immutable desktop configurations.",
		Caveats:     nil,
	},
	"microsoft-edge": {
		Category:    "Desktop",
		SupportTier: FCOSTierCommunity,
		Short:       "Microsoft Edge — Chromium-based browser as a sysext layer",
		Long:        "Ships Microsoft Edge as a system extension for Fedora CoreOS immutable desktop configurations.",
		Caveats:     nil,
	},
	"1password-gui": {
		Category:    "Security",
		SupportTier: FCOSTierCommunity,
		Short:       "1Password — password manager GUI as a sysext layer",
		Long:        "Ships the 1Password desktop application as a system extension for Fedora CoreOS. Provides password and secret management with a native GUI.",
		Caveats:     nil,
	},
	"bitwarden": {
		Category:    "Security",
		SupportTier: FCOSTierCommunity,
		Short:       "Bitwarden — open-source password manager GUI",
		Long:        "Ships the Bitwarden desktop application as a system extension for Fedora CoreOS. Provides password management with support for self-hosted Vaultwarden servers.",
		Caveats:     nil,
	},
	"ghostty": {
		Category:    "Desktop",
		SupportTier: FCOSTierCommunity,
		Short:       "Ghostty — fast GPU-accelerated terminal emulator",
		Long:        "Ghostty is a GPU-accelerated terminal emulator focused on speed, native platform integration, and correct terminal emulation. Ships as a sysext layer.",
		Caveats:     nil,
	},
	"glab": {
		Category:    "Development",
		SupportTier: FCOSTierCommunity,
		Short:       "GLab — GitLab CLI for managing repos, issues, and pipelines",
		Long:        "GLab is the official GitLab CLI tool for managing repositories, merge requests, issues, and CI/CD pipelines from the terminal.",
		Caveats:     nil,
	},
	"mullvad-vpn": {
		Category:    "Networking",
		SupportTier: FCOSTierCommunity,
		Short:       "Mullvad VPN — privacy-focused VPN client with WireGuard support",
		Long:        "Mullvad VPN provides a privacy-focused VPN client with support for WireGuard and OpenVPN protocols. Ships the GUI and daemon as a sysext layer.",
		Caveats:     nil,
	},
	"incus": {
		Category:    "Virtualization",
		SupportTier: FCOSTierCommunity,
		Short:       "Incus — modern system container and VM manager (LXD successor)",
		Long:        "Incus is a system container and virtual machine manager, the community successor to LXD. Manages lightweight containers and full VMs on Fedora CoreOS nodes.",
		Caveats:     nil,
	},
	"netbird-ui": {
		Category:    "Networking",
		SupportTier: FCOSTierCommunity,
		Short:       "NetBird — open-source WireGuard-based VPN with management UI",
		Long:        "NetBird creates a WireGuard-based overlay network with a management UI. Ships the NetBird client with GUI for Fedora CoreOS.",
		Caveats:     nil,
	},
	"cloud-hypervisor": {
		Category:    "Virtualization",
		SupportTier: FCOSTierCommunity,
		Short:       "Cloud Hypervisor — lightweight KVM-based VMM for modern cloud workloads",
		Long:        "Cloud Hypervisor is a lightweight Virtual Machine Monitor built on KVM, designed for running modern cloud workloads with minimal overhead.",
		Caveats:     nil,
	},
	"dnclient": {
		Category:    "Networking",
		SupportTier: FCOSTierCommunity,
		Short:       "Defined Networking — Nebula-based overlay VPN with managed control plane",
		Long:        "Defined Networking client connects to the Defined.net managed control plane for simplified Nebula overlay network management.",
		Caveats:     nil,
	},
	"openconnect": {
		Category:    "Networking",
		SupportTier: FCOSTierCommunity,
		Short:       "OpenConnect — VPN client supporting Cisco AnyConnect, Pulse, GlobalProtect",
		Long:        "OpenConnect is a multi-protocol VPN client supporting Cisco AnyConnect, Pulse Secure, GlobalProtect, and other SSL VPN protocols.",
		Caveats:     nil,
	},
}

// FCOSLookup returns curated metadata for a named FCOS extension.
// Returns the metadata and true if found; zero ExtensionMeta and false if not in catalog.
func FCOSLookup(name string) (ExtensionMeta, bool) {
	meta, ok := fcosCatalog[name]
	return meta, ok
}
