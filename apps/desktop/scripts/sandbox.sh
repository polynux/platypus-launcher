#!/usr/bin/env bash
# Sandbox dev run (issue #68): the real app in a throwaway sandbox, on the
# fake programs (testbin/, #66) and the seeded echo library with fake games
# (ECHO_SEED / ECHO_FIXTURES, #67).
#
# Usage: sandbox.sh [--vnc] [--clean] [--sandbox-dir <dir>] [dev|start] [args passed to `bun run`...]
#
# A fresh dir under /tmp holds an isolated HOME and PLATYPUS_DATA_DIR /
# CONFIG_DIR / CACHE_DIR / PLUGINS; echo is installed from the working tree,
# approved and logged in (code letmein); testbin/ is first on PATH and the
# fake Proton build is registered as a runner where the app discovers runners
# (compatibilitytools.d, like a real custom Proton). Remote debugging listens
# on a free port from 9233 (printed, loopback only) and Vite on 5193 (or a
# free one nearby when another sandbox starts at the same time), so two
# sandboxes run side by side. Everything the run starts stops with it, by
# PID (see headless.sh); the sandbox dir is kept for inspection unless
# --clean deletes it afterwards.
set -u

VNC=0
CLEAN=0
TARGET=dev
SANDBOX_DIR=""
while [ $# -gt 0 ]; do
  case "$1" in
    --vnc) VNC=1;;
    --clean|--rm) CLEAN=1;;
    --sandbox-dir) SANDBOX_DIR="$2"; shift;;
    --sandbox-dir=*) SANDBOX_DIR="${1#--sandbox-dir=}";;
    --) ;; # bun passes `--` through; it carries no meaning here
    dev|start) TARGET="$1";;
    *) break;; # the rest passes through to `bun run`
  esac
  shift
done

ROOT=$(cd "$(dirname "$0")/../../.." && pwd)
cd "$ROOT"

# True when something on this machine listens on 127.0.0.1:$1.
listening() { (exec 3<>"/dev/tcp/127.0.0.1/$1") 2>/dev/null; }

# First free port from $1 up (skips 6080: taken on our dev server).
free_port() {
  p=$1
  while [ "$p" -lt "$(( $1 + 200 ))" ]; do
    if [ "$p" -ne 6080 ] && ! listening "$p"; then echo "$p"; return 0; fi
    p=$((p + 1))
  done
  return 1
}

SANDBOX=""
if [ -n "$SANDBOX_DIR" ]; then
  # A given dir is reused across starts (issue #69: the restart scenario
  # stops the app and starts it again on the same library and settings).
  SANDBOX="$SANDBOX_DIR"
  mkdir -p "$SANDBOX"
  echo "sandbox: $SANDBOX (reused)"
else
  SANDBOX=$(mktemp -d /tmp/platypus-sandbox-XXXXXX)
  echo "sandbox: $SANDBOX"
fi
HEADLESS=0
cleanup_sandbox() {
  [ "$HEADLESS" -ne 0 ] && kill "$HEADLESS" 2>/dev/null
  if command -v release_reservations >/dev/null 2>&1; then release_reservations; fi
  [ "$CLEAN" = 1 ] && rm -rf "$SANDBOX"
}
trap cleanup_sandbox EXIT INT TERM

REAL_HOME="$HOME"
# Tool shims (asdf) resolve versions from $HOME: pin them at the real home
# before it is replaced, or go/node/python3 vanish inside the sandbox.
export ASDF_DIR="${ASDF_DIR:-$REAL_HOME/.asdf}"
export ASDF_DATA_DIR="${ASDF_DATA_DIR:-$REAL_HOME/.asdf}"
export ASDF_DEFAULT_TOOL_VERSIONS_FILENAME="${ASDF_DEFAULT_TOOL_VERSIONS_FILENAME:-$REAL_HOME/.tool-versions}"
# Build caches live outside the sandbox: the sandbox stays a fresh app home
# while go reuses the machine's warm cache (a cold HOME would recompile and
# redownload modules on every run). Asked before HOME changes.
export GOCACHE="$(go env GOCACHE)" GOMODCACHE="$(go env GOMODCACHE)"
export HOME="$SANDBOX/home"
mkdir -p "$HOME"
export PLATYPUS_DATA_DIR="$SANDBOX/data"
export PLATYPUS_CONFIG_DIR="$SANDBOX/config"
export PLATYPUS_CACHE_DIR="$SANDBOX/cache"
export PLATYPUS_PLUGINS="$SANDBOX/data/plugins"
mkdir -p "$PLATYPUS_DATA_DIR" "$PLATYPUS_CONFIG_DIR" "$PLATYPUS_CACHE_DIR" "$PLATYPUS_PLUGINS"

