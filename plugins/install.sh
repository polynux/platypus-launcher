#!/bin/sh
# Installs a plugin from this folder into ~/.local/share/hydra/plugins/<name>.
# Hydra then lists it as unapproved until you approve it in the Plugins view.
#
#   plugins/install.sh echo             # fake store, for trying the plugin UI
#   plugins/install.sh epic-legendary   # needs legendary: pipx install legendary-gl
#
# Go plugins are built; other plugins are copied as they are. Files are
# replaced by rename, so a running plugin keeps its old binary until restart
# (and Hydra asks you to approve the new one).
set -eu
name=${1:?usage: plugins/install.sh <plugin>}
here=$(cd "$(dirname "$0")" && pwd)
src=$here/$name
dest=${HYDRA_PLUGINS:-$HOME/.local/share/hydra/plugins}/$name
[ -f "$src/plugin.json" ] || { echo "no plugin named $name in $here" >&2; exit 1; }
exec_rel=$(sed -n 's/.*"exec": *"\([^"]*\)".*/\1/p' "$src/plugin.json")
[ -n "$exec_rel" ] || { echo "$name/plugin.json has no exec" >&2; exit 1; }

mkdir -p "$dest/$(dirname "$exec_rel")"
tmp="$dest/$exec_rel.new"
if [ -f "$src/main.go" ]; then
  (cd "$here/.." && go build -o "$tmp" "./plugins/$name")
else
  cp "$src/$exec_rel" "$tmp"
  chmod 755 "$tmp"
fi
mv -f "$tmp" "$dest/$exec_rel"
# Extra module files sitting next to the exec (e.g. python modules it
# imports) are part of the plugin; replace them by rename too so a running
# plugin keeps its old ones.
exec_dir=$(dirname "$exec_rel")
for f in "$src/$exec_dir"/*; do
  base=$(basename "$f")
  case "$base" in
    "$(basename "$exec_rel")"|__pycache__) continue ;;
    *.pyc) continue ;;
  esac
  [ -f "$f" ] || continue
  mv -f "$f" "$dest/$exec_dir/$base"
done
cp "$src/plugin.json" "$dest/plugin.json.new" && mv -f "$dest/plugin.json.new" "$dest/plugin.json"
echo "installed $name in $dest"
