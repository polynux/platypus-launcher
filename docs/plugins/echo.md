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
   has no login; it only has friends and chat.

4. Find each feature:

   | Feature | Where it shows up |
   | --- | --- |
   | Library and install | The **Echo** source in the sidebar lists **Echo Tiny** and the other fake games. Open one to install and launch it. |
   | Friends | The friends block at the bottom of the sidebar. Click it for the modal: one section per store (**Echo**, and **Echo B** if installed), each with its own status selector and connection line. Turn a store's friends off and on from its section. |
   | Chat | In the friends modal, **Message** next to a friend opens the conversation. Echo's friends with chat are Ana Ruiz, Bo Lindqvist, Gus Varga and **Ivy Chen**, the echo bot: she answers with what you wrote, reversed. Unread messages show as a badge on the friends block (summed over both stores). A new message raises a toast with the friend's name and the store (never the text). Each store has a **Message alerts** switch in its section. |
   | Family | In the friends modal, the **Family** part of a store's section lists its family members (Ana and Ben for Echo). The game **Echo Shared Racer** is lent by both: open its page and the **Copy** panel lists their copies. Ben's has the Track pack as well as the Soundtrack; pick the copy the game is played from. |
   | Redeem a code | One **Redeem a code** button in the sidebar, shown while a ready store offers redeem. Pick the store and type `ECHO-OK`. |
   | Cloud saves | **Settings**, section **Cloud saves**: one row per store that offers cloud saves (off, ask, auto). A game page can override its store's choice. |
   | Plugin settings | On Echo's card in **Plugins**: "Enable overlay (example)", "Download region (example)", "Extra launch arguments (example)" and "Bandwidth limit, MB/s (example)". They are examples of what a store can ask for; Echo only keeps the values. |
   | Capabilities | Each plugin card's **capabilities** line. It is collapsed; click it to see what the plugin offers and which parts are on. |

Echo B is the same program under its own `plugin.json`: its friends are a second
list (four people), and it has its own chat, so chat and family copies can be
tried against two stores at once.

## Knobs

Echo reads `ECHO_*` environment variables for tests (`plugins/echo/main.go`
lists them): for example `ECHO_SOCIAL_TICK` changes one friend every few
milliseconds, so you can watch the friends list update. `ECHO_CHAT_TICK` makes a
friend send a message every few milliseconds, so the unread badge and the toast
show up without anyone writing. `ECHO_FAMILY_IN_USE` and `ECHO_COPY_IN_USE`
mark a borrowed game or one member's copy as in use, so launching it shows the
refusal.
