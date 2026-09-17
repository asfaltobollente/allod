# Tutorial: Your First Allod Node in 15 Minutes

This guide walks you through setting up a self-hosted, personal cloud node on an **Ubuntu Server** machine.

---

## Requirements

1. **Hardware**: An x86-64 mini PC or an ARM64 board (e.g., Raspberry Pi 5) with at least 8 GB RAM.
2. **Storage**: Two identical storage drives for local btrfs RAID 1 data protection.
3. **Operating System**: Clean installation of **Ubuntu Server 24.04 LTS**.

---

## Step 1: Install Allod & Setup Privileges

Clone the repository and build the binaries with Go (>= 1.25):

```bash
git clone https://github.com/asfaltobollente/allod.git
cd allod
go build -o allod ./cmd/allod
go build -o allod-panel ./cmd/allod-panel
go build -o allod-helperd ./cmd/allod-helperd

# Install the privileged helper daemon
sudo install -m 0755 allod-helperd /usr/local/bin/allod-helperd
sudo groupadd -f allod
sudo usermod -aG allod $USER
sudo systemctl restart allod-helperd
```

---

## Step 2: Initialize Node Configuration

Generate a clean configuration for your node:

```bash
# Verify system resources and doctor diagnostics
allod doctor

# Inspect the default configuration
cat configs/config.example.yaml
```

Apply the initial configuration to generate rootless Quadlet container units:

```bash
# Apply configuration and generate Podman Quadlets
allod apply -c configs/config.example.yaml --systemd
```

---

## Step 3: Access the Web Dashboard

Start the unprivileged web dashboard:

```bash
systemctl --user enable --now allod-panel
```

Open your browser and navigate to:
```text
http://<node-ip>:8080/
```

You can now configure your modules (Immich Photos, Nextcloud, Samba shares, and Peer Backup) with instant preflight checks!
