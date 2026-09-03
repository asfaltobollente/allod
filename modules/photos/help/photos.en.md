# Photos Module (Immich)

Immich is Allod's high-performance self-hosted photo and video backup solution.

## Available Levels

* **`standard` (1.5 GB RAM)**:
  * Automated background backup from iOS & Android mobile apps.
  * Chronological timeline, GPS map, and shared albums.
  * Recommended for systems with 8 GB of RAM.
* **`full` (4.0 GB RAM)**:
  * Adds facial recognition and semantic AI search.
  * Requires 16 GB of system RAM and AVX2 CPU.

## Samba LAN Sharing (Windows / Mac)

Allod allows exposing the photo library directly to Windows Explorer or Mac Finder at:
`\\<SERVER-IP>\shares\photos`

To organize photos cleanly into `/user/year/photo.ext`:
1. Open Immich at `http://<SERVER-IP>:2283`.
2. Navigate to **Administration ➔ Settings ➔ Storage Template**.
3. Enable the template and set: `{{user.name}}/{{y}}/{{filename}}`.
4. In **Administration ➔ Jobs**, click **Run** on **Storage Template Migration**.
