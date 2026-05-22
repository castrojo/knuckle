// Package model defines the core data types for the knuckle installer.
// This is a leaf package with zero external dependencies.
package model

// WizardStep represents each step in the installer wizard.
type WizardStep int

const (
	StepWelcome WizardStep = iota
	StepNetwork
	StepStorage
	StepUser
	StepSysext
	StepNvidia    // conditional: only visited when nvidia-runtime sysext is selected
	StepTailscale // conditional: only visited when tailscale sysext is selected
	StepUpdate
	StepReview
	StepInstall
	StepDone
)

// String returns the human-readable name for each step.
func (s WizardStep) String() string {
	switch s {
	case StepWelcome:
		return "Welcome"
	case StepNetwork:
		return "Network"
	case StepStorage:
		return "Storage"
	case StepUser:
		return "User"
	case StepSysext:
		return "Sysext"
	case StepNvidia:
		return "GPU Setup"
	case StepTailscale:
		return "Tailscale"
	case StepUpdate:
		return "Update Strategy"
	case StepReview:
		return "Review"
	case StepInstall:
		return "Install"
	case StepDone:
		return "Done"
	default:
		return "Unknown"
	}
}

// InstallConfig is the complete installation configuration built by the wizard.
type InstallConfig struct {
	Arch                string // amd64 or arm64 (determined at ISO build time; default "amd64")
	Channel             string // stable, beta, alpha, edge
	Version             string // optional: pin to specific Flatcar version (flatcar-install -V)
	Hostname            string
	Timezone            string // e.g. "UTC", "America/New_York"
	Network             NetworkConfig
	Disk                DiskInfo
	Users               []UserConfig
	SSHKeys             []string // authorized_keys entries
	Sysexts             []SysextEntry
	UpdateStrategy      UpdateStrategy
	IgnitionURL         string // external ignition URL (mutually exclusive with local gen)
	NvidiaDriverVersion string // Flatcar NVIDIA kernel driver series, e.g. "570-open". Empty = none.
	Swap                SwapConfig
	Tailscale           TailscaleConfig
	DryRun              bool
}

// SwapConfig holds swap-file provisioning settings.
// Enabled+SizeMB=0 ⇒ auto-size via min(detectedRAM, DefaultSwapSizeMB) at install time
// (the wizard picks a sensible default for the host). Enabled=false ⇒ no swap unit.
type SwapConfig struct {
	Enabled bool
	SizeMB  int // 0 = auto. Otherwise explicit MiB (e.g. 4096 for 4 GiB).
}

// DefaultSwapSizeMB is the swap size knuckle picks when Swap.Enabled = true and
// Swap.SizeMB = 0. 4 GiB is the upstream Flatcar example and a reasonable cap
// for home-server workloads; explicit SizeMB always overrides.
const DefaultSwapSizeMB = 4096

// TailscaleConfig holds optional Tailscale provisioning for the installed system.
// Populated only when the tailscale sysext is selected. AuthKey is written to
// /etc/tailscale/tailscale.env (mode 0600) and consumed by a oneshot systemd
// unit that runs `tailscale up` at first boot.
type TailscaleConfig struct {
	// AuthKey is the Tailscale auth/preauth key (tskey-auth-…). Empty = step skipped.
	AuthKey string
	// Mode controls how the node advertises itself:
	//   "connect"      — plain client (default)
	//   "exit-node"    — advertise as exit node (--advertise-exit-node)
	//   "subnet-router" — advertise routes via --advertise-routes
	Mode string
	// Routes is the comma-separated CIDR list advertised when Mode == "subnet-router".
	Routes string
}

// Tailscale mode constants.
const (
	TailscaleModeConnect      = "connect"
	TailscaleModeExitNode     = "exit-node"
	TailscaleModeSubnetRouter = "subnet-router"
)

// UpdateStrategy holds OS update and reboot settings for Flatcar.
type UpdateStrategy struct {
	RebootStrategy string // "reboot", "off", "etcd-lock"
	RebootWindow   string // optional: "Mon-Fri 04:00-05:00" format
	LocksmithGroup string // optional: used with etcd-lock
}

