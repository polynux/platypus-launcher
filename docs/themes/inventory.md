# Theme inventory: the launcher's parts

Input for the component catalog (ADR) and the layout and variant issues under #90. Refs #91.

## Scope and method

- Source: `apps/ui/src` on `main` (40caa67). Every line reference is to that tree.
- Not used: `tools/design-lab`. It is an older design. Nothing here relies on it.
- Read only. Nothing was run.
- Coverage: every `View` (`types.ts:224-239`), the app shell (`App.tsx`), the embedded store view, and every file in `components/` and `components/ds/`.
- Depth varies. Blocks marked **full** were read in full. Blocks marked **map** were read by heading, props, and core call sites only. Those files are listed in [Depth](#depth).

Short names used below: **Core call** means a `LauncherApi` call made by that block itself. **Required** means the block must stay reachable (see [Required parts](#required-parts)).

---

## 1. Views and shells

### 1.1 Views

| View (`types.ts`) | Rendered in `App.tsx` | Component | Depth |
|---|---|---|---|
| `dashboard` | `639-651` | `Dashboard` (`Dashboard.tsx:31`) | full |
| `library` (optional `collection`) | `652-679` | `LibraryView` (`LibraryView.tsx:371`) | full |
| `game` (`id`, `from`) | `680-715` | `GamePage` (`GamePage.tsx:174`). Missing game: `EmptyState`, `713-714` | full |
| `activity` | `716-717` | `ActivityView` (`ActivityView.tsx:8`) | full |
| `fixes` | `718-719` | `FixesView` (`FixesView.tsx:88`) | full |
| `runners` | `720-721` | `SystemView` (`SystemView.tsx:166`). The view is named `runners`, the component `SystemView` | full |
| `plugins` | `722-731` | `PluginsView` (`PluginsView.tsx:61`) | full |
| `storage` | `732-740` | `StorageView` (`StorageView.tsx:38`) | map |
| `downloads` | `741-759` | `DownloadsView` (`DownloadsView.tsx:146`) | full |
| `settings` | `760-761` | `LauncherSettingsView` (`LauncherSettingsView.tsx:28`) | full |
| `store` (`plugin`, `url?`) | `762-779` | `StoreView` (`StoreView.tsx:49`). Plugin not ready: `EmptyState`, `764-770` | full |

View state is one `useState` (`App.tsx:67`). Views get their data as props from `App`. There is no store.

### 1.2 App shell

| Part | Lines | Notes | Core calls | Required |
|---|---|---|---|---|
| Root and background | `App.tsx:784-785` | `app-wash` or `app-wash-game` class, chosen by view | none | yes |
| Sidebar | `App.tsx:786-816`; `Sidebar.tsx:77-326` | Nav, Tools group, collections, sources, store section, footer | none | yes |
| Main column (scroll root) | `App.tsx:818-829` | Scroll resets on view change (`App.tsx:116`) | none | yes |
| Migration banner | `App.tsx:819-827`; `MigrationNotice.tsx:13` | One-time notice, dismissable | `migrationNotice`, `dismissMigrationNotice` (`App.tsx:304, 619`) | no |
| Loading skeleton | `App.tsx:625`; `LibrarySkeleton.tsx:7` | Shown until first load | none | no |
| Backend-unreachable state | `App.tsx:626-637` | Dashboard and library only | none | no |
| Add game dialog | `App.tsx:831`; `AddGameModal.tsx:46` | Per-app dialog host | `detectGameExecutable` (`74`), `pickGameExecutable` (`89`), `pickFolder` (`102`, `112`). Saving goes through `App.addGame` (`App.tsx:515-532`, `addCustomGame` at `518`) | no |
| Cloud saves host | `App.tsx:833`; `CloudSavesHost.tsx:27` | Launch decisions, conflicts, offline notice | `launchGame`, `launchGameAnyway`, `launchGameViaClient` | no |
| Family warning host | `App.tsx:834`; `FamilyWarningHost.tsx:22` | Borrowed game about to be taken back | none | no |
| Terms host | `App.tsx:835`; `TermsHost.tsx:7` | Terms gate before install | via `lib/termsRequest.ts` | no |
| Redeem dialog | `App.tsx:837`; `RedeemDialog.tsx:20` | One entry for every store that offers redeem | `redeemCode` | no |
| Collections dialog | `App.tsx:838-846`; `CollectionsDialog.tsx:94` | Manager, or one game (`gameId`) | collections calls (`CollectionsDialog.tsx`) | no |
| Friends modal | `App.tsx:848-883`; `FriendsModal.tsx:58` | Chat panel replaces the list inside the modal | `setSocial*`, `restartSocial` via props | no |
| Toasts | `App.tsx:885`; `ds/sonner.tsx:35`; `lib/notify.ts:123` | Top-right, up to 3 visible (`ds/sonner.tsx:40`) | none | yes (capability feedback) |
| Tooltip provider | `App.tsx:784` | Delay 300 ms | none | yes |

There is no top bar component. The main column starts with the migration banner, then each view draws its own `p-7` header.

Job progress is not a shell part. `JobProgress` mounts in `GamePage` (`1087`, `1200`) and `MaintenanceTab` (`111`). The queue lives on the Downloads view and in the Sidebar count (`App.tsx:796`).

App-level core calls (the shell's own data): startup load `App.tsx:174-183` (`getGameSettings`, `getLibrary` or `rescanLibrary`, `getProtonVersions`, `listPlugins`, `getSessions`); `getTweaks`, `listCollections`, `migrationNotice` at `301-304`; `listJobs` at `443`; `getSessions` on focus at `366`; `invalidateLibrary` at `415`; `getLauncherSettings` at `287`.

App state every view depends on: `games` (`50`), `plugins` (`52`), `social` (`53`), `collections` (`57`), `jobs` (`61`), `sessions` (`76`), `tweaks` (`82`), `dismissedHero` (`83`), `activity` (`86`), `search`, `source`, `libraryMode`, `ownership` (`68-71`).

### 1.3 Embedded store view

`StoreView.tsx:49`. A native view is placed over the content area. The page itself cannot be styled by CSS or React. Only the toolbar and the frame are ours.

| Block | Lines | Notes |
|---|---|---|
| Toolbar: back, forward, reload, home, address strip (lock, host, loading), title | `170-214`; `ToolbarButton` `254` | Mirrors the native view's navigation state |
| Outside-domain notice | `215-232` (`data-testid="store-blocked"` at `217`) | Offers "open in browser". Navigation outside the domains never loads |
| Native area | `248` (`data-testid="store-area"`) | Its rect is sent to the shell |
| Bounds reporting | `67-72` (`getBoundingClientRect` at `71`, `setBounds` at `72`); `ResizeObserver` at `124-133`; scroll and resize listeners at `130-131` | A layout that moves the area must keep this in step |
| Open and close | `90` (`bridge.open`), `105` (`close`) | Bridge declared in `effect/bridge.ts:51-56` |

Entry points: Sidebar store section (`Sidebar.tsx:244-262`) and "open in store" on the game page (`GamePage.tsx:355-370`).

---

## 2. Regions and blocks per view

Each table row is a block a layout could order, group, hide or move. **Required** blocks are listed under [Required parts](#required-parts).

### 2.1 Dashboard (`Dashboard.tsx:31`)

| Block | Lines | Data it needs | Core call | Required |
|---|---|---|---|---|
| Hero: first game needing attention, with stat chips | `57-93` (chips `81-92`) | `games`, `dismissedHero` (props) | none | no |
| Welcome banner (greeting and counts) | `94-96`; `WelcomeBanner` `194-213` | counts | none | no |
| Empty library | `44-53` | none | none | no |
| Continue grid (8 tiles, most recent first) | `98-121` | `games` sorted (`39`) | none | no |
| Waiting on you (3 items, then "more in fixes") | `123-152` | `games` needing attention (`33`) | none | no |
| Recently added (first 5 sideloaded games) | `154-159` | `games` where `store === 'custom'` (`40`) | none | no |
| Activity (8 events, link to full log) | `163-165` (`ActivityPanel`, `ActivityFeed.tsx:43`) | `activity` | none | no |

### 2.2 Library (`LibraryView.tsx:371`)

| Block | Lines | Data it needs | Core call | Required |
|---|---|---|---|---|
| Title line: counts, scan time, collection chip, offline notice | `431-466`; collection chip `440-459` | counts, `collection`, `useNetState` (`427`) | none | no |
| Mode switch: cards or table | `467-470` (`SegRow`, `LibraryMode` at `26`) | `mode` | none | no |
| Add game button | `471-473` | none | none | no |
| Search box | `478-504` | `search` | none | no |
| Store chips (shown when there are two or more stores) | `506-531` (`sourceChips` `71-74`) | `games` | none | no |
| Filters toggle and panel: ownership, family, tier, status, collection, sort | `532-583` | filter state | none | no |
| Plugin warnings (logged out, unavailable, list failed) with login or plugins button | `586-619` | `warnings`, `plugins` | none | yes (see below) |
| Empty state | `621-628` | counts | none | no |
| Last played strip (3 hero tiles) | `631-661` | `played` (`418`) | none | no |
| All titles grid (virtualized) | `663-674` (`WindowedGrid`, `WindowedList.tsx:23`) | `rest` | none | no |
| Activity panel and recently added (6 games) | `676-684` | `activity`, `recentlyAdded` (`424`) | none | no |
| Table view (virtualized rows, sortable heads) | `686-705` (`WindowedRows`, `WindowedList.tsx:53`) | `tableRows` | none | no |

Cards and rows: `GameCard` at `297-341` (cover `2:3`, job overlay `314-320`, collections context menu `305`). `TableRow` at `343-369` (`onContextMenu` `350`, row height `h-11` at `351`). Constants: `CARD_MIN_WIDTH = 140` (`293`), `CARD_GAP` (`292`), `TABLE_COLS` (`274`), `TABLE_ROW_HEIGHT = 44` (`295`).

### 2.3 Game page (`GamePage.tsx:174`)

Hooks that load data (called by this page): `useDetails` (`118-172`, `getGameDetails` at `150`), `useCompat` (`lib/compat.ts`, `getGameCompat`), `useCrashHints` (`lib/useCrashHints.ts`, `getCrashHints`), `useAchievementProgress` (`ProgressAchievements.tsx:13`), `useInstallSize` (`lib/installSize.ts`).

| Block | Lines | Data it needs | Core call | Required |
|---|---|---|---|---|
| Back link (label from `view.from`) | `294-296` | `backLabel`, `onBack` | none | yes (navigation) |
| Hero: art, badges (tier, store, size, not installed, update, pinned, needs connection), title, playtime, last played | `298-322` | `game`, `details`, `plugin`, `sourceLogos` | none | no |
| Hero actions: Update, Install (or options), Play or Stop (`PlayButton.tsx:24`), Game settings, Open in store | `323-372` | `session`, `job`, `plugin` | `onPlay`/`onStop` via App | Install and Play: yes. Rest: no |
| Outside-folder alert | `376-395` | `game.outside_folder` | none | no (warning) |
| Install note (size, version, folder) | `397-425` | `installing`, `installSize` | `getInstallSize`, `getGameOptions` (via `lib/installSize.ts`) | no |
| Refreshing line | `427-429` | `refreshing` | none | no |
| Media strip (thumbnails, wheel scrolls sideways) | `433-435` (`MediaStrip` `570-653`; wheel `204-214`) | `media` | none | no |
| Lightbox | `436` (`Lightbox.tsx:34`) | `media` | none | no |
| About: description, requirements tabs, links, info finder and unlink | `438-446` (`AboutSection` `666-811`; `InfoFinder` `994-1081`; `InfoUnlink` `968-992`) | `details` | `searchArtOnline` (`1008`), `linkGameInfo` (`976`, `1020`) | no |
| Compatibility panel: verdict, facts, `CompatFacts` | `449-465` | `info`, `ratingBadge`, `fix` | none (`info` from `useCompat`) | no |
| Crash hints | `467` (`CrashHints.tsx:44`) | `crash` | `openCrashLog`, `runWinetricksVerb` | no |
| Status and notes | `469` (`StatusNotesSection` `834-948`) | `status`, `user_notes` | `setGameStatus` (`887`), `setGameNotes` (`858`) | no |
| Collections | `471` (`CollectionsSection` `950-966`) | `collections` | none | no |
| Installation (store, folder, job) | `473-475` (`InstallationPanel` `1157-1206`) | `plugin`, `job` | none | no |
| Copies (`CopyPicker`) | `477` (`CopyPicker.tsx:19`) | `game`, `plugin` | `setPreferredCopy` | no |
| Achievements: progress | `481-482` (`ProgressAchievements.tsx:32`) | `progress` | `getGameAchievements` | no |
| Achievements: list | `483-485` (`AchievementsSection` `1208-1253`) | `ach` | none | no |
| Session activity (6 events for this game) | `487-492` | `activity` filtered by title (`236`) | none | no |
| Game settings dialog | `493-514` | see 3.2 | several (see 3.2) | no |
| Install dialog and install options dialog | `515-533` | `plugin` | `installGameTo`; options calls in `InstallOptionsDialog` | no |

Other blocks in the file: `InstallButton` (`1083-1115`, shows `JobProgress` while a job runs), `GameHeading` (`1117`), `InstallSizeLine` (`1132`).

### 2.4 Activity (`ActivityView.tsx:8`)

| Block | Lines | Notes |
|---|---|---|
| Header (title, subtitle) | `66-71` | Same header pattern as other views |
| Feed panel (up to 50 events) | `72-74` (`ActivityFeed.tsx:20`) | No core calls |

### 2.5 Fixes (`FixesView.tsx:88`)

| Block | Lines | Notes |
|---|---|---|
| Header: intro with counts, about text, attention or all filter | `96-112` | Filter default `attention` (`90`) |
| Fix cards: cover, tier badge, source, reason, winetricks and DLL chips | `121-146` | Empty states `114-119` |

### 2.6 Runners (`SystemView.tsx:166`)

| Block | Lines | Notes |
|---|---|---|
| Header with rescan button (disabled while scanning) | `170-182` | Rescan calls `loadData('rescan')` (`App.tsx:721`) |
| Runner rows: name, custom and system badges, path, "found" | `189-206` | Empty state `184-187` |

### 2.7 Storage (`StorageView.tsx:38`) — map

| Block | Lines | Core call |
|---|---|---|
| Header with rescan | `93-103` | `refreshDiskUsage` (`63`) |
| Drive cards with legend | `106-113`; `DriveCard` `304`; `Legend` `342` | `getDiskUsage` (`54`) |
| Game list (size, open folder, remove, uninstall, move) | `116` onward; `RemoveFlow` `282`; `MoveDialog` use at `225` | `scanGameSize` (`79`), `openGameFolder` (`89`) |

### 2.8 Plugins (`PluginsView.tsx:61`)

| Block | Lines | Notes |
|---|---|---|
| Header: subtitle, plugins folder, rescan | `127-142` | Core: `openPluginsFolder`, `rescanPlugins` |
| Store plugin cards: identity, status badge, description, problem text | `150-174`, `156` | Card per store plugin |
| Capabilities | `176` (`PluginCapabilities.tsx:23`) | `offers` gates it at `220` |
| Plugin settings | `178` (`PluginSettingsPanel`, `PluginSettings.tsx:161`) | Only when `ready` |
| Missing requirements | `180-191` | Install first, rescan hint |
| Login row (checking, logged in as, logged out, log in or out) | `193-218` | `authStatus`, `logout`, `authBegin`, `authComplete` |
| Install folder row (change, reset, also allowed) | `220-241` | `choosePluginInstallFolder`, `setPluginInstallFolder` |
| Actions: approve or re-approve, disable, enable, retry | `243-281` | `approvePlugin`, `setPluginEnabled`, `retryPlugin` |
| Data plugin cards | `288-322` | No actions |
| Built-in list | `324-335` (`BUILT_IN`, `43-45`) | Static list |
| Approve dialog | `337-347` (`ApproveModal` `375`) | |
| Login dialog (QR or form) | `348-360` (`AuthModal` `465`; QR poll `454-462`, `492-551`) | |

### 2.9 Downloads (`DownloadsView.tsx:146`)

| Block | Lines | Notes |
|---|---|---|
| Header: subtitle, resume all, clear finished | `146` onward | `onClear` from `App.tsx:746-756` |
| Queue section (active jobs) | `146` onward | |
| Finished section | `146` onward | |
| Job row: title, state, detail, progress bar, actions | `64-144` (`JobRow`) | Actions: pause, resume, cancel, retry, discard |

### 2.10 Settings (`LauncherSettingsView.tsx:28`)

| Block | Lines | Own core calls | Required |
|---|---|---|---|
| Header | `58-62` | none | yes (reaching Settings) |
| Language (system or explicit) | `65`; `LanguageSetting` `124-183` | `getLauncherSettings`, `setLanguage` | no |
| Game folders | `67`; `GameFoldersSettings.tsx:16` | `getGameFolders`, `addGameFolder`, `removeGameFolder`, `makeDefaultGameFolder`, `chooseGameFolder` | no |
| Prefixes folder | `69`; `PrefixesFolderSetting` `377-440` | `setPrefixesFolder`, `pickFolder` | no |
| Cleanup (keep cancelled downloads for N days) | `71-103` | `setCancelCleanupDays` | no |
| Ratings (online lookups) | `105`; `RatingsSetting` `185-225` | `setRatingsOnline` | no |
| Artwork (online lookups) | `107`; `ArtworkSetting` `227-264` | `setArtworkOnline` | no |
| Game info (online lookups) | `109`; `GameInfoSetting` `266-303` | `setGameInfoOnline` | no |
| Chat notifications | `111`; `ChatNotificationsSetting` `305-339` | `setChatDesktopNotify` | no |
| Tray | `113`; `TraySetting` `341-375` | `setMinimizeToTray` | no |
| Save backups defaults | `115`; `BackupsSetting.tsx:18` | `getBackupSettings`, `setBackupSettings`, `refreshSaveManifest` | no |
| Cloud saves defaults | `117`; `CloudSavesDefaults` (`CloudSavesSetting.tsx:15`) | `getCloudSettings`, `setCloudPluginMode` | no |

Each section loads and saves its own settings. `getLauncherSettings` appears at `37`, `132`, `190`, and in other sections (eight call sites in the file). The sections share nothing but `onNotify`.

---

## 3. Containers

Anything that presents a group of content and could vary in presentation.

### 3.1 Container primitives

| Container | Lines | Presentation today | Variant switch today | Opened from |
|---|---|---|---|---|
| `Modal` | `Modal.tsx:28` | Centred dialog on Radix `Dialog`. Esc and outside click close unless `dismissable` is false (`34-35`) | `size`: `sm` 440, `md` 560, `lg` 720, `xl` 920 (`7-12`) | Most dialogs (below) |
| `ModalOrInline` | `Modal.tsx:45-46` | Dialog, or inline children when `embedded` | `embedded` prop. Only two users: `VersionsDialog.tsx:169`, `InstallOptionsDialog.tsx:117` | Game settings tabs and standalone |
| `Panel` | `ui.tsx:19` | Rounded card, `bg-white/[0.045]`, ring | `as`, `className` | Dashboard, library, game page, plugins, downloads |
| Settings section | `LauncherSettingsView.tsx:20` (`SECTION`) | Hairline top border, no card. Comment says "not a card" | none | Settings view |
| Setting group | `SettingsModal.tsx:83` (`SettingGroup`) | Section with optional heading | `heading` prop | Game settings general tab |
| Tab panel | `GameSettingsModal.tsx:147-168` | Section with `h3` and tab ARIA | none | Game settings |
| Sidebar Tools group | `Sidebar.tsx:168-187` | Collapsible group, opens on active child, remembered per machine | open state in localStorage (`TOOLS_OPEN_KEY`, `66`) | Sidebar |

### 3.2 Game settings dialog (`GameSettingsModal.tsx:59`)

Presentation: `Modal` at size `xl`, fixed height `h-[min(43rem,calc(100dvh-6rem))]` (`92`). Tabs are a vertical list at container width `@2xl` (`110-115`), otherwise a wrapping row. Arrow keys move between tabs (`126-134`).

Tab list is data-driven: `lib/gameSettingsTabs.ts:8-9` (union), `35-51` (which tabs appear and when).

| Tab | Shown when (`gameSettingsTabs.ts`) | Body (`GameSettingsModal.tsx`) | Core calls in the body |
|---|---|---|---|
| General | always (`36`) | `147-161`: `CustomGameYear` `180-240` (sideloaded only), `GeneralSettings` (`SettingsModal.tsx:156`), `LegalSection` (`160`) | `setCustomGameYear`, runner and settings calls (map) |
| Cloud saves | plugin, installed, `cloud-saves` offered (`39`) | `247` (`CloudSavesPanel`, `framed={false}`) | map |
| Versions | plugin, installable, `versions` offered (`40`) | `249` (`VersionsDialog`, embedded) | `getGameVersions`, `installGameVersion`, `pinGameVersion`, `unpinGameVersion`, `unlockVersionChannel`, `updateGame` |
| Options | plugin, installed, installable, `install-options` offered (`41`) | `251` (`InstallOptionsDialog`, embedded, mode `update`) | `getGameOptions`, `updateGameWithOptions` |
| Plugin settings | plugin, `settings` offered (`42`) | `253` (`GameSettingsPanel`, `framed={false}`) | `getGameSettingsFields`, `saveGameSettingsFields` |
| Tweaks | installed, tweaks exist (`43`) | `254-267` (`TweaksPanel`, `framed={false}`) | none |
| Customization | always (`44`) | `269` (`CustomizationTab`) | `getCustomization`, `setCustomTitle`, art calls |
| Maintenance | always (`49`) | `271-272` (`MaintenanceTab`) | see 3.3 |

Shared draft: `useGameSettingsDraft` (`SettingsModal.tsx:118`) keeps General's unsaved edits across tabs.

### 3.3 Maintenance tab (`MaintenanceTab.tsx:29`)

| Row or block | Lines | Notes | Core calls |
|---|---|---|---|
| Remove (disabled while running) | `123-124`; `RemoveFirstRow` `272` | Sideloaded only in practice | `checkRemove` (`263`) |
| Verify (opens `VerifyDialog`) | `171-173`; dialog `234` | | `verifyGame`, `repairGame` (in `VerifyDialog.tsx`) |
| Move (opens `MoveDialog`) | `178-180`; dialog `236` | | `checkMove`, `moveGame` (in `MoveDialog.tsx`) |
| Uninstall, with confirm and backup check | `184-230` | Confirm panel `RemoveConfirmPanel` `332-449` | `uninstallGameWithoutBackup` (`101`), `getAutoBackupState` (`80`) |
| Save backups | `131`, `147`, `168` (`SaveBackupsSection`) | Mounted in three branches | `getSavePaths` (`79`), backup calls (in `SaveBackupsSection.tsx`) |

Row shell: `Row` (`MaintenanceTab.tsx:480-482`), a `section` with a hairline bottom border. Save-backup data for the tab is prefetched when the dialog opens (`maintenancePrefetch.tsx:46-76`, `useMaintenancePrefetchSource` used at `GameSettingsModal.tsx:85`).

### 3.4 Other dialogs and panels

| Dialog or panel | File:line | Size or chrome | Opened from |
|---|---|---|---|
| Install | `InstallDialog.tsx:29`, `Modal` `md` (`54`) | Dialog | `GamePage.tsx:516` |
| Install options (install) | `InstallOptionsDialog.tsx:39`, `ModalOrInline` (`117`) | Dialog | `GamePage.tsx:524-533` |
| Versions | `VersionsDialog.tsx:61`, `ModalOrInline` `lg` (`169`) | Dialog or tab | Game settings tab |
| Verify | `VerifyDialog.tsx:22`, `md` (`94`) | Dialog | Maintenance |
| Move | `MoveDialog.tsx:26`, `md` (`67`) | Dialog | Maintenance, Storage (`StorageView.tsx:225`) |
| Collections | `CollectionsDialog.tsx:94`, `md` (`194`) | Dialog; reorder by buttons (`245-249`) | App, library, game page, Sidebar |
| Copy compare | `CopyCompareDialog.tsx:23`, `lg` (`38`) | Dialog, table | `CopyPicker.tsx:91` |
| Copy picker | `CopyPicker.tsx:19` | Panel (`43`) | Game page |
| Add game | `AddGameModal.tsx:46`, `md` (`169`) | Dialog | App |
| Redeem | `RedeemDialog.tsx:20`, default `sm` (`45`) | Dialog | App |
| Terms | `TermsDialog.tsx:29`, `md` (`49`) | Dialog, not dismissable while busy | `TermsHost` |
| Cloud conflict | `CloudSavesConflict.tsx:31`, `lg` (`54`) | Dialog | `CloudSavesHost.tsx:83`, `CloudSavesPanel.tsx:228` |
| Cloud host (decision, client running, offline) | `CloudSavesHost.tsx:27`; dialogs `70`, `103`, `116` | Dialog, default size | App |
| Cloud saves panel | `CloudSavesPanel.tsx:26`; `framed` `34`, frame `126-128`, `CloudFrame` `242` | Framed or bare | Game settings tab, game page |
| Plugin settings panel | `PluginSettings.tsx:75`, `framed` `156`; `GameSettingsPanel` `176` | Framed or bare | Plugins view, game settings tab |
| Tweaks panel | `TweaksPanel.tsx:11`, `framed` `17`, `83` | Framed or bare | Game settings tab |
| Approve plugin | `PluginsView.tsx:375`, `md` (`378`) | Dialog | Plugins view |
| Login (plugin) | `PluginsView.tsx:465`, `md` (`577`) | Dialog, QR or form | Plugins view |
| Friends | `FriendsModal.tsx:58`, `lg` (`63`, `69`) | Dialog; chat replaces the list (`59-67`) | App |
| Chat | `ChatPanel.tsx:31` | Slot in the friends modal, `70vh` (`FriendsModal.tsx:64`) | Friends modal |
| Lightbox | `Lightbox.tsx:34` | Overlay with own keys (`66-69`) | Game page |
| Legal | `LegalSection.tsx:16` | Section (`44`) | Game settings general tab |

No sheet or drawer exists in the app. A slide-in variant would be new.

**Dead container:** `SettingsModal` (`SettingsModal.tsx:762-770`) is a standalone game settings dialog. Only the test files import it (`components/__tests__/prefix-sharing.test.tsx:44` and others). No production code mounts it. `GeneralSettings` is the live part.

---

## 4. Recurring components

### 4.1 Layout and surfaces

| Component | Lines | Used by | Variants implied today |
|---|---|---|---|
| `Panel` | `ui.tsx:19` | Most views | Background (`bg-white/[0.04]`, `[0.045]`, `[0.05]`), flex column |
| `cardClass` | `ui.tsx:23` | Dashboard tiles (`Dashboard.tsx:106`), library cards | Hover lift and ring |
| `Cover` | `ui.tsx:169` | Everywhere art appears | Aspect: `3:4` (dashboard, `108`), `2:3` (library, `GameCard`), `16:9` banner (`GamePage.tsx:299`), last-played tile `h-[210px]` (`LibraryView.tsx:637`), 8px list thumb (`Dashboard.tsx:222`), 28px table thumb (`LibraryView.tsx`, `TableRow`) |
| `Cover` fallback | `ui.tsx:198-203` | Same | Typographic tile when no art |
| `EmptyState` | `ui.tsx:147` | Most views | Dashed border, mono label |
| Section chrome | `LauncherSettingsView.tsx:20`, `SettingsModal.tsx:83`, `GameSettingsModal.tsx:165`, `MaintenanceTab.tsx:480-482` | Settings, game settings | Four patterns (hairline, grouped, `h3` tab title, row) |
| View header | `ActivityView.tsx:66-71`, `FixesView.tsx:96-112`, `SystemView.tsx:170-182`, `StorageView.tsx:93-103`, `PluginsView.tsx:127-142`, `DownloadsView.tsx:146`, `LibraryView.tsx:431-584`, `LauncherSettingsView.tsx:58-62` | Every view | Title, subtitle, right-side actions. Not extracted: each file writes it out |
| `SectionTitle` | `ui.tsx:30` | No importer | Exported but unused |
| `MonoLabel` | `ui.tsx:26` | Many | Kicker label, `h2` or `div` |
| `SegRow` | `ui.tsx:53` | Library (`469`, `508`, `548`, `553`), Fixes (`106`) | Segmented control, `ToggleGroup` |
| `TogglePill` | `ui.tsx:73` | Rows with a switch-like state | Visual only |

### 4.2 Game representations

| Component | Lines | Used by | Notes |
|---|---|---|---|
| Dashboard tile | `Dashboard.tsx:104-120` | Dashboard continue grid | `cover-grid` class, `3:4`, `hideNative` compat chip |
| Library card | `LibraryView.tsx:297-341` (`GameCard`) | Library cards mode | `2:3`, grid min 140, job overlay `314-320`, status and not-installed badges, context menu `305` |
| Table row | `LibraryView.tsx:343-369` (`TableRow`) | Library table mode | 44px height, columns `274` |
| Last-played tile | `LibraryView.tsx:631-661` | Library cards mode | Hero tile, `210px` |
| Recent list row | `Dashboard.tsx:215-233` (`RecentList`) | Dashboard (`157`), library (`681`) | Imported from `Dashboard` into `LibraryView.tsx:7` |
| Fix card | `FixesView.tsx:121-146` | Fixes | Tier badge, source badge, chips |
| Game card body and compat | `CompatBadges.tsx:139` (`GameCardBody`), `99` (`CompatChip`) | Dashboard, library cards | `hideNative` (`99`, `101`, `153`) |
| Hero | `Dashboard.tsx:57-93`, `GamePage.tsx:298-322`, `LibraryView.tsx:631-661` | Dashboard, game page, library | Three hero forms, each with its own gradient |
| Plugin logo | `PluginLogo.tsx:40` | Sidebar, library chips, game page, plugins | Image or monogram tile |

### 4.3 Status, badges and progress

| Component | Lines | Used by | Notes |
|---|---|---|---|
| `TierBadge` | `ui.tsx:83` | Dashboard (`140`), fixes (`132`), library (`642`), game page (`304`) | Maps tier to badge variant; unknown tiers become `silver` |
| `TweakDot` | `ui.tsx:110` | Dashboard, fixes, library cards | Its `title` is English and not translated (`ui.tsx:113`) |
| `Badge` | `ds/badge.tsx:25` | Everywhere | Variants: neutral, accent, info, ok, gold, bronze and tiers |
| `Button` | `ds/button.tsx:38` | Everywhere | Variants: primary, neutral, ghost, link, danger; sizes; `size="none"` |
| `JobProgress` | `JobProgress.tsx:13` | Game page `InstallButton` (`1087`), `InstallationPanel` (`1200`), Maintenance (`111`) | Popover-sized (`w-72`, `19`) with cancel |
| `JobRow` | `DownloadsView.tsx:64` | Downloads queue and finished | Full row with pause, resume, cancel, retry, discard |
| Card job overlay | `LibraryView.tsx:314-320` | Library card | Inline bar on the cover |
| Progress achievements | `ProgressAchievements.tsx:32` | Game page | Card per achievement |

Four progress shapes exist (popover, row, card overlay, achievement card). They share no component except `JobProgress` and `JobRow` lineage.

### 4.4 Rows and lists

| Component | Lines | Used by | Notes |
|---|---|---|---|
| `WindowedGrid` | `WindowedList.tsx:23` | Library cards | Virtualized by width |
| `WindowedRows` | `WindowedList.tsx:53` | Library table | Virtualized by row height |
| `ActivityFeed` | `ActivityFeed.tsx:20` | Activity view, game page (`490`) | `limit` prop |
| `ActivityPanel` | `ActivityFeed.tsx:43` | Dashboard (`164`), library (`677`) | Panel with "full log" link |
| Runner row | `SystemView.tsx:189-206` | Runners | Badges and path |
| Plugin card | `PluginsView.tsx:156` | Plugins | Largest row type |
| Friend row | `FriendsModal.tsx:205`, `FriendRowMain` `231` | Friends modal | Avatar with dot |
| Storage row | `StorageView.tsx` (list from `116`) | Storage | Remove and move via `RemoveFlow` (`282`) |
| Friends summary | `FriendsSummary.tsx:19` | Sidebar footer (`Sidebar.tsx:284`) | Count and unread badge; opens the friends modal |
| Legal list | `LegalSection.tsx:24` | Game settings general | |

Lists here are plain `ul` and `li`, except the two windowed ones.

### 4.5 Forms and controls

`ds/` holds the shared controls: button, input, textarea, select, switch, toggle-group, number-field, label, badge, dialog, tooltip, scroll-area, separator, sonner. `ui.tsx:10-15` says so. `NumberField` has its own key handler (`ds/number-field.tsx:70`).

---

## 5. Interaction patterns

### 5.1 Navigation

- One `View` state (`App.tsx:67`). Views change through `setView`.
- Back target: `openGame` records the origin view (`App.tsx:605-608`). `GamePage` back link uses it (`GamePage.tsx:686-687`). Deleting a game returns to its origin (`App.tsx:549`).
- Scroll resets to top on view change (`App.tsx:116`).
- Sidebar nav items set `aria-current` (`Sidebar.tsx:119`, `251`, `275`).
- Sidebar search: Enter opens the library, Escape clears (`Sidebar.tsx:154-157`). Typing does not navigate.

### 5.2 Keyboard and focus handlers

Own keyboard handlers, found by search:

| # | Where | What it does |
|---|---|---|
| 1 | `Sidebar.tsx:154-157` | Search: Enter opens library, Escape clears |
| 2 | `GameSettingsModal.tsx:126-134` | Tab list: arrow keys move and focus the next tab |
| 3 | `Lightbox.tsx:66-69` | Escape closes, arrows move (stops propagation on Escape) |
| 4 | `ChatPanel.tsx:125-126` | Enter sends, Shift+Enter newline |
| 5 | `PluginSettings.tsx:65` | Enter commits a draft field |
| 6 | `ds/number-field.tsx:70` | Step keys |
| 7 | `MaintenanceTab.tsx:94-95` | Window keydown (capture) while the uninstall confirm waits: Escape backs out of the confirm only |
| 8 | `Modal.tsx:34-35` | Escape and outside click ignored when not dismissable |

Not keyboard: window `focus` (`App.tsx:385`), `scroll` and `resize` in `lib/windowing.ts:89-90`, `focus` in `lib/pauseWhileHidden.ts:37`, `StoreView.tsx:130-131`. The brief's "8 handlers" matches the list above when item 8 is counted.

Focus: dialogs trap and restore focus through Radix (`Modal.tsx:26`). Tab focus moves on arrow keys (`GameSettingsModal.tsx:133`). The maintenance cancel button takes focus (`MaintenanceTab.tsx:211`, `autoFocus`).

### 5.3 Reorder

No drag and drop exists (no `draggable`, no drop handlers). Collections reorder with Move up and Move down buttons (`CollectionsDialog.tsx:245-249`, `reorderCollections` at `162`).

### 5.4 Windowed lists

`WindowedGrid` and `WindowedRows` (`WindowedList.tsx:23`, `53`) virtualize library cards and rows. They use the scroll and resize listeners in `lib/windowing.ts:89-90`.

### 5.5 Lightbox

`Lightbox.tsx:34`. Opened from the media strip (`GamePage.tsx:436`). Keys at `66-69`. The media list combines screenshots and videos (`toMediaItems`, `21`).

### 5.6 Other gestures

- Context menu on a library card or row opens collections (`LibraryView.tsx:305`, `350`).
- Horizontal wheel scrolls the media strip (`GamePage.tsx:204-214`).
- Live events drive state: sessions (`App.tsx:333`), jobs (`App.tsx:444`), playtime (`App.tsx:477`), library art (`App.tsx:481`).

---

## 6. Proposed grouping: first catalog draft

Names are proposals. Axes are the things a theme could choose between. Built-in choice is the current behaviour.

### 6.1 Components and their axes

| Catalog name | Replaces (today) | Variant axes | Built-in choice |
|---|---|---|---|
| `Page` | Each view's `p-7` root | Padding; header slot | Current |
| `ViewHeader` | Eight hand-written headers (section 4.1) | Title, subtitle, actions position | Right-aligned actions |
| `Section` | Four section patterns (4.1) | Chrome: `framed`, `hairline`, `plain`; heading shown or hidden | `hairline` in Settings; `framed` on game page |
| `Dialog` | `Modal` (`Modal.tsx:28`) | Size: `sm`, `md`, `lg`, `xl`. Presentation: `dialog`, `inline` (existing `ModalOrInline`). Behaviour: `dismissable` | Size per dialog as in section 3.4 |
| `Tabs` | Game settings tab list | Orientation: `vertical` at container width, or `horizontal`. Tab set comes from `settingsTabs` | Vertical wide, horizontal narrow |
| `GameTile` | Dashboard tile, library card | Aspect `3:4` or `2:3`. Footer: compat on or off, `hideNative`. Job overlay on or off. Min width or fixed grid | Dashboard `3:4`, library `2:3` |
| `GameRow` | Library table row, `RecentList` row | Column set; compact or full | Table columns `274`; compact list |
| `Hero` | Dashboard hero, game page hero, last-played tile | Height; stat chips on or off; action set | Dashboard `290px`, game `min 280px` |
| `ActivityFeed` | `ActivityFeed`, `ActivityPanel` | `limit`; "full log" link on or off | `8`, `6`, `50` by view |
| `JobProgress` | Popover, `JobRow`, card overlay, install button | `compact` (popover), `row`, `overlay` | As today |
| `Badge`, `TierBadge`, `TweakDot` | Same | Tone; tier labels | As today |
| `Button` | `ds/button.tsx` | Variant, size (already in `ds`) | As today |
| `SegControl` | `SegRow` | Options | As today |
| `EmptyState` | `ui.tsx:147` | Icon on or off | As today |
| `ListRow` | Runner, fix, friend, storage rows | Leading media, trailing actions | As today |
| `StoreFrame` | `StoreView.tsx:49` | Toolbar on or off. Area is native and fixed | Toolbar with address strip |

Tokens (colours, fonts, radii) are out of scope here. The epic covers them.

### 6.2 Blocks per view: what a theme may change

"Reorder", "hide" and "move" are the three things a layout can do. "Required" means the block cannot be hidden (see the next section).

| View | Reorder | Hide | Move to another container | Coupling that blocks the split |
|---|---|---|---|---|
| Shell (Sidebar, main) | Sidebar sections; collection list | Store section; Tools group | Sidebar to top bar (new) | Sidebar state in `App` (`Sidebar.tsx:77`); tools open state in localStorage |
| Dashboard | All seven blocks | Hero, welcome, waiting, recently added, activity | Activity to a side column | `dismissedHero` in `App` (`83`) |
| Library | Last played, all titles, activity, recent, filter groups | Last played, activity, recent, filters, store chips | Filters to a sidebar | Filter state in `LibraryView` (`373-391`); `RecentList` imported from `Dashboard` (`LibraryView.tsx:7`) |
| Game page | Media, about, compat, status, collections, installation, copies, achievements, activity | Most panels (see 2.3) | Compat and facts to hero; settings to a sheet | Local data hooks (`GamePage.tsx:118-172`); modal state (`177-184`) |
| Game settings dialog | Tab order (data-driven) | Any tab with `settingsTabs` | Tabs to left or top | Shared draft across tabs (`SettingsModal.tsx:118`) |
| Activity, Fixes, Runners, Downloads | Header and body only | Header actions | None | Downloads actions use `App` jobs (`App.tsx:742-758`) |
| Storage | Drives, games list | Drives section | Drives to header | Own core calls (`StorageView.tsx:54-89`) |
| Plugins | Store cards, data cards, built-in list | Built-in list | None | Own core calls and auth cache (`PluginsView.tsx:57-120`) |
| Settings | Sections | Each section | None | Each section loads its own settings |
| Store | Toolbar parts | Toolbar buttons (not the area) | None | Native rect (`StoreView.tsx:67-72`) |

### 6.3 Required parts

These cannot be hidden. The reason is the product rule that the UI shows real state with no placeholder (`PRODUCT.md:61`), plus the need to reach Settings and Plugins.

| Part | Where | Why |
|---|---|---|
| Sidebar Settings entry | `Sidebar.tsx:272-283` | Only route to settings |
| Sidebar Plugins entry | `Sidebar.tsx:103` (in Tools) | Only route to plugin approval and login |
| Plugin warnings in the library | `LibraryView.tsx:586-619` | Shows logged-out or unavailable sources and the login button |
| Plugin login row | `PluginsView.tsx:193-218` | Only place to log in to a store |
| Install and Play on the game page | `GamePage.tsx:336-346` | Core action |
| Back link | `GamePage.tsx:294-296` | Navigation |
| Toasts | `ds/sonner.tsx:35` | Action feedback |

The outside-folder alert (`GamePage.tsx:376-395`) is a warning, not a route. It should stay visible, but a maintainer should decide whether it counts as required.

### 6.4 Coupling that blocks each split

- **Shared state in `App`.** Every view gets its data as props (`App.tsx:638-779`). Moving a block into another container means passing the same props.
- **Local data calls.** Blocks that call core themselves can't move without their data: `GamePage` (`118-172`, `1008`, `976`, `1020`, `858`, `887`), `StorageView` (`54-89`), `PluginsView` (`61-120`), `LauncherSettingsView` (eight `getLauncherSettings` sites), `MaintenanceTab` (`79`, `80`, `101`, `263`, `304`), `CustomizationTab`.
- **Helpers imported across views.** `LibraryView` imports `RecentList` from `Dashboard` (`LibraryView.tsx:7`) and `ActivityPanel` from `ActivityFeed` (`6`). `JobProgress` imports `jobKindKey` from `DownloadsView` (`JobProgress.tsx:10`). Fix a shared piece in one place and the other view moves too.
- **Duplicates of `lib/games`.** `Dashboard.tsx:172` (`attentionReason`) and `Dashboard.tsx:183` (`storeLabel`) duplicate `lib/games.ts:14` and `lib/games.ts:41`. The comment at `Dashboard.tsx:180-182` says the dashboard reads its own copy on purpose, for translated labels.
- **Dead code.** `SettingsModal` (`SettingsModal.tsx:762-770`) has no production caller. `SectionTitle` (`ui.tsx:30`) has no importer.
- **Store rect.** The native store area must re-report its bounds after any layout change (`StoreView.tsx:67-72`, `124-133`).
- **i18n gaps that block translated themes.** `TweakDot` title (`ui.tsx:113`), Sidebar source labels (`Sidebar.tsx:59-63`), Sidebar version string (`Sidebar.tsx:321`), game-page fact "Platypus" literal (`GamePage.tsx:263`), library offline default (`LibraryView.tsx:463`).
- **Fixed sizes.** Sidebar width `w-56` (`Sidebar.tsx:137`). Library card minimum `CARD_MIN_WIDTH` (`LibraryView.tsx:293`). Dashboard hero `h-[290px]` (`Dashboard.tsx:59`).

---

## 7. Sizing and order

### 7.1 Size per view

S = a few blocks, no shared state. M = several blocks or one dialog. L = many blocks, own data, or a file over 600 lines.

| Part | Size | Why |
|---|---|---|
| Shell (Sidebar, main column, hosts) | M | Sidebar 326 lines; many hosts |
| Settings view | S | Eleven sections, no shared state, all simple |
| Game settings dialog (tabs) | M | Tabs already data-driven; shared draft |
| Dashboard | M | Seven blocks, all from props |
| Activity | S | One feed |
| Fixes | S | One card type |
| Runners | S | One row type |
| Downloads | S | One row type, props only |
| Storage | M | Own core calls, remove flow |
| Library | L | Two modes, filters, warnings, virtualized |
| Game page | L | 1,277 lines, many own data hooks; split the file before theming |
| Plugins | L | 638 lines, two card kinds, QR login polling |
| Store view | M | Native overlay, bridge |

### 7.2 Order

1. **Settings.** Confirm, with a caveat. It is a good first testbed for the mechanics: the `Section` and `Dialog` axes, the `framed` and `embedded` flags, and token use. It has no game data, and the eleven sections don't interact. The caveat: it tests the least layout. Its sections are independent, so it cannot prove that reordering blocks works, which is most of what a theme needs. Use Settings to prove the catalog and the variant mechanics. Use the Dashboard to prove layouts.
2. **Game settings dialog.** It already has the tab list as data and a dialog-or-inline switch (`ModalOrInline`). It tests `Tabs` and `Dialog` together.
3. **Dashboard.** First real layout test. Seven blocks, all from props.
4. **Activity, Fixes, Runners, Downloads.** Cheap. They prove `ViewHeader`, `EmptyState`, `ListRow` and `JobRow`.
5. **Library.** Tests `GameTile` and `GameRow` variants and virtualized lists.
6. **Storage, then Plugins.** Each has its own core calls, so they need the data split first.
7. **Game page.** Split `GamePage.tsx` into its blocks first. Then theme.
8. **Store view.** Last. Its area is native and its bounds need care.

---

## Depth

Files read in full: `App.tsx`, `Sidebar.tsx`, `Dashboard.tsx`, `ActivityFeed.tsx`, `ActivityView.tsx`, `FixesView.tsx`, `SystemView.tsx`, `ui.tsx`, `Modal.tsx`, `GameSettingsModal.tsx`, `lib/gameSettingsTabs.ts`, `LibraryView.tsx` (main body and card and row), `DownloadsView.tsx` (row and layout), `JobProgress.tsx`, `StoreView.tsx` (structure and bounds), `LauncherSettingsView.tsx` (structure), `PluginsView.tsx` (`120-370`).

Mapped by heading, props and call sites only (**map**): `GamePage.tsx` sections below line 540, `SettingsModal.tsx` (`GeneralSettings` body `156-760`), `StorageView.tsx`, `MaintenanceTab.tsx` (uninstall body), `VersionsDialog.tsx`, `InstallOptionsDialog.tsx`, `CloudSavesPanel.tsx`, `CloudSavesHost.tsx`, `CloudSavesConflict.tsx`, `ChatPanel.tsx` (beyond the key handler), `FriendsModal.tsx` (`StoreSection`, `FriendGroup`), `AddGameModal.tsx`, `CompatBadges.tsx`, `CustomizationTab.tsx`, `PluginSettings.tsx`, `CollectionsDialog.tsx`, `SaveBackupsSection.tsx`, `LauncherSettingsView.tsx` section bodies (`124-440`), `CopyCompareDialog.tsx`, `CopyPicker.tsx`, `CrashHints.tsx`, `TweaksPanel.tsx`, `ProgressAchievements.tsx`, `RunnerChoices.tsx`, `InstallLocationPicker.tsx`, `SteamAppIdField.tsx`, `VerifyDialog.tsx`, `MoveDialog.tsx`, `RedeemDialog.tsx`, `TermsDialog.tsx`, `FamiliesPanel.tsx`, `FamilyWarningHost.tsx`, `ExternalLink.tsx`, `ErrorNotice.tsx`, `Lightbox.tsx` (presentation), `WindowedList.tsx` (internals), `ds/*` (listed, not read in full).

Every file in `components/` is accounted for in the tables above. Map-depth blocks are where a later pass should go first.

---

## Open questions

1. Which game-page blocks are "required"? The outside-folder alert is the clearest case (see 6.3).
2. Should the `SettingsModal` standalone dialog be removed? Its tests would move to `GeneralSettings`.
3. Is `SectionTitle` (`ui.tsx:30`) meant for the view header or the sections? Either way it is unused.
4. The brief lists a `Hero` in the Dashboard as the first layout block. Should the Dashboard hero and the game-page hero share one catalog component, or stay two?
