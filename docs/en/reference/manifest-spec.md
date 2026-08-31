# Reference: Module Manifest Specification (`module.yaml`)

Every service in Allod is declared through a declarative `module.yaml` file conforming to `schemas/module.schema.json`.

---

## Schema Overview

```yaml
id: photos               # Unique module identifier (^[a-z][a-z0-9-]{2,20}$)
tier: recommended        # core | recommended | optional
provides: [photo-backup] # List of features provided
conflicts: [photoprism]  # Mutually exclusive module IDs
platforms: [amd64, arm64] # Supported architectures

levels:
  off:
    ram_mb: 0
  standard:
    ram_mb: 1500
    requires:
      modules: [storage]
    grants: [Galleria web, backup da iOS e Android, ricerca per data]
  full:
    ram_mb: 4000
    requires:
      modules: [storage]
      cpu_flags: [sse4_2]
      total_ram_mb: 8192
    grants: [Riconoscimento volti, ricerca semantica ML, transcodifica video]

ports:
  - n: 2283
    scope: mesh          # loopback | lan | mesh
    share: member        # none | friend | member

privileges:
  userns: rootless       # rootless | host
  devices: []            # Host device paths (e.g., /dev/dri/renderD128)
  caps: []               # Linux capabilities

images:
  - ref: ghcr.io/immich-app/immich-server
    tag: v1.118.0
    channel: patch       # pinned | patch | security
    args: []             # Container arguments

update:
  requires_release_notes: false
  breaks_mobile_apps: true

transitions:
  off -> standard: safe
  standard -> full: safe
  full -> standard: safe
  standard -> off: destructive

datasets:
  - id: photos
    includes: [data, db]
    excludes: [cache, thumbs]

help:
  en: help/photos.en.md
  it: help/photos.it.md
```