// NetworkConfig holds network settings.
type NetworkConfig struct {
	Mode      NetworkMode // dhcp or static
	Interface string
	Address   string // CIDR notation for static
	Gateway   string
	DNS       []string
}

// NetworkMode is either DHCP or Static.
type NetworkMode int

const (
	NetworkDHCP NetworkMode = iota
	NetworkStatic
)

// String returns the string representation of the network mode.
func (m NetworkMode) String() string {
	switch m {
	case NetworkDHCP:
		return "dhcp"
	case NetworkStatic:
		return "static"
	default:
		return "unknown"
	}
}

// DiskInfo represents a discovered block device.
type DiskInfo struct {
	Path       string // /dev/disk/by-id/... preferred
	DevPath    string // /dev/sda fallback
	Model      string
	Serial     string
	Size       uint64 // bytes
	SizeHuman  string // "500 GB"
	Transport  string // sata, nvme, usb, virtio
	Removable  bool
	Partitions []PartitionInfo
}

// PartitionInfo represents a partition on a disk.
type PartitionInfo struct {
	Path       string
	Label      string
	FSType     string
	Size       uint64
	MountPoint string
}

// UserConfig holds user account settings.
type UserConfig struct {
	Username     string
	SSHKeys      []string
	PasswordHash string // empty = no password
	Groups       []string
}

// NvidiaDriverSeries describes an available NVIDIA kernel driver series for Flatcar.
// These are official Flatcar-built sysexts (kernel modules signed per Flatcar kernel
// release), separate from the nvidia-runtime bakery sysext (Container Toolkit).
// Activated via /etc/flatcar/enabled-sysext.conf at first boot.
// See: https://www.flatcar.org/docs/latest/setup/customization/using-nvidia/
type NvidiaDriverSeries struct {
	// ID is the series identifier appended to "nvidia-drivers-" in enabled-sysext.conf.
	// e.g. ID "570-open" → "nvidia-drivers-570-open".
	ID          string
	Label       string // short label for the list row
	Description string // one-sentence GPU compatibility note shown on the GPU Setup screen
	Recommended bool
}

// NvidiaDriverOptions is the ordered list of available NVIDIA kernel driver series.
// Latest / recommended is first. Update when Flatcar adds or drops a driver series.
var NvidiaDriverOptions = []NvidiaDriverSeries{
	{
		ID:          "570-open",
		Label:       "570  open-source  (latest)",
		Description: "RTX 20xx / 30xx / 40xx, A-series, H-series, and newer. Recommended for all modern NVIDIA GPUs.",
		Recommended: true,
	},
	{
		ID:          "550-open",
		Label:       "550  open-source  (LTS)",
		Description: "Long-term support branch. Same GPU support as 570-open. Prefer when stability over recency matters.",
	},
	{
		ID:          "535-open",
		Label:       "535  open-source  (older LTS)",
		Description: "Older LTS branch. Supports RTX 20xx and newer. Use if 570/550 have issues on your hardware.",
	},
	{
		ID:          "460",
		Label:       "460  proprietary  (legacy GPUs)",
		Description: "Required for GTX 9xx/10xx, GTX 600/700, Quadro/Tesla Kepler/Maxwell/Pascal. Not open-source.",
	},
}

// DefaultNvidiaDriverSeries is the driver series selected when auto-detecting an NVIDIA GPU.
const DefaultNvidiaDriverSeries = "570-open"

type SysextEntry struct {
	Name        string
	Description string
	Version     string
	URL         string
	Sha256      string `json:",omitempty"` // SHA256 hash for Ignition download verification; empty = unverified
	Selected    bool
	// Category and SupportTier are curated display metadata populated by the bakery
	// package at fetch time. They are not user-supplied and are excluded from JSON
	// serialization (they are re-derived from the catalog on every fetch).
	Category    string `json:"-"` // functional group, e.g. "Container Runtime"
	SupportTier string `json:"-"` // one of the Tier* constants in internal/bakery
}

// NetworkInterface represents a discovered network interface.
type NetworkInterface struct {
	Name      string
	MAC       string
	State     string // up, down
	Speed     string
	Driver    string
	IPv4Addrs []string
	IPv6Addrs []string
}
