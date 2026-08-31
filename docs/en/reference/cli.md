# Reference: Allod CLI Commands

The `allod` command-line tool is the primary orchestrator for node configuration, Quadlet generation, ring management, and diagnostics.

---

## Command Syntax

```bash
allod [command] [flags]
```

## Global Flags
* `-c, --config <path>`: Path to configuration file (default: `configs/config.example.yaml`).
* `--state-db <path>`: Path to state database (default: `state.db`).
* `--ring <path>`: Path to ring topology configuration (default: `configs/ring.example.yaml`).

---

## Subcommands

### `allod plan`
Displays the differential execution plan comparing desired `config.yaml` against applied `state.db`.
* Output codes:
  * `[+]`: New module to be created.
  * `[~]`: Modified level or changed configuration.
  * `[=]`: Unchanged module (no restart needed).
  * `[-]`: Removed or disabled module.

### `allod apply`
Generates Quadlet unit files and records applied hashes in `state.db` (idempotent).
* Flags:
  * `--systemd`: Directly targets standard Ubuntu Quadlet directory (`~/.config/containers/systemd/` for rootless, `/etc/containers/systemd/` for root).
  * `--out-dir <path>`: Custom output folder.

### `allod set <module>=<level>`
Validates requirements, runs hardware preflight, and safely updates module level.
* Flags:
  * `--accept-risk`: Bypasses hardware/RAM limits (cannot bypass invalid level names).

### `allod doctor`
Performs comprehensive diagnostic checks on active modules, resource allocations, and manifest integrity.

### `allod install <hostname>`
Generates zero-touch `cloud-init` configuration for automated Ubuntu Server provisioning.

### `allod ring status`
Displays group topology, member quotas, and verification of 2-replica remote placement.

### `allod ring simulate --remove <member>`
Simulates member departure, identifying lost datasets, degraded replicas, and automated rebalance plans.

### `allod sbom`
Generates a complete Software Bill of Materials in standard CycloneDX JSON format for Cyber Resilience Act compliance.
