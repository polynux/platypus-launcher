#!/bin/sh
# Installs a plugin from this folder into ~/.local/share/platypus/plugins/<name>.
# Platypus then lists it as unapproved until you approve it in the Plugins view.
#
#   plugins/install.sh echo             # fake store, for trying the plugin UI
#   plugins/install.sh epic-legendary   # needs legendary: pipx install legendary-gl
#
# Go plugins are built; other plugins are copied as they are. Files are
# replaced by rename, so a running plugin keeps its old binary until restart
# (and Platypus asks you to approve the new one).
set -eu
name=${1:?usage: plugins/install.sh <plugin>}
here=$(cd "$(dirname "$0")" && pwd)
src=$here/$name
dest=${PLATYPUS_PLUGINS:-$HOME/.local/share/platypus/plugins}/$name
[ -f "$src/plugin.json" ] || { echo "no plugin named $name in $here" >&2; exit 1; }

# .new leftovers from an interrupted install are never valid; drop them.
cleanup() {
  rm -f "$dest"/*.new "$dest/bin"/*.new 2>/dev/null || true
}
trap cleanup EXIT INT TERM

# The manifest is JSON: parse it with python3, never with sed (a description
# mentioning '"exec": "bin/x"' must not hijack the install target).
if ! command -v python3 >/dev/null 2>&1; then
  hint=$(grep -A1 '"bin": *"python3"' "$src/plugin.json" | grep -o '"hint": *"[^"]*"' | head -n 1 || true)
  echo "$name: python3 is missing${hint:+\ ($hint)}. Install python3 first." >&2
  exit 2
fi
kind=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1])).get("kind",""))' "$src/plugin.json")
exec_rel=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1])).get("exec",""))' "$src/plugin.json")
icon_rel=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1])).get("icon",""))' "$src/plugin.json")
case "$icon_rel" in
  "") ;;
  /*|../*|*/../*) echo "$name/plugin.json icon escapes the folder: $icon_rel" >&2; exit 1 ;;
esac

# Data plugins (kind: data, no exec) are JSON only: copy the listed data
# files plus the manifest.
if [ "$kind" = "data" ]; then
  data_files=$(python3 -c 'import json,sys; print("\n".join(json.load(open(sys.argv[1])).get("data",["fixes.json","tweaks.json"])))' "$src/plugin.json")
  mkdir -p "$dest"
  # shellcheck disable=SC2086
  for d in $data_files; do
    case "$d" in
      ""|/*|../*|*/../*) echo "$name: refusing data file $d (must stay inside the folder)" >&2; exit 1 ;;
    esac
    if [ -f "$src/$d" ]; then
      mkdir -p "$dest/$(dirname "$d")"
      cp "$src/$d" "$dest/$d.new" && mv -f "$dest/$d.new" "$dest/$d"
    fi
  done
  cp "$src/plugin.json" "$dest/plugin.json.new" && mv -f "$dest/plugin.json.new" "$dest/plugin.json"
  if [ -n "$icon_rel" ] && [ -f "$src/$icon_rel" ]; then
    mkdir -p "$dest/$(dirname "$icon_rel")"
    cp "$src/$icon_rel" "$dest/$icon_rel.new" && mv -f "$dest/$icon_rel.new" "$dest/$icon_rel"
  fi
  trap - EXIT INT TERM
  echo "installed $name in $dest"
  exit 0
fi

[ -n "$exec_rel" ] || { echo "$name/plugin.json has no exec" >&2; exit 1; }
case "$exec_rel" in
  /*|../*|*/../*) echo "$name/plugin.json exec escapes the folder: $exec_rel" >&2; exit 1 ;;
esac

if ! command -v cp >/dev/null 2>&1; then
  echo "$name: cp is missing. Install coreutils first." >&2
  exit 2
fi

mkdir -p "$dest/$(dirname "$exec_rel")"
tmp="$dest/$exec_rel.new"
if [ -f "$src/main.go" ]; then
  if ! command -v go >/dev/null 2>&1; then
    hint=$(python3 -c 'import json,sys; m=json.load(open(sys.argv[1]));
print(next((r.get("hint","") for r in m.get("requires",[]) if r.get("bin")=="go"),""))' "$src/plugin.json")
    [ -n "$hint" ] || hint="install the Go toolchain from go.dev (only needed to build this plugin)"
    echo "$name: go is missing ($hint)." >&2
    exit 2
  fi
  (cd "$here/.." && go build -o "$tmp" "./plugins/$name")
else
  cp "$src/$exec_rel" "$tmp"
  chmod 755 "$tmp"
fi
mv -f "$tmp" "$dest/$exec_rel"
# Go helpers shipped next to a non-Go exec (e.g. cmd/platypus-steam-dl in a
# Python plugin) are built into the plugin's bin/ alongside it.
if [ -d "$src/cmd" ]; then
  if ! command -v go >/dev/null 2>&1; then
    hint=$(python3 -c 'import json,sys; m=json.load(open(sys.argv[1]));
print(next((r.get("hint","") for r in m.get("requires",[]) if r.get("bin")=="go"),""))' "$src/plugin.json")
    [ -n "$hint" ] || hint="install the Go toolchain from go.dev (only needed to build this plugin's helpers)"
    echo "$name: go is missing ($hint)." >&2
    exit 2
  fi
  for cmd in "$src"/cmd/*/; do
    [ -f "$cmd/main.go" ] || [ -n "$(ls "$cmd"*.go 2>/dev/null)" ] || continue
    helper=$(basename "$cmd")
    htmp="$dest/bin/$helper.new"
    mkdir -p "$dest/bin"
    (cd "$here/.." && go build -o "$htmp" "./plugins/$name/cmd/$helper")
    chmod 755 "$htmp"
    mv -f "$htmp" "$dest/bin/$helper"
  done
fi
# Extra module files sitting next to the exec (e.g. python modules it
# imports) are part of the plugin: copy them out, then rename over the old
# ones (never into a running file, and never out of the source folder).
# Files deleted upstream are removed from dest too, so reinstalls can't
# leave zombie helpers behind (still importable and still hashed).
exec_dir=$(dirname "$exec_rel")
exec_base=$(basename "$exec_rel")
for f in "$src/$exec_dir"/*; do
  base=$(basename "$f")
  case "$base" in
    "$exec_base"|__pycache__) continue ;;
    *.pyc) continue ;;
  esac
  [ -f "$f" ] || continue
  new="$dest/$exec_dir/$base.new"
  mkdir -p "$dest/$exec_dir"
  cp "$f" "$new" && mv -f "$new" "$dest/$exec_dir/$base"
done
if [ -d "$dest/$exec_dir" ]; then
  for f in "$dest/$exec_dir"/*; do
    [ -f "$f" ] || continue
    base=$(basename "$f")
    case "$base" in
      "$exec_base"|*.new) continue ;;
    esac
    # Helpers built from cmd/ land in bin/ without a source file there.
    if [ "$exec_dir" = bin ] && [ -d "$src/cmd/$base" ]; then
      continue
    fi
    if [ ! -e "$src/$exec_dir/$base" ]; then
      rm -f "$f"
    fi
  done
fi
# The logo from the manifest ("icon": "icon.svg"), if any, ships with the
# plugin: the core serves it locally to the UI.
if [ -n "$icon_rel" ] && [ -f "$src/$icon_rel" ]; then
  mkdir -p "$dest/$(dirname "$icon_rel")"
  cp "$src/$icon_rel" "$dest/$icon_rel.new" && mv -f "$dest/$icon_rel.new" "$dest/$icon_rel"
fi
cp "$src/plugin.json" "$dest/plugin.json.new" && mv -f "$dest/plugin.json.new" "$dest/plugin.json"
trap - EXIT INT TERM
echo "installed $name in $dest"
