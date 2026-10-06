#!/bin/sh
# Scaffolds a minimal working store plugin: plugins/<id>/ with a manifest, a
# Python stub that speaks the whole protocol (api 1), and a README. The stub
# lists one fake game, installs it into the folder Hydra hands it, and launches
# it — enough to approve in the Plugins view and try end to end with no other
# tooling. Replace the marked sections with real store calls.
#
#   plugins/new.sh my-store mystore
#
# Install the result with plugins/install.sh <id>.
set -eu

id=${1:?usage: plugins/new.sh <id> [store]   # id: folder and plugin id}
store=${2:-$id}
here=$(cd "$(dirname "$0")" && pwd)
dest="$here/$id"
[ "$id" != "$(basename "$id")" ] && { echo "id must be a plain folder name" >&2; exit 1; }
[ -e "$dest" ] && { echo "$dest already exists" >&2; exit 1; }
case $id in steam|custom) echo "the id $id is reserved" >&2; exit 1;; esac

mkdir -p "$dest/bin"

cat > "$dest/plugin.json" <<EOF
{
  "id": "$id",
  "name": "$(echo "$id" | sed 's/-/ /g; s/\b./\u&/') (stub)",
  "version": "0.1.0",
  "author": "you",
  "description": "Scaffolded store plugin: lists one game, installs and launches it. Edit bin/hydra-$id.",
  "api": 1,
  "store": "$store",
  "exec": "bin/hydra-$id",
  "capabilities": ["library", "install", "launch", "auth"],
  "requires": [{ "bin": "python3", "hint": "install python3 from your distribution" }],
  "install_dir": "$store",
  "permissions": { "network": [], "read": [], "write": [] }
}
EOF

cat > "$dest/bin/hydra-$id" <<'EOF'
#!/usr/bin/env python3
"""A minimal store plugin (api 1): JSON-RPC over stdin/stdout, one object per
line. See docs/plugins/protocol.md. Everything between BEGIN/END STORE markers
is what you replace to wrap a real store."""

import json
import os
import re
import subprocess
import sys
import threading

API = 1
CODE_AUTH_REQUIRED = -32001


class RPCError(Exception):
    def __init__(self, message, code=1):
        super().__init__(message)
        self.code = code


out_lock = threading.Lock()


def send(msg):
    line = json.dumps(msg, separators=(",", ":"))
    with out_lock:
        sys.stdout.write(line + "\n")
        sys.stdout.flush()


def notify(method, params):
    send({"method": method, "params": params})


def tail(text, n=3):
    lines = [l.strip() for l in text.splitlines() if l.strip()]
    return " · ".join(lines[-n:]) if lines else "no output"


# --- auth ----------------------------------------------------------------

user = None  # login state lives in this process; a real plugin keeps tokens on disk


def auth_status(_params=None):
    return {"loggedIn": user is not None, **({"user": user} if user else {})}


def auth_begin(_params):
    # BEGIN STORE: point the user at the real login and ask for the credential.
    return {
        "instructions": "Log in to the store, then paste anything to continue.",
        "fields": [{"id": "token", "label": "login code", "secret": True}],
    }


def auth_complete(params):
    global user
    fields = (params or {}).get("fields") or {}
    token = (fields.get("token") or "").strip()
    if not token:
        raise RPCError("paste the login code from the store's page")
    user = "stub-user"  # BEGIN STORE: exchange the token here.
    return auth_status()


def auth_logout(_params):
    global user
    user = None
    return {"loggedIn": False}


# --- library -------------------------------------------------------------

GAMES = {
    "stub-game": {
        "title": "Stub Game",
        "native": True,  # launches as a plain script, no Proton
        "script": "#!/bin/sh\necho 'stub game is playing'\nsleep 3\n",
    },
}


def library_list(_params):
    out = []
    for app, g in GAMES.items():
        installed = os.path.isfile(install_path(app))
        out.append({
            "id": app,
            "title": g["title"],
            "installed": installed,
            "installable": True,
            "native": g["native"],
            "installPath": os.path.dirname(install_path(app)) if installed else "",
        })
    return {"games": out}


def install_path(app):
    # Where the game landed. Hydra picks the folder and passes it to
    # game.install; remember it (a real plugin keeps this in HYDRA_PLUGIN_DATA).
    return INSTALLS.get(app, "")


def install_size(params):
    app = params["id"]
    if app not in GAMES:
        raise RPCError(f"unknown game {app}")
    return {"downloadBytes": 1024, "diskBytes": 4096}


# --- jobs ----------------------------------------------------------------

jobs = {}
jobs_lock = threading.Lock()
INSTALLS = {}  # game id -> installed game.sh path


def start_job(params, kind):
    app, job_id = params["id"], params["jobId"]
    if app not in GAMES:
        raise RPCError(f"unknown game {app}")
    if kind == "uninstall":
        script = INSTALLS.pop(app, "")
        if script and os.path.isfile(script):
            import shutil
            shutil.rmtree(os.path.dirname(script), ignore_errors=True)
        notify("job.done", {"jobId": job_id})
        return {}
    install_dir = params.get("installDir") or ""
    if not os.path.isabs(install_dir):
        raise RPCError("installDir is missing or relative; Hydra always sends one")
    threading.Thread(target=follow_install, args=(job_id, app, install_dir), daemon=True).start()
    return {}


