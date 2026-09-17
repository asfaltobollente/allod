#!/usr/bin/env bash
# scripts/check-status-table.sh - Automated README Project Status table check
set -euo pipefail

README="README.md"
if [ ! -f "$README" ]; then
    echo "❌ ERROR: $README not found"
    exit 1
fi

echo "==> Verifying Project Status table in $README..."

if ! grep -q "## 🧭 Project Status" "$README"; then
    echo "❌ ERROR: Missing '## 🧭 Project Status' section in $README"
    exit 1
fi

# Check for the three key status categories
for section in "Funzionante oggi" "In sviluppo" "Pianificato"; do
    if ! grep -i -q "$section" "$README"; then
        echo "❌ ERROR: Missing '$section' category in Project Status table"
        exit 1
    fi
done

echo "✓ Found '## 🧭 Project Status' section with all 3 lifecycle categories."
echo "✅ Project Status table verification passed."
exit 0