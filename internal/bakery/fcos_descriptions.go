package bakery

// Support tier constants for FCOS community sysexts.
const (
	// TierFCOSCommunity marks extensions published by the fedora-sysexts community project.
	// These are community-maintained builds not directly associated with the Fedora or FCOS
	// upstream teams.
	TierFCOSCommunity = "FCOS Community"
)

// fcosCatalog is the curated metadata catalog for fedora-sysexts/community extensions.
// When the community repo ships an extension not listed here, LookupFCOS returns ok=false
// and the entry is displayed under "Community" with the raw GitHub release body.
var fcosCatalog = map[string]ExtensionMeta{
	"docker-ce": {
		Category:    "Container Runtime",
		SupportTier: TierFCOSCommunity,
		Short:       "Docker CE — community edition container engine for FCOS",
		Long:        "Docker CE provides the full Docker container engine (daemon, CLI, containerd, runc) built for Fedora CoreOS. Enables running and managing OCI containers on FCOS nodes. Start the daemon with systemctl enable --now docker after provisioning.",
		Caveats:     nil,
	},
	"tailscale": {
		Category:    "Networking",
		SupportTier: TierFCOSCommunity,
		Short:       "Tailscale — zero-config WireGuard mesh VPN for FCOS nodes",
		Long:        "Tailscale creates an encrypted WireGuard mesh network between nodes with no manual firewall rules or key exchange required. Ships tailscale, tailscaled, and a systemd service unit. Authenticate nodes with tailscale up after provisioning.",
		Caveats:     nil,
	},
	"vscode": {
		Category:    "Developer Tools",
		SupportTier: TierFCOSCommunity,
		Short:       "Visual Studio Code — Microsoft's source code editor for FCOS",
		Long:        "Visual Studio Code is a lightweight but powerful source code editor supporting debugging, syntax highlighting, intelligent code completion, and extensions. Ships the code binary. Intended for developer workstation use-cases on FCOS, not server deployments.",
		Caveats:     []string{"Not intended for headless server deployments — requires a graphical environment or SSH X11 forwarding."},
	},
	"vscodium": {
		Category:    "Developer Tools",
		SupportTier: TierFCOSCommunity,
		Short:       "VSCodium — community build of VS Code without Microsoft telemetry",
		Long:        "VSCodium is a community-maintained binary distribution of Visual Studio Code built from the same MIT-licensed source but without Microsoft branding and telemetry. Ships the codium binary. Compatible with most VS Code extensions via the Open VSX Registry.",
		Caveats:     []string{"Not intended for headless server deployments — requires a graphical environment or SSH X11 forwarding."},
	},
	"nordvpn-gui": {
		Category:    "Networking",
		SupportTier: TierFCOSCommunity,
		Short:       "NordVPN GUI — NordVPN Linux graphical client for FCOS",
		Long:        "NordVPN GUI provides the official NordVPN Linux desktop client for connecting to NordVPN servers. Requires a NordVPN subscription. Ships the nordvpn GUI binary and supporting files.",
		Caveats:     []string{"Requires a valid NordVPN subscription.", "Not intended for headless server deployments."},
	},
	"microsoft-edge": {
		Category:    "Developer Tools",
		SupportTier: TierFCOSCommunity,
		Short:       "Microsoft Edge — Chromium-based browser from Microsoft for FCOS",
		Long:        "Microsoft Edge is Microsoft's Chromium-based web browser. Ships the microsoft-edge binary and supporting files for FCOS. Intended for developer workstation or kiosk use-cases.",
		Caveats:     []string{"Not intended for headless server deployments — requires a graphical environment."},
	},
	"dnclient": {
		Category:    "Networking",
		SupportTier: TierFCOSCommunity,
		Short:       "Defined Networking client — zero-trust overlay network for FCOS",
		Long:        "dnclient is the Defined Networking overlay VPN client providing zero-trust networking between hosts. Ships the dnclient binary and a systemd service unit. Requires a Defined Networking account and enrolment token.",
		Caveats:     []string{"Requires a Defined Networking account and enrolment token."},
	},
	"openconnect": {
		Category:    "Networking",
		SupportTier: TierFCOSCommunity,
		Short:       "OpenConnect — open-source VPN client for Cisco AnyConnect and others",
		Long:        "OpenConnect is an open-source VPN client compatible with Cisco AnyConnect, Juniper, Pulse, and GlobalProtect servers. Ships the openconnect binary for FCOS. Typically invoked manually or via a wrapper script.",
		Caveats:     nil,
	},
}

// LookupFCOS returns curated metadata for the named FCOS community extension.
// Returns the metadata and true if found; zero ExtensionMeta and false if not in catalog.
// Unknown extensions should be displayed with Category "Community" and the raw release body.
func LookupFCOS(name string) (ExtensionMeta, bool) {
	meta, ok := fcosCatalog[name]
	return meta, ok
}
