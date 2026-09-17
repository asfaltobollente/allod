# Security Policy — Allod

Security and data ownership are the primary design principles of Allod. We welcome responsible security vulnerability reports from researchers and community members.

## Supported Versions

| Version | Supported          |
| :---    | :---               |
| 2.x     | :white_check_mark: |
| < 2.0   | :x:                |

## Privilege Boundary Architecture

Allod is designed with strict privilege separation:
1. **Unprivileged Web Panel (`allod-panel`)**: Runs entirely as a rootless systemd user service with zero direct block-device access.
2. **Root Helper Daemon (`allod-helperd`)**: Accepts only a closed action whitelist (see [Root Helper Socket API](docs/en/reference/helper-api.md)) over a local UNIX socket (`/run/allod/helper.sock`).
3. **Immutable Federated Backups**: Remotely accepted backups run in append-only mode (`rest-server --append-only`), ensuring a compromised node cannot delete existing historical backups from peer nodes. *Note (Known Limitation)*: Currently, `rest-server` runs with `--append-only --no-auth`; client orchestration, per-peer `htpasswd` credentials, scheduling, and restore verification are under active development.

## Reporting a Vulnerability

If you discover a security vulnerability in Allod, please **do not open a public GitHub issue**.

Please report vulnerabilities privately via:
* **GitHub Security Advisories**: Navigate to the repository's **Security** tab and click **Report a vulnerability** to submit a confidential report.

### Response Timeline
* **Initial Acknowledgement**: Best effort by the maintainer (typically within 7 days).
* **Fix & Coordinated Disclosure**: Once a remediation is verified on `main`.

## Scope & Exclusions
* Denial of service attacks against personal nodes behind rate limits are out of scope unless they bypass the root privilege boundary.
* Physical access attacks against unencrypted local hardware are out of scope.
