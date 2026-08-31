# Tutorial: Your First Allod Node in 15 Minutes

This guide walks you through setting up a self-hosted, personal cloud node on an **Ubuntu Server** machine.

---

## Requirements

1. **Hardware**: An x86-64 mini PC or an ARM64 board (e.g., Raspberry Pi 5) with at least 8 GB RAM.
2. **Storage**: Two identical storage drives for local btrfs RAID 1 data protection.
3. **Operating System**: Clean installation of **Ubuntu Server 24.04 LTS**.

---

## Step 1: Install Allod Core

On your fresh Ubuntu Server, install the Allod repository keyring and package:

```bash
# Add official Allod repository
sudo wget -O /usr/share/keyrings/allod.gpg https://dl.allod.dev/key.gpg
echo "deb [signed-by=/usr/share/keyrings/allod.gpg] https://dl.allod.dev/apt stable main" | sudo tee /etc/apt/sources.list.d/allod.list

# Install allod-core
sudo apt-get update
sudo apt-get install -y allod-core
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
