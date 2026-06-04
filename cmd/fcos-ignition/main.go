package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/projectbluefin/knuckle/internal/iso"
)

func main() {
	sshKey := flag.String("ssh-key", "", "SSH public key for debug access on the core user")
	flag.Parse()

	data, err := iso.GenerateInstallerIgnition(iso.OSFCOS, *sshKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	os.Stdout.Write(data)
}
