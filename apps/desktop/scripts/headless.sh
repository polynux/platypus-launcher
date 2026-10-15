#!/usr/bin/env bash
# Run the desktop app on a headless Linux server: starts Xvfb on a free
# display number, then `bun run dev` (or `start`) on it. With --vnc, also
# starts x11vnc on that display and the noVNC web client, so the app can be
# opened from a LAN browser.
#
# Usage: headless.sh [--vnc] [dev|start] [args passed to `bun run`...]
#
# Env overrides (exact values; the script refuses a busy one):
#   PLATYPUS_XVFB_DISPLAY  display number, default: first free from 99
#   PLATYPUS_VNC_PORT      VNC port, default: first free from 5910
#   PLATYPUS_WEB_PORT      browser port, default: first free from 6090 (never 6080)
#
# PLATYPUS_* behaviour is untouched (point PLATYPUS_DATA_DIR etc. at a
# throwaway dir as usual). Everything started (Xvfb, x11vnc, the noVNC
# proxy, the app) logs to one file under /tmp; Ctrl-C or the app exiting
# stops them all, by PID.
set -u

VNC=0
if [ "${1:-}" = "--vnc" ]; then VNC=1; shift; fi
TARGET="${1:-dev}"
if [ "$TARGET" != dev ] && [ "$TARGET" != start ]; then
  echo "usage: headless.sh [--vnc] [dev|start] [args...]" >&2; exit 2
fi
if [ $# -gt 0 ]; then shift; fi

if [ "$VNC" = 1 ] && [ ! -f "$HOME/.vnc/passwd" ]; then
  echo "headless.sh: no VNC password: run \`x11vnc -storepasswd\` once first" >&2
  exit 1
fi

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

# Exact port from env, or the first free one from a default start.
pick_port() { # $1 = env value, $2 = default start
  if [ -n "$1" ]; then
    case "$1" in *[!0-9]*) echo "headless.sh: bad port: $1" >&2; exit 2;; esac
    if [ "$1" = 6080 ] || listening "$1"; then
      echo "headless.sh: port $1 is reserved or in use" >&2; exit 1
    fi
    echo "$1"
  else
    free_port "$2" || { echo "headless.sh: no free port from $2" >&2; exit 1; }
  fi
}

pick_display() { # $1 = env value
  if [ -n "$1" ]; then
    case "$1" in *[!0-9]*) echo "headless.sh: bad display: $1" >&2; exit 2;; esac
    if [ -e "/tmp/.X$1-lock" ]; then echo "headless.sh: display :$1 is in use" >&2; exit 1; fi
    echo "$1"
  else
    n=99
    while [ "$n" -lt 200 ]; do
      if [ ! -e "/tmp/.X$n-lock" ]; then echo "$n"; return 0; fi
      n=$((n + 1))
    done
    echo "headless.sh: no free display" >&2; exit 1
  fi
}

ROOT=$(cd "$(dirname "$0")/../../.." && pwd)
cd "$ROOT"
LOG=/tmp/platypus-headless-$(basename "$ROOT")-$(date +%Y%m%d-%H%M%S).log
echo "log: $LOG"

N=$(pick_display "${PLATYPUS_XVFB_DISPLAY:-}") || exit 1
export DISPLAY=:$N

Xvfb ":$N" -screen 0 1280x800x24 -nolisten tcp >>"$LOG" 2>&1 &
XVFB=$!
for ((i = 0; i < 50; i++)); do
  [ -S "/tmp/.X11-unix/X$N" ] && break
  kill -0 "$XVFB" 2>/dev/null || { echo "headless.sh: Xvfb failed, see $LOG" >&2; exit 1; }
  sleep 0.2
done
if [ ! -S "/tmp/.X11-unix/X$N" ]; then
  echo "headless.sh: Xvfb never opened display :$N, see $LOG" >&2
  kill "$XVFB" 2>/dev/null; exit 1
fi
echo "display: :$N (Xvfb pid $XVFB)"

KIDS=("$XVFB")
cleanup() { kill "${KIDS[@]}" 2>/dev/null; wait 2>/dev/null; KIDS=(); }
trap cleanup INT TERM

if [ "$VNC" = 1 ]; then
  VNC_PORT=$(pick_port "${PLATYPUS_VNC_PORT:-}" 5910) || { cleanup; trap - INT TERM; exit 1; }
  x11vnc -display ":$N" -rfbport "$VNC_PORT" -rfbauth "$HOME/.vnc/passwd" \
    -localhost -forever -shared >>"$LOG" 2>&1 &
  X11VNC=$!; KIDS+=("$X11VNC")
  WEB_PORT=$(pick_port "${PLATYPUS_WEB_PORT:-}" 6090) || { cleanup; trap - INT TERM; exit 1; }
  if [ -x /usr/share/novnc/utils/novnc_proxy ]; then
    /usr/share/novnc/utils/novnc_proxy --listen "$WEB_PORT" \
      --vnc "localhost:$VNC_PORT" --web /usr/share/novnc >>"$LOG" 2>&1 &
  else
    websockify --web /usr/share/novnc "$WEB_PORT" "localhost:$VNC_PORT" >>"$LOG" 2>&1 &
  fi
  PROXY=$!; KIDS+=("$PROXY")
  sleep 1
  kill -0 "$X11VNC" 2>/dev/null || { echo "headless.sh: x11vnc failed, see $LOG" >&2; cleanup; trap - INT TERM; exit 1; }
  kill -0 "$PROXY" 2>/dev/null || { echo "headless.sh: noVNC proxy failed, see $LOG" >&2; cleanup; trap - INT TERM; exit 1; }
  echo "vnc: x11vnc on 127.0.0.1:$VNC_PORT (pid $X11VNC), web on $WEB_PORT (pid $PROXY)"
  echo "open http://$(hostname).local:$WEB_PORT/vnc.html (or http://$(hostname -I 2>/dev/null | awk '{print $1}'):$WEB_PORT/vnc.html)"
fi

echo "app: bun run $TARGET $*"
bun run "$TARGET" "$@" >>"$LOG" 2>&1 &
APP=$!; KIDS+=("$APP")
wait "$APP"
CODE=$?
cleanup
trap - INT TERM
exit "$CODE"
