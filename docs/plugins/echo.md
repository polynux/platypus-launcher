# Echo: the test store plugin

Echo is a fake store. It has no network and installs tiny fake games, so you
can try every store feature in the app without a real account. The code is in
`plugins/echo`; `plugins/echo-b` is a second fake store that runs the same
program.

## How to try

1. Install echo. It is a Go program, so you need the Go toolchain once:

   ```sh
   plugins/install.sh echo
   ```

   Optional, for a second store with friends of its own (install echo first):

   ```sh
   plugins/install.sh echo-b
   ```

   Both land in `~/.local/share/hydra/plugins`.

2. Open **Plugins**, then **Review and approve** on Echo (and Echo B). Hydra
   runs a plugin only after you approve it. Then **Rescan** if they are not listed.

3. Log in: on Echo's card click **Log in**, and type the code `letmein`. Echo B
   has no login; it only has friends.

4. Find each feature:

   | Feature | Where it shows up |
   | --- | --- |
   | Library and install | The **Echo** source in the sidebar lists **Echo Tiny** and the other fake games. Open one to install and launch it. |
   | Friends | The friends block at the bottom of the sidebar. Click it for the modal: one section per store (**Echo**, and **Echo B** if installed), each with its own status selector and connection line. Turn a store's friends off and on from its section. |
   | Redeem a code | One **Redeem a code** button in the sidebar, shown while a ready store offers redeem. Pick the store and type `ECHO-OK`. |
   | Cloud saves | **Settings**, section **Cloud saves**: one row per store that offers cloud saves (off, ask, auto). A game page can override its store's choice. |
   | Plugin settings | On Echo's card in **Plugins**: "Enable overlay (example)", "Download region (example)", "Extra launch arguments (example)" and "Bandwidth limit, MB/s (example)". They are examples of what a store can ask for; Echo only keeps the values. |
   | Capabilities | Each plugin card's **capabilities** line. It is collapsed; click it to see what the plugin offers and which parts are on. |

## Knobs

Echo reads `ECHO_*` environment variables for tests (`plugins/echo/main.go`
lists them): for example `ECHO_SOCIAL_TICK` changes one friend every few
milliseconds, so you can watch the friends list update.
