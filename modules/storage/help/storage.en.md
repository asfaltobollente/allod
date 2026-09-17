# Storage Module (Btrfs Pool Orchestration)

The `storage` module manages physical hard disks, SSDs, and NVMe drives using modern Btrfs filesystem features.

## Supported Modes & Subvolumes

* **RAID 1 (Recommended)**: Mirroring across two physical drives with automatic metadata and data checksumming and self-healing.
* **Single / RAID 0**: Single drive or striped pool for non-redundant installations.
* **Mount Point**: Dedicated mount on `/mnt/allod-storage` with `compress=zstd:1` for transparent data compression.
* **Isolated Subvolumes**:
  * `/mnt/allod-storage/cloud`
  * `/mnt/allod-storage/photos`
  * `/mnt/allod-storage/shares`
  * `/mnt/allod-storage/backup`
