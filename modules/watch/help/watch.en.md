# Watchdog Module (Resource & Process Monitor)

The `watch` module provides continuous background monitoring of system resources, container health, and storage integrity.

## Monitoring Invariants

* **RAM Reserves**: Ensures host memory usage stays below hardware thresholds, maintaining the 2.0 GB OS reserve.
* **Storage Invariants**: Monitors Btrfs disk free space and alerts before storage depletion causes copy-on-write degradation.
* **Process Liveness**: Verifies that rootless Quadlet user services and helper daemons are active and responding.
