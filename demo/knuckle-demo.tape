# Knuckle TUI demo — reproducible recording via VHS
# https://github.com/charmbracelet/vhs
#
# Prerequisites:
#   go build -o bin/knuckle ./cmd/knuckle
#   vhs demo/knuckle-demo.tape
#
# --demo uses hardcoded mock hardware and sysexts — no VM, no network, no real disks.

Output demo/knuckle-install.gif
Output demo/knuckle-install.mp4

Set Shell "bash"
Set FontSize 14
Set Width 1200
Set Height 700
Set Theme "Dracula"
Set TypingSpeed 60ms
Set PlaybackSpeed 1.0

# Start knuckle in demo mode
Type "bin/knuckle --demo"
Enter
Sleep 1.5s

# Welcome — browse channel cards
Down
Sleep 600ms
Down
Sleep 600ms
Down
Sleep 600ms
Up
Sleep 500ms
Up
Sleep 500ms
Up
Sleep 1s

# Select stable → Network step
Enter
Sleep 1.2s

# Network — DHCP (two form groups)
Enter
Sleep 800ms
Enter
Sleep 1.2s

# Storage — pick first disk
Enter
Sleep 1.2s

# User — identity group, then auth group
Enter
Sleep 800ms
Enter
Sleep 1.2s

# Sysext — select docker, kubernetes, tailscale
Down
Sleep 400ms
Space
Sleep 500ms
Down
Sleep 400ms
Down
Sleep 400ms
Space
Sleep 500ms
Down
Sleep 400ms
Space
Sleep 600ms
Enter
Sleep 1.2s

# Update strategy — default reboot
Enter
Sleep 1.2s

# Review — pause on summary
Sleep 3s

# Quit
Ctrl+C
Sleep 1s
