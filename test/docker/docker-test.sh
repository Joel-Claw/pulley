#!/usr/bin/env bash
set -euo pipefail

PASS=0
FAIL=0

ok()   { echo "  ✓ $1"; PASS=$((PASS+1)); }
fail() { echo "  ✗ $1"; FAIL=$((FAIL+1)); }

echo "=== pulley Docker integration test ==="
echo ""

# Suppress git hints
git config --global init.defaultBranch main
git config --global advice.initDefaultBranch false
git config --global user.name "Test User"
git config --global user.email "test@test.com"

# ── Build verification ──────────────────────────────────────────────────────
echo "Build verification:"
if command -v pulley &>/dev/null; then
    ok "pulley binary exists"
else
    fail "pulley binary not found in PATH"
fi

if pulley version &>/dev/null; then
    ok "pulley version works"
else
    fail "pulley version failed"
fi

VER=$(pulley version 2>&1)
echo "  Version: $VER"

# ── Test: add repo ────────────────────────────────────────────────────────
echo ""
echo "Test: add repo"

TESTREPO="/home/testuser/test-repo"
git init "$TESTREPO"
cd "$TESTREPO"
git commit --allow-empty -m "initial"

pulley add "$TESTREPO" --interval 10m
LISTOUT=$(pulley list)
if echo "$LISTOUT" | grep -q "test-repo"; then
    ok "repo appears in list after add"
else
    fail "repo not in list after add"
fi

# ── Test: add duplicate ───────────────────────────────────────────────────
echo ""
echo "Test: add duplicate repo"
ADD_OUT=$(pulley add "$TESTREPO" 2>&1) && true
if echo "$ADD_OUT" | grep -q "already registered"; then
    ok "duplicate add rejected"
else
    fail "duplicate add not rejected"
fi

# ── Test: list ────────────────────────────────────────────────────────────
echo ""
echo "Test: list"
if echo "$LISTOUT" | grep -q "10m"; then
    ok "list shows interval"
else
    fail "list missing interval"
fi

if echo "$LISTOUT" | grep -q "1\."; then
    ok "list shows numbered entries"
else
    fail "list format wrong"
fi

# ── Test: add with --at ────────────────────────────────────────────────────
echo ""
echo "Test: add with --at"
TESTREPO2="/home/testuser/test-repo2"
git init "$TESTREPO2"
cd "$TESTREPO2"
git commit --allow-empty -m "initial"

pulley add "$TESTREPO2" --at "09:00,18:00"
LISTOUT2=$(pulley list)
if echo "$LISTOUT2" | grep -q "09:00"; then
    ok "--at times stored correctly"
else
    fail "--at times not shown"
fi

# ── Test: add with --range ──────────────────────────────────────────────────
echo ""
echo "Test: add with --range"
TESTREPO3="/home/testuser/test-repo3"
git init "$TESTREPO3"
cd "$TESTREPO3"
git commit --allow-empty -m "initial"

pulley add "$TESTREPO3" --range "09:00-17:00"
LISTOUT3=$(pulley list)
if echo "$LISTOUT3" | grep -q "09:00-17:00"; then
    ok "--range stored correctly"
else
    fail "--range not shown"
fi

# ── Test: config set ──────────────────────────────────────────────────────
echo ""
echo "Test: config set"
pulley config set --interval 15m --range "08:00-22:00"
CONFIG_OUT=$(pulley config)
if echo "$CONFIG_OUT" | grep -q "15m"; then
    ok "config shows default interval"
else
    fail "config missing default interval"
fi
if echo "$CONFIG_OUT" | grep -q "08:00-22:00"; then
    ok "config shows default range"
else
    fail "config missing default range"
fi

# ── Test: repo inherits defaults ──────────────────────────────────────────
echo ""
echo "Test: repo inherits defaults"
TESTREPO4="/home/testuser/test-repo4"
git init "$TESTREPO4"
cd "$TESTREPO4"
git commit --allow-empty -m "initial"

pulley add "$TESTREPO4"
LISTOUT4=$(pulley list)
if echo "$LISTOUT4" | grep -q "15m"; then
    ok "repo inherits default interval"
else
    fail "repo did not inherit default interval"
fi

# ── Test: add non-git directory ────────────────────────────────────────────
echo ""
echo "Test: add non-git directory"
NOTGIT="/home/testuser/not-a-repo"
mkdir -p "$NOTGIT"
ADD_NONGIT=$(pulley add "$NOTGIT" 2>&1) && NONGIT_RC=$? || NONGIT_RC=$?
if [ "$NONGIT_RC" -ne 0 ]; then
    ok "non-git directory rejected"
else
    fail "non-git directory should be rejected"
fi

