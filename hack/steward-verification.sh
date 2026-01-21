#!/bin/bash
set -e

echo "=============================================="
echo "Steward Rebrand Verification Script"
echo "=============================================="

pass() { echo "✓ PASS: $1"; }
fail() { echo "✗ FAIL: $1"; }

echo ""
echo "=== 1. Verify Module Path ==="
if head -1 go.mod | grep -q "module github.com/butlerdotdev/steward"; then
    pass "go.mod module path is correct"
else
    fail "go.mod module path is incorrect"
fi

echo ""
echo "=== 2. Check for Remaining Kamaji/Clastix References ==="
echo "(Excluding LICENSE, NOTICE, and docs FAQ)"

REFS=$(grep -ri "kamaji\|clastix" --include="*.go" --include="*.yaml" --include="Makefile" . 2>/dev/null | grep -v ".git" || true)
if [ -z "$REFS" ]; then
    pass "No kamaji/clastix references in code files"
else
    fail "Found kamaji/clastix references:"
    echo "$REFS"
fi

echo ""
echo "=== 3. Verify Chart Directories ==="
[ -d charts/steward ] && pass "charts/steward/ exists" || fail "charts/steward/ missing"
[ -d charts/steward-crds ] && pass "charts/steward-crds/ exists" || fail "charts/steward-crds/ missing"
[ ! -d charts/kamaji ] && pass "charts/kamaji/ removed" || fail "charts/kamaji/ still exists"
[ ! -d charts/kamaji-crds ] && pass "charts/kamaji-crds/ removed" || fail "charts/kamaji-crds/ still exists"

echo ""
echo "=== 4. Verify API Group ==="
if grep -q 'GroupVersion = schema.GroupVersion{Group: "steward.butlerlabs.dev"' api/v1alpha1/groupversion_info.go; then
    pass "API group is steward.butlerlabs.dev"
else
    fail "API group is incorrect"
fi

echo ""
echo "=== 5. Build Test ==="
echo "Running: CGO_ENABLED=0 go build ./..."
if CGO_ENABLED=0 go build ./... 2>/dev/null; then
    pass "Build succeeded"
else
    fail "Build failed"
fi

echo ""
echo "=============================================="
echo "Verification Complete"
echo "=============================================="
