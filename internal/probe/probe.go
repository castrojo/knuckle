package probe

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/castrojo/knuckle/internal/model"
	"github.com/castrojo/knuckle/internal/runner"
)

// Prober is the interface for system hardware discovery
type Prober interface {
	ListDisks(ctx context.Context) ([]model.DiskInfo, error)
	ListNetworkInterfaces(ctx context.Context) ([]model.NetworkInterface, error)
}

// SystemProber uses real system commands via the runner
type SystemProber struct {
	Runner runner.Runner
}

func NewSystemProber(r runner.Runner) *SystemProber {
	return &SystemProber{Runner: r}
}

// lsblkOutput matches the JSON output of `lsblk --json --bytes --output NAME,PATH,MODEL,SERIAL,SIZE,TRAN,RM,TYPE,FSTYPE,LABEL,MOUNTPOINT`
type lsblkOutput struct {
	Blockdevices []lsblkDevice `json:"blockdevices"`
}

type lsblkDevice struct {
	Name       string        `json:"name"`
	Path       string        `json:"path"`
	Model      *string       `json:"model"`
	Serial     *string       `json:"serial"`
	Size       json.Number   `json:"size"`
	Tran       *string       `json:"tran"`
	RM         bool          `json:"rm"`
	Type       string        `json:"type"`
	FSType     *string       `json:"fstype"`
	Label      *string       `json:"label"`
	MountPoint *string       `json:"mountpoint"`
	Children   []lsblkDevice `json:"children,omitempty"`
}

func (p *SystemProber) ListDisks(ctx context.Context) ([]model.DiskInfo, error) {
	result, err := p.Runner.Run(ctx, "lsblk", "--json", "--bytes", "--output", "NAME,PATH,MODEL,SERIAL,SIZE,TRAN,RM,TYPE,FSTYPE,LABEL,MOUNTPOINT")
	if err != nil {
		return nil, fmt.Errorf("lsblk: %w", err)
	}

	var output lsblkOutput
	if err := json.Unmarshal([]byte(result.Stdout), &output); err != nil {
		return nil, fmt.Errorf("parsing lsblk output: %w", err)
	}

	var disks []model.DiskInfo
	for _, dev := range output.Blockdevices {
		if dev.Type != "disk" {
			continue
		}

		size, _ := dev.Size.Int64()
		disk := model.DiskInfo{
			DevPath:   dev.Path,
			Path:      dev.Path,
			Model:     deref(dev.Model),
			Serial:    deref(dev.Serial),
			Size:      uint64(size),
			SizeHuman: humanSize(uint64(size)),
			Transport: deref(dev.Tran),
			Removable: dev.RM,
		}

		// Parse partitions from children
		for _, child := range dev.Children {
			if child.Type == "part" {
				childSize, _ := child.Size.Int64()
				disk.Partitions = append(disk.Partitions, model.PartitionInfo{
					Path:       child.Path,
					Label:      deref(child.Label),
					FSType:     deref(child.FSType),
					Size:       uint64(childSize),
					MountPoint: deref(child.MountPoint),
				})
			}
		}

		disks = append(disks, disk)
	}

	return disks, nil
}

func (p *SystemProber) ListNetworkInterfaces(ctx context.Context) ([]model.NetworkInterface, error) {
	result, err := p.Runner.Run(ctx, "ip", "-j", "addr", "show")
	if err != nil {
		return nil, fmt.Errorf("ip addr: %w", err)
	}

	var ipOutput []ipAddrEntry
	if err := json.Unmarshal([]byte(result.Stdout), &ipOutput); err != nil {
		return nil, fmt.Errorf("parsing ip output: %w", err)
	}

	var ifaces []model.NetworkInterface
	for _, entry := range ipOutput {
		// Skip loopback
		if entry.IfName == "lo" {
			continue
		}

		iface := model.NetworkInterface{
			Name:   entry.IfName,
			State:  entry.OperState,
			Driver: entry.LinkType,
		}

		// Extract MAC address
		if entry.Address != "" {
			iface.MAC = entry.Address
		}

		// Extract IP addresses
		for _, addr := range entry.AddrInfo {
			switch addr.Family {
			case "inet":
				iface.IPv4Addrs = append(iface.IPv4Addrs, fmt.Sprintf("%s/%d", addr.Local, addr.PrefixLen))
			case "inet6":
				iface.IPv6Addrs = append(iface.IPv6Addrs, fmt.Sprintf("%s/%d", addr.Local, addr.PrefixLen))
			}
		}

		ifaces = append(ifaces, iface)
	}

	return ifaces, nil
}

type ipAddrEntry struct {
	IfName    string       `json:"ifname"`
	Address   string       `json:"address"`
	OperState string       `json:"operstate"`
	LinkType  string       `json:"link_type"`
	AddrInfo  []ipAddrInfo `json:"addr_info"`
}

type ipAddrInfo struct {
	Family    string `json:"family"`
	Local     string `json:"local"`
	PrefixLen int    `json:"prefixlen"`
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func humanSize(bytes uint64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)
	switch {
	case bytes >= TB:
		return fmt.Sprintf("%.1f TB", float64(bytes)/float64(TB))
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(MB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
