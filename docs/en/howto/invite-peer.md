# How-to: Invite a Friend to your Ring

Allod achieves decentralized data safety through **federated peer backup rings**. By connecting with friends over a secure WireGuard mesh (via Headscale/Tailscale), your nodes store encrypted, append-only backup replicas for each other.

---

## 1. Prerequisites

* Two or more running Allod nodes connected to the same WireGuard/Headscale overlay network.
* Mesh IP addresses for all participating nodes (e.g. `100.64.0.1`, `100.64.0.2`, `100.64.0.3`).

---

## 2. Adding a Member to `ring.yaml`

The group topology is maintained in a shared, declarative `ring.yaml` file.

To invite a new member (*Sara*), add her node definition to `configs/ring.example.yaml`:

```yaml
name: allod-ring-alpha
target_replicas: 2

members:
  allod-luca:
    id: allod-luca
    address: 100.64.0.1
    quota_gb: 500
    datasets:
      - id: photos
        size_gb: 45
        critical: true
      - id: documents
        size_gb: 15
        critical: true

  allod-marco:
    id: allod-marco
    address: 100.64.0.2
    quota_gb: 500
    datasets:
      - id: family-media
        size_gb: 60
        critical: true

  # Newly added peer:
  allod-sara:
    id: allod-sara
    address: 100.64.0.3
    quota_gb: 500
    datasets:
      - id: design-assets
        size_gb: 30
        critical: true
```

---

## 3. Verify 2-Replica Placement

Run the ring status command to verify that all critical datasets automatically gain 2 distinct remote replicas:

```bash
allod ring status --ring configs/ring.example.yaml
```

Output:
```text
=== Gruppo Allod: allod-ring-alpha ===
Target Repliche Remote: 2 | Nodi Membri: 3

--- Assegnazione Repliche Federate ---
  • allod-luca:photos         (45 GB) [CRITICO] -> Repliche su: [allod-marco, allod-sara]      | Stato: OK (2/2 repliche)
  • allod-marco:family-media  (60 GB) [CRITICO] -> Repliche su: [allod-luca, allod-sara]       | Stato: OK (2/2 repliche)
  • allod-sara:design-assets  (30 GB) [CRITICO] -> Repliche su: [allod-marco, allod-luca]      | Stato: OK (2/2 repliche)

✓ Stato Gruppo: OTTIMALE (Ogni dataset critico possiede 2 repliche remote distinte)
```

No manual reconfiguration of Luca or Marco's node is needed!
