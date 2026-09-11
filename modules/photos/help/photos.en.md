# Photos Module (Immich)

Immich is Allod's high-performance sovereign solution for backing up, managing, and viewing photos and videos from smartphones and desktop clients.

---

## Available Resource Levels

* **`standard` (1.5 GB allocated RAM)**:
  * Automated background backup from official iOS & Android mobile apps.
  * Chronological timeline, interactive GPS map, and multi-user shared albums.
  * Recommended for systems with 8 GB of total system RAM.
* **`full` (4.0 GB allocated RAM)**:
  * Adds facial recognition (Face Clustering) and advanced AI semantic search (CLIP).
  * Requires AVX2 CPU instructions and at least 16 GB of system RAM.

---

## Sovereign Triad Integration (Samba + Immich + Jellyfin)

Allod implements the **Triad** model to deliver shared family entertainment while strictly preserving individual privacy:

* **Public Network Media**:
  * Resides in `\\<SERVER-IP>\public` (accessible to LAN devices without individual logins or via a shared guest account).
  * Automatically scanned and indexed by **Jellyfin** for in-home movie, TV show, and music streaming.
* **Personal & Private Storage**:
  * Resides in the password-protected private share `\\<SERVER-IP>\<username>`.
  * Contains isolated subfolders: `photos\`, `media\`, and `documents\`.
  * The `photos\` folder is transparently linked via *bind mount* to the user's Immich library, remaining completely invisible to Jellyfin and to other LAN users.

---

## Security Model & Data Privacy at Rest

### Why does Immich not store files encrypted at rest?
High-performance photo and media servers require direct access to image and video streams to:
1. Instantly generate compressed WebP thumbnails and mobile preview assets;
2. Execute facial embedding vectors and semantic machine learning pipelines;
3. Perform real-time hardware-accelerated transcoding to H.264/HEVC.

Encrypting individual raw files on disk would prevent AI processing pipelines and real-time streaming from functioning.

### How Allod guarantees maximum photo security:
1. **Strict 0770 Filesystem Permissions (`chmod -R go-rwx`)**:
   * Both the source folder `/mnt/allod-storage/photos/upload/library/<username>` and the SMB mount target are enforced with `0770` permissions.
   * Non-privileged Linux OS users and unauthorized daemons cannot traverse or read files on the host filesystem. Only `root` and the isolated Podman container process have access.
2. **Selective Bind-Mount (Principle of Least Privilege)**:
   * Only the specific folder `library/<username>` is mounted to Samba.
   * PostgreSQL databases, thumbnail caches, internal configuration files, and other users' libraries are never exposed over SMB.
3. **App-Exclusive Isolation Mode (Optional)**:
   * When provisioning a user (or dynamically via the *Unlink from SMB* button in the dashboard), administrators can disable SMB network exposure.
   * When unlinked, photos remain strictly confined to Immich's isolated container volume, accessible only via the official Immich app secured with passwords and 2FA (two-factor authentication).

---

## Configuration Guide: Immich Storage Label & Template

To ensure photos uploaded from smartphones route directly into the matching user's named Samba folder, follow these steps:

### Step 1: Set User Storage Label
1. Open Immich at `http://<SERVER-IP>:2283` (or click **Immich Photos** in the Allod Launchpad) using an Administrator account.
2. Click the gear icon (top right) ➔ **Administration** ➔ **Users** tab.
3. Click the edit icon (or the three dots) next to the target user (e.g. `mario`).
4. In the **Storage Label** field, enter the exact Allod/Samba username (e.g. `mario`).
5. Click **Save**. From this point forward, all uploads by this user will be saved to `photos/upload/library/mario/`.

### Step 2: Enable & Configure Storage Template
1. In **Administration**, open the **Settings** tab ➔ **Storage Template**.
2. Check **Enable storage template**.
3. Set the recommended template pattern:
   ```text
   {{y}}/{{MM}}/{{filename}}
   ```
   *(This sorts media chronologically by Year and Month: e.g., `2026/09/IMG_1234.jpg`).*
4. Click **Save** at the bottom right.

### Step 3: Run Storage Template Migration Job
1. Open the **Administration** ➔ **Jobs** tab.
2. Locate the row named **Storage Template Migration**.
3. Click **Run**. Immich will automatically relocate and organize existing photos into the structured `library/<username>` folder.

---

## Automated Provisioning with Allod's "Create Triad User"

The Allod Web Panel automates the entire orchestration:

1. Click **🛡️ Create Triad User** in the top bar or Launchpad.
2. The orchestrator checks the live status of **Samba**, **Immich**, and **Jellyfin**; if any service is down, user creation is halted with an explanatory alert.
3. Provide the **Username** (e.g. `mario`) and **Password**.
4. Check or uncheck **Link Immich photos library to private Samba share**.
5. Click **Create User & Orchestrate Triad**:
   * Creates the Linux system account with safe `/usr/sbin/nologin` shell.
   * Sets the Samba password and provisions `\\allod\mario`.
   * Enforces restrictive `0770` permissions on private subdirectories.
   * If enabled, executes the bind-mount of the Immich library into `\\allod\mario\photos`.
   * The user table in the modal allows toggling the SMB photo mount on and off at any time with one click!

