# How-To Guide: Native Wake-on-LAN (WoL) in Allod

Allod includes a native **Wake-on-LAN (WoL)** engine implemented entirely in pure Go. It allows you to remotely power on your primary desktop workstation or any LAN-connected device with **a single click from the Web Dashboard** or with a **straightforward CLI command**, completely removing the need to SSH into your server.

---

## ⚡ Features of Allod's WoL Engine

1. **Zero External Dependencies**: No extra Debian or Ubuntu packages (such as `wakeonlan` or `etherwake`) are required. The standard AMD Magic Packet (102 bytes over UDP port 9) is crafted natively in memory.
2. **Intelligent Multi-Interface Broadcasting**:
   * On servers with encrypted private mesh interfaces (such as WireGuard or NetBird), standard global broadcast `255.255.255.255` may occasionally be routed into virtual tunnel interfaces.
   * Allod's WoL engine automatically inspects all active physical network interfaces supporting broadcast (`net.FlagBroadcast`), calculates the local subnet broadcast address (e.g. `192.168.1.255` or `192.168.0.255`), and transmits a copy through each physical adapter, ensuring the packet always reaches the physical network switch.
3. **Database Persistence in `state.db`**: Saved devices (Name, MAC address, Port, Broadcast IP) are stored in Allod's SQLite database with automatic tracking of the last power-on timestamp.
4. **Rootless Operation**: Opening and broadcasting UDP packets on port 9 requires no root privileges.

---

## 🖥️ Target Computer Configuration

For a network card to receive the Magic Packet and power on the computer, the target machine must be configured in both BIOS/UEFI and the operating system.

### 1. BIOS/UEFI Settings (Motherboard)
1. Reboot the target computer and enter the BIOS/UEFI menu (usually by tapping `DEL`, `F2`, or `F12` during boot).
2. Navigate to **Advanced**, **Power Management**, or **ACPI Configuration**:
   * **Wake on LAN / WOL**: Set to **Enabled**.
   * **Power On By PCI-E Device**: Set to **Enabled**.
   * **ErP / EuP Ready**: Set to **Disabled** (ErP shuts off standby power to the NIC to save energy, preventing it from listening for Magic Packets).
   * **Deep Sleep**: Set to **Disabled**.
3. Save changes and boot the operating system.

---

### 2. Windows Configuration (Target PC)

On Windows systems, you need to verify network adapter properties and disable Fast Startup.

#### A. Network Adapter Properties
1. Press `Win + X` and select **Device Manager**.
2. Expand **Network adapters** and double-click your physical Ethernet adapter (e.g. *Realtek Gaming GbE*, *Intel Ethernet Controller*).
3. Switch to the **Power Management** tab:
   * Check **Allow this device to wake the computer**.
   * Check **Only allow a magic packet to wake the computer**.
4. Switch to the **Advanced** tab:
   * Find **Wake on Magic Packet** and set it to **Enabled**.
   * If available, set **Shutdown Wake-On-Lan** to **Enabled**.

#### B. Disable Fast Startup (Recommended)
Windows Fast Startup places the computer into a hybrid hibernation state (S4) that frequently turns off standby power to the NIC:
1. Press `Win + R`, type `powercfg.cpl`, and press Enter.
2. Click **Choose what the power buttons do** on the left menu.
3. Click **Change settings that are currently unavailable**.
4. Uncheck **Turn on fast startup (recommended)**.
5. Click **Save changes**.

---

### 3. Linux Configuration (Target PC)

If the target machine runs Linux:

1. Check current WoL status with `ethtool`:
   ```bash
   sudo ethtool <eth-interface> | grep Wake-on
   # Example output:
   # Supports Wake-on: pumbg
   # Wake-on: d   <-- 'd' means disabled
   ```
2. Enable Magic Packet reception:
   ```bash
   sudo ethtool -s <eth-interface> wol g
   ```
3. To persist across reboots using NetworkManager:
   ```bash
   sudo nmcli connection modify "<ConnectionName>" 802-3-ethernet.wake-on-lan magic
   ```

---

## 🌐 Using WoL from the Allod Web Dashboard

1. **Launchpad (1-Click Access)**:
   * Beneath your application grid, the **⚡ Wake-on-LAN Hub** card displays all saved devices with an instant **`⚡ Power On`** button and last wake timestamp.
   * Clicking the button immediately transmits the Magic Packet and displays confirmation feedback with the reached subnet details.
2. **Dedicated Management Modal (`wol-modal`)**:
   * Click **`⚡ Wake-on-LAN`** in the Launchpad toolbar or in **Settings & Maintenance**.
   * From this modal, you can:
     * Review all saved computers, MAC addresses, and power-on history.
     * Add new machines or edit existing devices.
     * Use **Quick Wake by MAC** to wake any network adapter instantly without saving.

---

## ⌨️ Using WoL via Allod CLI

Allod provides the `allod wol` command line interface:

### 1. Power on a saved computer by name
```bash
allod wol wake "Desktop PC"
```

### 2. Wake any computer directly by MAC address
```bash
allod wol wake 00:D8:61:33:0E:1F
```
*(Accepts colon-separated `00:11:22:33:44:55`, hyphen-separated `00-11-22-33-44-55`, or raw hex `001122334455`).*

### 3. List all saved WoL devices
```bash
allod wol list
```
*Example output:*
```
ID    NAME                  MAC ADDRESS         BROADCAST           PORT    LAST POWER ON       
----------------------------------------------------------------------------------------------
1     Desktop PC            00:d8:61:33:0e:1f   255.255.255.255     9       2026-09-24 21:15:02
2     Studio Workstation    00:11:22:33:44:55   255.255.255.255     9       Never               
```

### 4. Add or update a device
```bash
allod wol add "Studio Workstation" 00:11:22:33:44:55
```

### 5. Remove a saved device
```bash
allod wol delete "Studio Workstation"
# or by database ID:
allod wol delete 2
```

---

## 🔍 Troubleshooting

* **PC does not turn on**:
  1. Inspect the Ethernet port LED on the back of the motherboard while the PC is off: it should be lit solid or blinking slowly in green or orange. If it is completely off, the NIC is not receiving standby power (check *ErP/EuP* and *Deep Sleep* in BIOS).
  2. On Windows, make sure **Fast Startup** is disabled.
  3. Ensure the network cable is connected to the onboard motherboard Ethernet port (USB network adapters rarely support WoL from complete shutdown).
  4. Ensure the entered MAC address corresponds to the wired physical NIC and not a Wi-Fi or virtual adapter.
