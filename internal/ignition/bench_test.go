package ignition

import (
	"fmt"
	"testing"

	"github.com/projectbluefin/knuckle/internal/model"
)

var (
	benchmarkIgnitionJSONSink string
	benchmarkButaneYAML       = buildBenchmarkButaneYAML()
)

func BenchmarkCompileToIgnition(b *testing.B) {
	butane := benchmarkButaneYAML
	b.ReportAllocs()
	b.SetBytes(int64(len(butane)))
	for i := 0; i < b.N; i++ {
		ignitionJSON, err := CompileToIgnition(butane)
		if err != nil {
			b.Fatalf("CompileToIgnition failed: %v", err)
		}
		benchmarkIgnitionJSONSink = ignitionJSON
	}
}

func buildBenchmarkButaneYAML() string {
	cfg := &model.InstallConfig{
		Hostname: "benchmark-node-01",
		Channel:  "stable",
	}
	builder := NewBuilder(cfg)

	for i := 0; i < 16; i++ {
		builder.AddStorageFile(fmt.Sprintf(`- path: /etc/knuckle/config-%02d.conf
  mode: 0644
  overwrite: true
  contents:
    inline: |
      key-%02d=value-%02d
      role=worker
      channel=stable
      feature_gate_%02d=true
`, i, i, i, i))
	}

	for i := 0; i < 4; i++ {
		builder.AddStorageLink(fmt.Sprintf(`- path: /etc/systemd/system/multi-user.target.wants/benchmark-%02d.service
  target: /usr/lib/systemd/system/benchmark-%02d.service
`, i, i))
	}

	for i := 0; i < 6; i++ {
		builder.AddSystemdUnit(fmt.Sprintf(`- name: benchmark-%02d.service
  enabled: true
  contents: |
    [Unit]
    Description=Benchmark Service %02d
    After=network-online.target
    [Service]
    Type=oneshot
    ExecStart=/usr/bin/echo benchmark-%02d
    [Install]
    WantedBy=multi-user.target
`, i, i, i))
	}

	builder.SetPasswdUsers(`- name: core
  ssh_authorized_keys:
    - ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGdllynsgXbmcFXhVJAIAkDbYjqZ2OgHgZJVFmFKtvF7 benchmark@test
  groups:
    - sudo
    - docker
  shell: /bin/bash`)

	return builder.Build()
}
