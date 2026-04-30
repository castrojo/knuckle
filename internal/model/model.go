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
	Channel        string        // stable, beta, alpha, edge
	Version        string        // optional: pin to specific Flatcar version (flatcar-install -V)
	Hostname       string
	Network        NetworkConfig
	Disk           DiskInfo
	Users          []UserConfig
	SSHKeys        []string // authorized_keys entries
	Sysexts        []SysextEntry
	UpdateStrategy UpdateStrategy
	IgnitionURL    string // external ignition URL (mutually exclusive with local gen)
	DryRun         bool
}

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

// SysextEntry represents a system extension from the Flatcar Bakery.
type SysextEntry struct {
	Name        string
	Description string
	Version     string
	URL         string
	Selected    bool
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
