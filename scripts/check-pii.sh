#!/usr/bin/env bash
# scripts/check-pii.sh - Automated PII & Secret Sanity Check
# Verifies that no personal private IPs or unauthorized emails exist in tracked git files.
set -euo pipefail

FAILURES=0

echo "==> Running PII & Secret scan on tracked files..."

# 1. Scan for emails not matching the allowed domains
# Allowed domains: example.com, example.org, users.noreply.github.com
DISALLOWED_EMAILS=$(git ls-files | xargs grep -E -n -I -o '[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}' 2>/dev/null | grep -v -E '@(example\.com|example\.org|users\.noreply\.github\.com)$' || true)

if [ -n "$DISALLOWED_EMAILS" ]; then
    echo "❌ ERROR: Found non-whitelisted email addresses:"
    echo "$DISALLOWED_EMAILS"
    FAILURES=$((FAILURES + 1))
else
    echo "✓ Email whitelist check passed."
fi

# 2. Scan for private IPv4 addresses not matching allowed patterns
# Allowed patterns:
# - 192.168.1.50
# - 100.64.*
# - 10.*
# - 127.0.0.1
# - 0.0.0.0
DISALLOWED_IPS=$(git ls-files | xargs grep -E -n -I -o '([0-9]{1,3}\.){3}[0-9]{1,3}' 2>/dev/null | \
    grep -E ':(192\.168\.|172\.(1[6-9]|2[0-9]|3[0-1])\.)' | \
    grep -v -E ':192\.168\.1\.50$' || true)

if [ -n "$DISALLOWED_IPS" ]; then
    echo "❌ ERROR: Found forbidden private IP addresses:"
    echo "$DISALLOWED_IPS"
    FAILURES=$((FAILURES + 1))
else
    echo "✓ Private IP whitelist check passed."
fi

if [ "$FAILURES" -gt 0 ]; then
    echo "❌ PII & Secret check failed with $FAILURES error(s)."
    exit 1
fi

echo "✅ All PII & Secret sanity checks passed successfully."
exit 0