# Fake programs first on PATH (issue #66): steam, winetricks, protontricks,
# xdg-open and the fake Proton build answer instead of system packages.
export PATH="$ROOT/testbin:$PATH"
export PLATYPUS_TESTBIN_LOG="$SANDBOX/testbin-calls.log"
touch "$PLATYPUS_TESTBIN_LOG"

# Seeded library and fake games (issue #67): 30 deterministic bulk games plus
# one game per launch behaviour (run / clean exit / fail / crash). The crash
# game runs through the runner (wine:true, issue #69) so the core creates a
# prefix for it and the winetricks fix applies; the others stay native.
export ECHO_GAMES=30
export ECHO_SEED=7
export ECHO_FIXTURES='{"sandbox-run":{"mode":"run"},"sandbox-ok":{"mode":"exit0"},"sandbox-fail":{"mode":"fail","code":3},"sandbox-crash":{"mode":"crash","log":"vcruntime","afterMs":200,"wine":true}}'

# Fake Proton as a runner: a custom Proton where runner discovery looks
# (internal/runner/scanner.go), with the real layout (proton plus
# files/bin/wine and files/bin/wineserver) the prefix logic expects.
FAKEPROTON="$HOME/.local/share/Steam/compatibilitytools.d/FakeProton-Sandbox"
mkdir -p "$FAKEPROTON"
cp -r "$ROOT/testbin/proton/proton" "$ROOT/testbin/proton/files" "$FAKEPROTON/"
chmod 755 "$FAKEPROTON/proton" "$FAKEPROTON/files/bin/wine" "$FAKEPROTON/files/bin/wineserver"
echo "runner: FakeProton-Sandbox ($FAKEPROTON/proton)"

# echo installed from the working tree, then approved and logged in through
# the real core (sandbox-setup.mjs speaks the Plugins view's own RPCs), so
# the app opens on the seeded library with no new core surface. Each step is
# guarded: a failed setup must never launch the app on a broken sandbox.
sh "$ROOT/plugins/install.sh" echo || { echo "sandbox.sh: installing echo failed" >&2; exit 1; }
mkdir -p "$SANDBOX/bin"
go build -o "$SANDBOX/bin/platypus-core-setup" ./cmd/platypus-core || { echo "sandbox.sh: building the core failed" >&2; exit 1; }
node "$ROOT/apps/desktop/scripts/sandbox-setup.mjs" "$SANDBOX/bin/platypus-core-setup" || { echo "sandbox.sh: approving echo failed" >&2; exit 1; }

# Free ports and displays are reserved, not just picked: first-free checks
# alone race when two sandboxes start together (both see :99/5193/9233 free
# and share them). Each number is held with an atomic mkdir until our own
# servers bind it, so a second starter's picks skip it. The first try prefers
# the memorable bases (:99, 5193, 9233, VNC 5910/6090); on collision later
# tries draw a random free one in range.
RESV=/tmp/platypus-sandbox-ports
mkdir -p "$RESV"
RESV_KEYS=""

first_free_display() {
  n=99
  while [ "$n" -lt 200 ]; do
    if [ ! -e "/tmp/.X$n-lock" ]; then echo "$n"; return 0; fi
    n=$((n + 1))
  done
  return 1
}

random_free() { # $1 = base, $2 = count: a free port in [base, base+count)
  for _ in $(seq 1 50); do
    p=$(($1 + RANDOM % $2))
    [ "$p" -ne 6080 ] && ! listening "$p" && { echo "$p"; return 0; }
  done
  return 1
}

random_free_display() {
  for _ in $(seq 1 50); do
    n=$((99 + RANDOM % 100))
    if [ ! -e "/tmp/.X$n-lock" ]; then echo "$n"; return 0; fi
  done
  return 1
}

