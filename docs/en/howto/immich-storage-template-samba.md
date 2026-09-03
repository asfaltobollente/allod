# Guide: Configuring Immich Storage Template for Samba LAN Sharing

This guide details how to configure Immich's **Storage Template** feature to organize smartphone photo backups into a clean, human-readable directory hierarchy:
```text
/library/<username>/<year>/<photo.ext>
```
making photos directly browsable over local network file shares (`\\allod\shares\photos`).

---

## 🎯 Why Enable Storage Template?

1. **Clean Browsing from Windows / macOS**:
   * Without Storage Template, Immich stores assets in the raw upload directory under random alphanumeric hashes.
   * With Storage Template enabled, Immich moves files into `/library` structured by user and year.
2. **Zero Internal Clutter**:
   * Allod's SMB bind mount (`\\allod\shares\photos`) targets `/library`.
   * You will not see thousands of AI thumbnail webp files (`thumbs/`) or video transcode scratchpads (`encoded-video/`).
3. **True Data Sovereignty & Zero Lock-in**:
   * Your files remain physically organized on disk by year and original name, viewable with any standard image viewer without requiring Immich.

---

## ⚙️ Step-by-Step Configuration

### 1. Open the Immich Web UI
Navigate to your Allod server in any browser:
👉 **`http://<SERVER-IP>:2283`**

### 2. Navigate to Administration Settings
1. Click the gear icon **Administration** (top right or navigation drawer).
2. Select **Settings** ➔ **Storage Template** in the left menu.

### 3. Enable & Define the Template Pattern
1. Toggle **`Enabled`** ON.
2. In the **TEMPLATE** input field, enter the desired compact structure:
   ```text
   {{user.name}}/{{y}}/{{filename}}
   ```
   *(Resulting path: `library/john/2026/beach_01.png`)*
3. Click **Save** in the bottom right corner.

### 4. Run the Migration Job for Existing Photos
1. Navigate to **Administration** ➔ **Jobs**.
2. Locate the **Storage Template Migration** job row.
3. Click **▶ Run**.
4. Immich will process all uploaded assets and move them into the `/library/<user>/<year>/` hierarchy.

---

## 🖥️ Accessing Your Photos from Windows Explorer

1. Press **`Win + R`** and enter:
   ```text
   \\<SERVER-IP>\shares\photos
   ```
2. You will see:
   ```text
   \\<SERVER-IP>\shares\photos\
      └── John/
           └── 2026/
                ├── IMG_001.png
                └── DSC_0042.jpg
   ```
3. All future photos backed up from your mobile device will automatically sort into the current year directory!