def follow_install(job_id, app, install_dir):
    import time
    dest = os.path.join(install_dir, app.replace("-", ""))
    os.makedirs(dest, exist_ok=True)
    script = os.path.join(dest, "game.sh")
    for pct in (50, 100):
        time.sleep(0.2)
        notify("job.progress", {"jobId": job_id, "percent": pct, "phase": "downloading",
                                "bytes": 512 * pct // 100, "total": 512})
    with open(script, "w") as f:
        f.write(GAMES[app]["script"])
    os.chmod(script, 0o755)
    INSTALLS[app] = script
    notify("job.done", {"jobId": job_id})


def job_cancel(params):
    with jobs_lock:
        job = jobs.pop(params.get("jobId"), None)
    if job and job.get("proc"):
        job["proc"].kill()
    return {}


# --- launch --------------------------------------------------------------

def launch_spec(params):
    app = params["id"]
    g = GAMES.get(app)
    if not g:
        raise RPCError(f"unknown game {app}")
    script = install_path(app)
    if not script or not os.path.isfile(script):
        raise RPCError(f"{g['title']} is not installed")
    return {
        "exe": script,
        "args": [],
        "cwd": os.path.dirname(script),
        "env": {},
        "needsWine": not g["native"],
        "native": g["native"],
    }


def game_details(params):
    # Returning an error is fine: Hydra shows the library entry without detail.
    raise RPCError("no store metadata", -32601)


# --- dispatch ------------------------------------------------------------

def initialize(params):
    return {"name": "STUBNAME", "version": "0.1.0",
            "api": API, "capabilities": ["library", "install", "launch", "auth"], "missing": []}


METHODS = {
    "initialize": initialize,
    "auth.status": auth_status,
    "auth.begin": auth_begin,
    "auth.complete": auth_complete,
    "auth.logout": auth_logout,
    "library.list": library_list,
    "game.details": game_details,
    "game.installSize": install_size,
    "game.install": lambda p: start_job(p, "install"),
    "game.update": lambda p: start_job(p, "install"),
    "game.uninstall": lambda p: start_job(p, "uninstall"),
    "job.cancel": job_cancel,
    "game.launchSpec": launch_spec,
}


def main():
    for line in sys.stdin:
        line = line.strip()
        if not line:
            continue
        try:
            msg = json.loads(line)
        except ValueError:
            continue
        resp = {"id": msg.get("id")}
        fn = METHODS.get(msg.get("method"))
        try:
            if fn is None:
                raise RPCError(f"method not found: {msg.get('method')}", -32601)
            resp["result"] = fn(msg.get("params") or {}) or {}
        except RPCError as e:
            resp["error"] = {"code": e.code, "message": str(e)}
        except Exception as e:  # never crash on a request; report it
            resp["error"] = {"code": 1, "message": f"{type(e).__name__}: {e}"}
        send(resp)


if __name__ == "__main__":
    main()
EOF

chmod 755 "$dest/bin/hydra-$id"

# The quoted heredoc kept the Python free of shell expansion; fill in the
# plugin name afterwards.
sed -i "s/STUBNAME/$id (stub)/" "$dest/bin/hydra-$id"

cat > "$dest/README.md" <<EOF
# $id

A Hydra store plugin for $store, scaffolded with \`plugins/new.sh\`. See
[docs/plugins/protocol.md](../../docs/plugins/protocol.md) for the protocol
and [docs/plugins/data-plugins.md](../../docs/plugins/data-plugins.md) for
JSON-only plugins (fix recipes, tweak packs).

## Run it

\`\`\`sh
plugins/install.sh $id
\`\`\`

Then approve it in Hydra's Plugins view and log in (any code works in the
stub). It lists one "Stub Game" that installs into Hydra's games folder and
"plays" for three seconds.

## Make it real

Everything to change lives in \`bin/hydra-$id\` between the BEGIN/END STORE
markers and the \`GAMES\` table:

1. \`auth_begin\`/\`auth_complete\`: hand the user the store's login page and
   exchange what they paste for tokens. Keep tokens on disk (the plugin's
   \`HYDRA_PLUGIN_DATA\` folder is private); never log them.
2. \`library_list\`: the games the user owns, installed or not. Set \`native\`
   when a game ships a Linux build.
3. \`install_size\`/\`start_job\`: drive the store's download, reporting
   \`job.progress\` notifications; installs must be resumable (Hydra re-issues
   the same \`jobId\` after a crash).
4. \`launch_spec\`: the executable inside the install folder Hydra passed to
   \`game.install\`; Hydra runs it (Proton, wrappers, session tracking).
5. \`plugin.json\`: list the command-line tools you need in \`requires\` and the
   hosts/paths you touch in \`permissions\` — the Plugins view shows both
   before the user approves.
EOF

echo "scaffolded $dest"
echo "next: plugins/install.sh $id"
