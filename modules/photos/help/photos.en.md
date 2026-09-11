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

## Samba LAN Sharing & User Privacy

Allod allows routing photos uploaded via Immich directly into each user's private Samba share, guaranteeing complete isolation:

* **Public Media (Movies / Music)**: Resides in `\\<SERVER-IP>\public` (open to all LAN clients and indexed by Jellyfin).
* **Personal Photos**: Resides in `\\<SERVER-IP>\<username>\photos`, strictly accessible only by that authenticated user (and hidden from Jellyfin).

### How to route Immich photos into personal SMB shares:

1. **Configure Storage Label in Immich**:
   * Open Immich at `http://<SERVER-IP>:2283` with an administrator account.
   * Go to **Administration ➔ Users**, click the three dots next to the user and select **Edit**.
   * In the **Storage Label** field, enter their exact SMB username (e.g., `mario`, `chiara`).
2. **Enable Storage Template**:
   * Under **Administration ➔ Settings ➔ Storage Template**, check **Enable storage template**.
   * Configure the template pattern (e.g., `{{y}}/{{MM}}/{{filename}}`).
   * Under **Administration ➔ Jobs**, click **Run** next to **Storage Template Migration** (to relocate any previously uploaded photos).
3. **Enable the User in Allod**:
   * In the Allod Web Panel under **Shares**, set the Samba password for the user.
   * Allod automatically creates the private share `\\allod\<username>` and bind-mounts their `/library/<username>` folder into `\\allod\<username>\photos`.
