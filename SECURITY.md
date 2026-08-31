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
2. **Root Helper Daemon (`allod-helperd`)**: Accepts only a closed, strictly validated list of 9 actions over a local UNIX socket (`/run/allod/helper.sock`).
3. **Immutable Federated Backups**: Remotely accepted backups run in append-only mode (`rest-server --append-only`), ensuring a compromised node cannot delete existing historical backups from peer nodes.

## Reporting a Vulnerability

If you discover a security vulnerability in Allod, please **do not open a public GitHub issue**.

Please report vulnerabilities privately via:
* **Email**: `security@allod.dev` (or open a GitHub Security Advisory)
* **GPG Key**: Available on `https://dl.allod.dev/security.gpg`

### Response Timeline
* **Initial Acknowledgement**: Within 48 hours.
* **Triage & Reproduction**: Within 5 business days.
* **Fix & Coordinated Disclosure**: As soon as a patch is verified and backported to active branches.

## Scope & Exclusions
* Denial of service attacks against personal nodes behind rate limits are out of scope unless they bypass the root privilege boundary.
* Physical access attacks against unencrypted local hardware are out of scope.
