# Headless Mode — JSON Config Schema

Knuckle's headless mode (`--headless --config <file.json>`) performs a fully
automated, non-interactive Flatcar install driven by a JSON configuration file.
This document is the authoritative reference for every field in that file.

## Quick-start examples

### Minimal DHCP install

```json
{
  "channel": "stable",
  "hostname": "flatcar-node",
  "disk": "/dev/sda",
  "network": { "mode": "dhcp" },
  "users": [
    {
      "username": "core",
      "ssh_keys": ["ssh-ed25519 AAAA... you@host"]
    }
  ],
  "update_strategy": "reboot"
}
```

### Static IP with sysexts and Tailscale

```json
{
  "channel": "stable",
  "hostname": "k8s-worker-1",
  "disk": "/dev/disk/by-id/ata-Samsung_SSD_870_EVO_S6ENNX0T123456",
  "network": {
    "mode": "static",
    "interface": "enp3s0",
    "address": "192.168.1.50/24",
    "gateway": "192.168.1.1",
    "dns": ["1.1.1.1", "8.8.8.8"]
  },
  "users": [
    {
      "username": "core",
      "github_user": "yourgithubuser",
      "groups": ["sudo", "docker"]
    }
  ],
  "sysexts": ["docker", "kubernetes"],
  "tailscale": {
    "auth_key": "tskey-auth-kXXXXXXXXX-XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX",
    "mode": "connect"
  },
  "update_strategy": "reboot",
  "reboot": true
}
```

### Dry-run (CI / testing)

```json
{
  "channel": "stable",
  "hostname": "ci-test",
  "disk": "/dev/sda",
  "network": { "mode": "dhcp" },
  "users": [{ "username": "core", "ssh_keys": ["ssh-ed25519 AAAA... ci@host"] }],
  "update_strategy": "reboot",
  "dry_run": true
}
```

---

## Full field reference