# reserve holds one number ($1 = tag, $2 = value); release drops what claim
# took. Tags keep resource kinds apart (D display, V vite, P debugging,
# Q vnc, W web), so a display :99 never blocks a port 99.
reserve() { mkdir "$RESV/$1-$2" 2>/dev/null && RESV_KEYS="$RESV_KEYS $1-$2"; }
release_reservations() {
  for k in $RESV_KEYS; do rmdir "$RESV/$k" 2>/dev/null; done
  RESV_KEYS=""
}

claim_ports() {
  for try in $(seq 1 30); do
    if [ "$try" = 1 ]; then
      N=$(first_free_display) || return 1
      VP=$(free_port 5193) && DP=$(free_port 9233) || return 1
      if [ "$VNC" = 1 ]; then VNP=$(free_port 5910) && WP=$(free_port 6090) || return 1; fi
    else
      N=$(random_free_display) || continue
      VP=$(random_free 5193 200) && DP=$(random_free 9233 200) || continue
      if [ "$VNC" = 1 ]; then VNP=$(random_free 5910 200) && WP=$(random_free 6090 200) || continue; fi
    fi
    if reserve D "$N" && reserve V "$VP" && reserve P "$DP" \
        && { [ "$VNC" = 0 ] || { reserve Q "$VNP" && reserve W "$WP"; }; }; then
      return 0
    fi
    release_reservations # partial take: drop it and draw again
    sleep 0.5
  done
  return 1
}

claim_ports || { echo "sandbox.sh: no free display/ports for another sandbox" >&2; exit 1; }
# RESV_KEYS reads e.g. "D-99 V-5193 P-9233": split each tag back out.
for k in $RESV_KEYS; do
  case "$k" in
    D-*) N=${k#D-};;
    V-*) VP=${k#V-};;
    P-*) DP=${k#P-};;
    Q-*) VNP=${k#Q-};;
    W-*) WP=${k#W-};;
  esac
done
export PLATYPUS_XVFB_DISPLAY="$N"
export PLATYPUS_VITE_PORT="$VP"
export PLATYPUS_REMOTE_DEBUGGING_PORT="$DP"
if [ "$VNC" = 1 ]; then
  # Exact values: headless.sh refuses a busy one instead of sharing it.
  export PLATYPUS_VNC_PORT="$VNP"
  export PLATYPUS_WEB_PORT="$WP"
fi
echo "vite: http://127.0.0.1:$VP"
echo "debug: http://127.0.0.1:$DP/json/list (CDP over the printed websocketDebuggerUrl)"

# The sandbox is throwaway and headless: no setuid sandbox helper is
# configured on the server, so Electron runs unprivileged like the AppImage
# does in containers (docs/electron.md).
export PLATYPUS_ELECTRON_ARGS="${PLATYPUS_ELECTRON_ARGS:-} --no-sandbox"

# headless.sh refuses --vnc without ~/.vnc/passwd: the sandbox HOME is fresh,
# so carry the user's password file over when it has one of its own.
if [ "$VNC" = 1 ] && [ ! -f "$HOME/.vnc/passwd" ] && [ -f "$REAL_HOME/.vnc/passwd" ]; then
  mkdir -p "$HOME/.vnc"
  cp "$REAL_HOME/.vnc/passwd" "$HOME/.vnc/passwd"
fi

echo "log: tail the path headless.sh prints below"
echo "app: bun run $TARGET $*"
if [ "$VNC" = 1 ]; then
  bash "$ROOT/apps/desktop/scripts/headless.sh" --vnc "$TARGET" "$@" &
else
  bash "$ROOT/apps/desktop/scripts/headless.sh" "$TARGET" "$@" &
fi
HEADLESS=$!
# The reservations held our numbers through the pick-vs-bind race; drop them
# once our own servers bound them, so later starters see busy ports instead
# of held ones. (A reservation outlives a slow start; a dead run's leftover
# only costs the next claim one retry.)
for _ in $(seq 1 150); do
  kill -0 "$HEADLESS" 2>/dev/null || break # headless died: nothing to wait for
  [ -e "/tmp/.X$N-lock" ] || { sleep 2; continue; }
  { [ "$TARGET" = start ] || listening "$VP"; } || { sleep 2; continue; }
  listening "$DP" && break
  sleep 2
done
release_reservations
wait "$HEADLESS"
CODE=$?
HEADLESS=0
trap - EXIT INT TERM
cleanup_sandbox
exit "$CODE"
