---
description: "UI & Design System guidelines for Allod Web Panel"
globs: "cmd/allod-panel/web/**"
---

# Allod UI Design Guidelines

Whenever creating or modifying files in `cmd/allod-panel/web/` (HTML, CSS, JavaScript):

1. **Consult DESIGN.md**: Always check [`DESIGN.md`](../../DESIGN.md) at the repository root for the official CSS design tokens, spacing grid, and typography standards.
2. **No Arbitrary Styles**: Never introduce hardcoded random hex colors or arbitrary pixel margins inline. Use CSS variables defined in `style.css` (`--background`, `--card`, `--primary`, `--border-subtle`, `--radius-*`, `--space-*`).
3. **The 6 States**: Ensure any new card, section, or modal handles: Idle, Loading, Empty, Error, Partial, and Success.
4. **Intent & Safety**: Guard destructive actions (wiping storage, deleting nodes/keys) with two-step or typed confirmations.
5. **Fast Feedback**: Provide sub-100ms visual response to user clicks, leveraging `showToast(msg, type)` for toast notifications.
6. **No External CDN**: Never load external CDNs or Google Fonts. All resources must remain 100% self-hosted and air-gapped.
