# How-to: Replace a Failed Disk in btrfs RAID 1

Allod uses **btrfs RAID 1** for local storage redundancy. Every block of data is stored on two separate physical drives. If one drive fails, the system continues running in degraded mode without data loss.

---

## 1. Identify the Failed Drive

Check SMART health diagnostics via the CLI:

```bash
allod doctor
```

Inspect the btrfs filesystem state:

```bash
sudo btrfs filesystem show /data
```

You will see one device listed as `missing` or with I/O errors.

---

## 2. Replacing the Failed Disk

1. Shut down the system and physically replace the defective drive with a new drive of equal or greater capacity.
2. Boot the server and identify the serial number of the new drive:
   ```bash
   ls -la /dev/disk/by-id/
   ```
3. Initialize and replace the drive in the btrfs pool:
   ```bash
   # Replace missing device with the new physical drive
   sudo btrfs replace start <devid_or_missing> /dev/disk/by-id/<new-drive-id> /data
   ```
4. Monitor the rebuild progress:
   ```bash
   sudo btrfs replace status /data
   ```

Once the rebuild completes, run `allod doctor` to confirm that all storage health checks are green.