### Top-level fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `channel` | string | no | `"stable"` | Flatcar release channel. One of `stable`, `beta`, `alpha`, `lts`, `edge`. |
| `version` | string | no | _(latest)_ | Pin to a specific Flatcar version, e.g. `"3510.2.8"`. Omit to use the latest for the channel. |
| `hostname` | string | yes | — | Machine hostname. Must be a valid RFC 1123 hostname label. |
| `disk` | string | yes* | — | Target disk path. Use a stable `/dev/disk/by-id/...` path in production. `*` Not required when `ignition_url` is set. |
| `network` | object | yes | — | See [Network](#network). |
| `users` | array | yes* | — | One or more user accounts. `*` Not required when `ignition_url` is set. |
| `update_strategy` | string | no | `"reboot"` | Flatcar update strategy. One of `reboot`, `off`, `etcd-lock`. |
| `arch` | string | no | `"amd64"` | CPU architecture. One of `amd64`, `arm64`. `arm64` is not available on the `lts` channel. |
| `timezone` | string | no | `"UTC"` | System timezone (IANA format, e.g. `"America/New_York"`). |
| `sysexts` | string[] | no | `[]` | List of system extension names from the bakery catalog (e.g. `["docker", "kubernetes"]`). |
| `nvidia_driver_version` | string | no | _(none)_ | NVIDIA kernel driver series. One of `570-open` (default/recommended), `550-open`, `535-open`, `460`. Omit to skip NVIDIA setup. |
| `tailscale` | object | no | — | See [Tailscale](#tailscale). Omit or leave `auth_key` blank to skip. |
| `swap` | object | no | _(enabled, 4 GiB)_ | See [Swap](#swap). Omit for the default (4 GiB enabled). |
| `ignition_url` | string | no | — | URL of an external Ignition config. When set, knuckle downloads this config instead of generating one — only `disk` is then required. Must be HTTPS. |
| `reboot` | bool | no | `false` | If `true`, reboot immediately after a successful install. |
| `dry_run` | bool | no | `false` | If `true`, simulate the install without writing to disk. Safe for CI and testing. |

---

### Network

The `network` object configures the installed system's network. Set `mode` to
`"dhcp"` for automatic configuration or `"static"` for a fixed IP.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `mode` | string | yes | `"dhcp"` or `"static"` |
| `interface` | string | static only | Network interface name, e.g. `"enp3s0"` |
| `address` | string | static only | IP address with CIDR mask, e.g. `"192.168.1.50/24"` |
| `gateway` | string | static only | Default gateway IP, e.g. `"192.168.1.1"` |
| `dns` | string[] | optional | DNS server IPs. Defaults to gateway if omitted. |

**DHCP example:**
```json
"network": { "mode": "dhcp" }
```

**Static example:**
```json
"network": {
  "mode": "static",
  "interface": "eth0",
  "address": "10.0.0.10/24",
  "gateway": "10.0.0.1",
  "dns": ["1.1.1.1", "8.8.8.8"]
}
```

---

### Users

The `users` array defines one or more accounts to create. At least one account
is required unless `ignition_url` is set.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `username` | string | yes | Login name. Must be a valid POSIX username. |
| `ssh_keys` | string[] | no* | List of SSH public key strings. |
| `github_user` | string | no* | GitHub username — knuckle fetches all public keys at install time. |
| `password` | string | no* | Pre-hashed password in crypt format (`$6$`, `$y$`, `$2b$`, `$5$`). **Not plaintext.** |
| `groups` | string[] | no | Additional groups. Defaults to `["sudo", "docker"]`. |

`*` Each user must have at least one of `ssh_keys`, `github_user`, or `password`.

**Generate a password hash:**
```sh
openssl passwd -6          # SHA-512 ($6$…)
mkpasswd --method=yescrypt # yescrypt ($y$…)
```

**Example with GitHub key fetch:**
```json
"users": [
  {
    "username": "core",
    "github_user": "octocat",
    "groups": ["sudo", "docker"]
  }
]
```

**Example with explicit SSH key:**
```json
"users": [
  {
    "username": "core",
    "ssh_keys": [
      "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA... user@host"
    ]
  }
]
```

---

### Tailscale

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `auth_key` | string | yes (if section present) | Tailscale pre-auth key (`tskey-auth-…`). Leave blank or omit the section to skip. |
| `mode` | string | no | `"connect"` (default), `"exit-node"`, or `"subnet-router"` |
| `routes` | string | subnet-router only | Comma-separated CIDRs to advertise, e.g. `"10.0.0.0/24,192.168.1.0/24"` |

---

### Swap

By default (when `swap` is omitted), knuckle creates a 4 GiB swap file at
`/var/swapfile`. To customise or disable swap, include the `swap` object.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `true` | `false` disables swap entirely. |
| `size_mb` | int | `4096` | Swap file size in MiB. `0` uses the default (4 GiB). Maximum: 32768 MiB (32 GiB). |

**Disable swap:**
```json
"swap": { "enabled": false }
```

**Custom size:**
```json
"swap": { "enabled": true, "size_mb": 8192 }
```

---

## External Ignition URL mode

When `ignition_url` is set, knuckle downloads the Ignition config from that
URL instead of generating one from `users`, `network`, `sysexts`, etc. In this
mode only `disk` is required; all other config fields (users, network, sysexts,
tailscale) are ignored.

```json
{
  "channel": "stable",
  "disk": "/dev/sda",
  "ignition_url": "https://config.example.com/node.ign",
  "update_strategy": "reboot"
}
```

The URL must be HTTPS and reachable from the installer environment.

---

## Validation rules

Knuckle validates the config before touching the disk. Key rules:

- `channel` must be one of `stable`, `beta`, `alpha`, `lts`, `edge`
- `version`, if set, must match `X.Y.Z` (all numeric components)
- `hostname` must be a valid RFC 1123 hostname label (no dots; max 63 chars)
- `disk` must start with `/dev/` and not contain `..` path traversal
- Static network: `interface`, `address` (CIDR), and `gateway` are all required
- DNS entries must be valid IP addresses
- Each user needs a valid POSIX username and at least one auth method
- `password` must be a valid crypt hash (not plaintext)
- `update_strategy` must be `reboot`, `off`, or `etcd-lock`
- `nvidia_driver_version` must be one of `570-open`, `550-open`, `535-open`, `460`
- `swap.size_mb` must be between 0 and 32768 (MiB)
- `tailscale.auth_key` must begin with `tskey-auth-`
- `ignition_url` must be an HTTPS URL

---

## CLI flags override JSON

Two CLI flags can override JSON config fields:

```sh
# Force dry-run even if the config file has "dry_run": false
knuckle --headless --config install.json --dry-run

# Override the log file path (default: /tmp/knuckle.log)
knuckle --headless --config install.json --log-file /var/log/knuckle.log
```
