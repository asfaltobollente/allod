# Shares Module (Samba Windows Network Shares)

The `shares` module provides high-speed SMB/CIFS file sharing across your local area network (LAN) using Samba.

## Network Discovery & Shares

* **Discovery**: Embedded `wsdd2` daemon for Windows Network Neighborhood automatic discovery and NetBIOS broadcast.
* **Public Share**: `\\<NODE-IP>\public` (mapped to `/mnt/allod-storage/shares/public`, accessible to all LAN devices).
* **Private User Shares**: `\\<NODE-IP>\<username>` (mapped to `/mnt/allod-storage/shares/<username>`, protected with Linux/Samba credentials, `0770` permissions).