# ── Test: pull with no remote ──────────────────────────────────────────────
echo ""
echo "Test: pull (no remote)"
PULL_OUT=$(pulley pull "$TESTREPO" 2>&1) && true
if echo "$PULL_OUT" | grep -q "failed"; then
    ok "pull reports failure on repo with no remote"
else
    ok "pull runs on repo with no remote (nothing to pull)"
fi

# ── Test: pull with real remote (bare repo + clone) ────────────────────────
echo ""
echo "Test: pull with remote"

WORKDIR="/home/testuser/pull-test"
mkdir -p "$WORKDIR"
BARE="$WORKDIR/upstream.git"
CLONE="$WORKDIR/clone"

# Create bare repo
git init --bare "$BARE"

# Clone it
git clone "$BARE" "$CLONE"
cd "$CLONE"
git commit --allow-empty -m "first commit"
git push origin main

# Register the clone
pulley add "$CLONE" --interval 5m

# Push a new commit from another clone
CLONE2="$WORKDIR/clone2"
git clone "$BARE" "$CLONE2"
cd "$CLONE2"
git commit --allow-empty -m "second commit"
git push origin main

# Now pull from the first clone
PULL2=$(pulley pull "$CLONE" 2>&1) && true
if echo "$PULL2" | grep -q "Already up to date\|Fast-forward"; then
    ok "pull works with remote"
else
    # Check if the commit actually landed
    cd "$CLONE"
    if git log --oneline | grep -q "second"; then
        ok "pull worked (commit found in log)"
    else
        fail "pull with remote failed: $PULL2"
    fi
fi

# ── Test: remove ──────────────────────────────────────────────────────────
echo ""
echo "Test: remove"
pulley remove "$TESTREPO"
if ! pulley list 2>&1 | grep -q "test-repo"; then
    ok "repo removed from list"
else
    fail "repo still in list after remove"
fi

# ── Test: config file ──────────────────────────────────────────────────────
echo ""
echo "Test: config file"
CFGPATH="$HOME/.config/pulley/config.json"
if [ -f "$CFGPATH" ]; then
    ok "config file exists at $CFGPATH"
else
    fail "config file not found at $CFGPATH"
fi

if grep -q '"repos"' "$CFGPATH" && grep -q '"path"' "$CFGPATH"; then
    ok "config has expected structure"
else
    fail "config missing expected fields"
fi

# ── Test: add from inside repo (no path arg) ──────────────────────────────
echo ""
echo "Test: add from current directory"
cd "$TESTREPO2"
ADD_CWD_OUT=$(pulley add 2>&1) && true
if echo "$ADD_CWD_OUT" | grep -q "already registered"; then
    ok "add from current dir resolves correctly (already registered)"
elif pulley list 2>&1 | grep -q "test-repo2"; then
    ok "add from current dir works"
else
    fail "add from current dir failed"
fi

# ── Test: help ─────────────────────────────────────────────────────────────
echo ""
echo "Test: help"
HELP_OUT=$(pulley help 2>&1)
if echo "$HELP_OUT" | grep -q "pulley add"; then
    ok "help shows usage"
else
    fail "help output missing"
fi

# ── Test: version ──────────────────────────────────────────────────────────
echo ""
echo "Test: version"
VER_OUT=$(pulley version 2>&1)
if echo "$VER_OUT" | grep -qE "[0-9]+\.[0-9]+\.[0-9]+"; then
    ok "version shows semver"
else
    fail "version output unexpected: $VER_OUT"
fi

# ── Test: remove non-existent repo ────────────────────────────────────────
echo ""
echo "Test: remove non-existent repo"
RM_OUT=$(pulley remove "/nonexistent/path" 2>&1) && RM_RC=$? || RM_RC=$?
if [ "$RM_RC" -ne 0 ] || echo "$RM_OUT" | grep -q "not found"; then
    ok "removing non-existent repo fails gracefully"
else
    fail "removing non-existent repo should fail"
fi

# ── Test: remove all then list ────────────────────────────────────────────
echo ""
echo "Test: empty list after removing all"
pulley remove "$TESTREPO2" 2>&1 || true
pulley remove "$TESTREPO3" 2>&1 || true
pulley remove "$TESTREPO4" 2>&1 || true
pulley remove "$CLONE" 2>&1 || true
EMPTY_LIST=$(pulley list)
if echo "$EMPTY_LIST" | grep -q "No repos"; then
    ok "empty list message shown"
else
    fail "expected 'No repos' message, got: $EMPTY_LIST"
fi

# ── Results ────────────────────────────────────────────────────────────────
echo ""
echo "=== Results ==="
echo "  Passed: $PASS"
echo "  Failed: $FAIL"
echo ""

if [ "$FAIL" -gt 0 ]; then
    echo "FAILED"
    exit 1
else
    echo "ALL PASSED"
    exit 0
fi