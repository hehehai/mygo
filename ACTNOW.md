# ActNow's changes to MyGo

Base: upstream `v0.10.0`. Each change is one commit (or a short series) whose message names its origin: the app and patch file it replaces (`<App> third_party/mygo-patches/NNNN-name.patch`).

## Sync with upstream

    git fetch upstream && git push origin upstream/main:upstream
    git rebase --onto <new upstream tag> <old upstream tag> main   # on a branch, re-test, then fast-forward main
    git tag vX.Y.Z-actnow.1 && git push origin vX.Y.Z-actnow.1     # tags are immutable once fetched through the Go proxy

## Changes

Commits on `main` since upstream v0.10.0 (merges left out). Patches upstream had already fixed, or that the apps now do with upstream's API, are not carried; the apps' migration notes list them.

| Commit | Change | Origin |
|---|---|---|
| `9b5be9af` | ui: where files were dropped (DroppedFilesAt, FileDragAt; Tester.DragFiles) | Act third_party/mygo-patches/0007-file-drop-position.patch Origin: Inboxly third_party/mygo-patches/0007-file-drop-position.patch Origin: CaptionCraft third_party/mygo-patches/0007-file-drop-position.patch Origin: Paralign third_party/mygo-patches/0007-file-drop-position.patch |
| `28d91c38` | ui: Theme.SetAccentFill, a fill white text reads on for a mid-tone accent | Act third_party/mygo-patches/0051-accent-fill.patch |
| `27c5e2c3` | ui: regenerate api_gen.go (PopoverMenu doc drifted upstream) | (fork maintenance) |
| `86df670a` | darwin: take Finder's ⌘-drags (generic operation); Tester data drags | Act third_party/mygo-patches/0030-generic-drag-and-tester-data-drags.patch |
| `4196eb13` | ui: Theme.DialogRadius, PopupRadius, ToastRadius and TooltipRadius | Act third_party/mygo-patches/0063-theme-popup-radii.patch Origin: VoxtMini third_party/mygo-patches/0008-panel-radius.patch |
| `db9273db` | watch: a watch holds its directory open only while it looks | Act third_party/mygo-patches/0033-watch-holds-no-directory.patch |
| `4278f916` | ui: Theme.TitleWeight | Act third_party/mygo-patches/0071-theme-title-weight.patch |
| `4b657b1a` | ui: SidebarEntry adds an item the keys choose without building it | Act third_party/mygo-patches/0069-sidebar-entry.patch |
| `df24c8d9` | transfer: PromisedFiles, files another app promises in a drop (macOS) | Act third_party/mygo-patches/0042-file-promise-drop.patch |
| `768f24d0` | transfer: remove the folder of a promise no one took | Act third_party/mygo-patches/0049-promise-folder-forgotten.patch |
| `0a85d3b6` | window: PanelKeyboard, when a panel takes the keyboard (for text, never) | Act third_party/mygo-patches/0003-panel-keyboard-only-for-text.patch Origin: Dicta third_party/mygo-patches/0003-panel-keyboard-only-for-text.patch Origin: Inboxly third_party/mygo-patches/0003-panel-keyboard-only-for-text.patch Origin: CaptionCraft third_party/mygo-patches/0003-panel-keyboard-only-for-text.patch Origin: Paralign third_party/mygo-patches/0003-panel-keyboard-only-for-text.patch Origin: Auralink third_party/mygo-patches/0003-nonactivating-window.patch Origin: VoxtMini third_party/mygo-patches/0003-nonactivating-window.patch |
| `4d5250a4` | ui: a touchpad's pinch as one gesture: InputZoom's Phase and smart zoom | Act third_party/mygo-patches/0006-touchpad-magnify.patch Origin: Act third_party/mygo-patches/0014-smart-magnify-scroll-mods.patch Origin: Inboxly third_party/mygo-patches/0006-touchpad-magnify.patch Origin: Inboxly third_party/mygo-patches/0014-smart-magnify-scroll-mods.patch Origin: CaptionCraft third_party/mygo-patches/0006-touchpad-magnify.patch Origin: Paralign third_party/mygo-patches/0006-touchpad-magnify.patch Origin: AudioSmith third_party/mygo-patches/0012-magnify-gesture.patch |
| `7137181f` | menus: SF Symbol and scaled image icons, no stray separators, Tester.MenuTree | Act third_party/mygo-patches/0019-menu-symbols.patch Origin: Act third_party/mygo-patches/0028-menu-item-image.patch Origin: Inboxly third_party/mygo-patches/0019-menu-symbols.patch Origin: Inboxly third_party/mygo-patches/0028-menu-item-image.patch |
| `5ee8297d` | ui: follow a native drag over the window, scale its preview, drop it back in tests | Act third_party/mygo-patches/0079-data-drag-follow-scale.patch |
| `56e94b39` | ui: a drop of this process's value tells its effect; a drag offers its own windows more than other apps | Act third_party/mygo-patches/0080-drop-effect-local-operations.patch |
| `8667843a` | ui: SetMenuLabels, a function every label of a view's menus passes through | Act third_party/mygo-patches/0041-menu-labels.patch |
| `a2a6cd90` | ui: Handle.OpenContextMenu opens a control's context menu from code | Act third_party/mygo-patches/0068-open-context-menu.patch |
| `e8dbef2f` | ui: a touchpad scroll's gesture phases, and swipes as InputSwipe | Act third_party/mygo-patches/0018-touchpad-swipe.patch Origin: Inboxly third_party/mygo-patches/0018-touchpad-swipe.patch |
| `9fb3b9a4` | screen: Display.Notch, where a MacBook's camera housing is | Act third_party/mygo-patches/0037-display-notch.patch Origin: Inboxly third_party/mygo-patches/0038-display-notch.patch |
| `7b1650d8` | window: OverMenuBar, a panel over the menu bar (the notch's shelf) | Act third_party/mygo-patches/0036-panel-over-menu-bar.patch Origin: Inboxly third_party/mygo-patches/0037-panel-over-menu-bar.patch |
| `d3715b70` | ui: a web view's clip on whole pixels, its page on whole points | Act third_party/mygo-patches/0053-webview-clip-whole-pixels.patch Origin: Act third_party/mygo-patches/0055-webview-frame-whole-points.patch |
| `9341f45f` | darwin: a web view's page snapshot is encoded off the main thread | Act third_party/mygo-patches/0057-capture-page-off-main.patch |
| `088bfd6a` | screen: WatchPointer, the pointer and drags in any app (notch shelves) | Act third_party/mygo-patches/0038-pointer-watch.patch Origin: Act third_party/mygo-patches/0043-pointer-watch-drag-end.patch Origin: Act third_party/mygo-patches/0048-pointer-watch-rest.patch Origin: Inboxly third_party/mygo-patches/0039-pointer-watch.patch Origin: Inboxly third_party/mygo-patches/0042-pointer-refresh.patch |
| `aa817ba9` | Page.SetNavigationFilter: decide every navigation of a page, later if need be | Act third_party/mygo-patches/0046-navigation-filter-content-rules.patch |
| `d56a4ad4` | webview: share a touchpad's gestures between a page and the view around it | Act third_party/mygo-patches/0025-webview-sideways-to-content.patch Origin: Act third_party/mygo-patches/0029-webview-pinch-in-to-content.patch Origin: Act third_party/mygo-patches/0054-webview-gestures.patch Origin: Act third_party/mygo-patches/0056-webview-edges-asked.patch Origin: Act third_party/mygo-patches/0058-webview-pdf-controls.patch Origin: Act third_party/mygo-patches/0059-webview-pinch-keeps-shown.patch Origin: Act third_party/mygo-patches/0061-webview-pdf-fit.patch Origin: Act third_party/mygo-patches/0062-webview-glide-after-load.patch |
| `10712064` | webview: a picture under a web view's page (WebView.SetUnderlay) | Act third_party/mygo-patches/0081-webview-underlay.patch Origin: Act third_party/mygo-patches/0082-webview-underlay-own-layer.patch |
| `9b8fa57c` | tray: overlay and dot in color over a template icon, NativeButton | Act third_party/mygo-patches/0020-tray-overlay.patch Origin: Inboxly third_party/mygo-patches/0020-tray-overlay.patch Origin: AudioSmith third_party/mygo-patches/0013-tray-dot.patch Origin: HereNow third_party/mygo-patches/0008-tray-native-button.patch |
| `19bca2a7` | darwin: Shift in menu key equivalents as AppKit has it | Act third_party/mygo-patches/0016-menu-shift-key-equivalent.patch Origin: Dicta third_party/mygo-patches/0004-menu-shift-key-equivalent.patch Origin: Inboxly third_party/mygo-patches/0016-menu-shift-key-equivalent.patch Origin: Pik third_party/mygo-patches/0008-menu-shift-key-equivalent.patch Origin: HereNow third_party/mygo-patches/0010-menu-shift-key-equivalent.patch Origin: AudioSmith third_party/mygo-patches/0009-menu-shift-key-equivalent.patch Origin: Auralink third_party/mygo-patches/0008-menu-shift-key-equivalent.patch Origin: CaptionCraft third_party/mygo-patches/0008-menu-shift-key-equivalent.patch Origin: Paralign third_party/mygo-patches/0008-menu-shift-key-equivalent.patch |
| `033a3901` | darwin: a transfer format named by a UTI is that pasteboard type | Pik third_party/mygo-patches/0003-uti-formats.patch Origin: Act third_party/mygo-patches/0008-uti-formats.patch Origin: Inboxly third_party/mygo-patches/0008-uti-formats.patch |
| `46f5c3de` | mygo build: CFBundleVersion takes the numbers of the version | Auralink third_party/mygo-patches/0007-bundle-version.patch Origin: VoxtMini third_party/mygo-patches/0007-bundle-version.patch |
| `c29e6a68` | cmd/mygo: versioned uploads to S3 are cached for good | Pik third_party/mygo-patches/0005-s3-immutable-uploads.patch |
| `364aa110` | cmd/mygo: wait again for a notarization after a network failure | AudioSmith third_party/mygo-patches/0004-notarize-wait-again.patch |
| `84967222` | macos.contents: helpers, frameworks and plug-ins beside Resources | Act third_party/mygo-patches/0004-macos-contents.patch Origin: Inboxly third_party/mygo-patches/0004-macos-contents.patch Origin: CaptionCraft third_party/mygo-patches/0004-macos-contents.patch Origin: Paralign third_party/mygo-patches/0004-macos-contents.patch Origin: HereNow third_party/mygo-patches/0001-macos-plugins.patch |
| `a408f5f6` | darwin: a tray image that declares its size keeps it | Pik third_party/mygo-patches/0006-tray-declared-size.patch Origin: Act third_party/mygo-patches/0010-tray-declared-size.patch Origin: Inboxly third_party/mygo-patches/0010-tray-declared-size.patch Origin: HereNow third_party/mygo-patches/0005-tray-declared-size.patch |
| `d07e507d` | ui: SetTheme keeps what the view gave the root | Pik third_party/mygo-patches/0007-settheme-keeps-root.patch Origin: Act third_party/mygo-patches/0011-settheme-keeps-root.patch Origin: Inboxly third_party/mygo-patches/0011-settheme-keeps-root.patch Origin: HereNow third_party/mygo-patches/0006-settheme-keeps-root.patch |
| `96e0c478` | ui: AccessInfo tells where an element is in the tree and if it presses | Pik third_party/mygo-patches/0004-tester-access-tree.patch Origin: Act third_party/mygo-patches/0009-tester-access-tree.patch Origin: Inboxly third_party/mygo-patches/0009-tester-access-tree.patch Origin: HereNow third_party/mygo-patches/0004-tester-access-tree.patch |
| `a478cda4` | notifications: buttons and text replies (RegisterNotificationCategory) | Inboxly third_party/mygo-patches/0029-notification-actions.patch |
| `49939b5a` | notifications: Passive, and App.NotificationStatus | Inboxly third_party/mygo-patches/0032-notification-passive-status.patch |
| `bb938f85` | window: SetVibrancyStyle, a material shaped to the window's own shape | Auralink third_party/mygo-patches/0004-vibrancy-style.patch Origin: VoxtMini third_party/mygo-patches/0004-vibrancy-style.patch Origin: Act third_party/mygo-patches/0044-vibrancy-frame.patch Origin: Act third_party/mygo-patches/0052-material-frame-sync.patch Origin: Inboxly third_party/mygo-patches/0040-vibrancy-frame.patch |
| `c87d246d` | darwin: headless tests of a touchpad's gesture phases and swipes | (fork maintenance) |
| `75be7b58` | updates: a release signature binds the version and channel; HTTPS redirects only | Act third_party/mygo-patches/0047-update-release-signature.patch Origin: Dicta third_party/mygo-patches/0006-update-release-signature.patch |
| `36ed56fa` | updater: a feed with nothing published is up to date, not an error | Act third_party/mygo-patches/0005-updates-nothing-published.patch Origin: Dicta third_party/mygo-patches/0005-updates-nothing-published.patch Origin: Inboxly third_party/mygo-patches/0005-updates-nothing-published.patch Origin: CaptionCraft third_party/mygo-patches/0005-updates-nothing-published.patch Origin: Paralign third_party/mygo-patches/0005-updates-nothing-published.patch Origin: Auralink third_party/mygo-patches/0006-prereleases.patch (ErrNotPublished) Origin: VoxtMini third_party/mygo-patches/0006-prereleases.patch (ErrNotPublished) |
| `35718fa9` | updates: a beta channel beside updates.url (updates.prereleases) | AudioSmith third_party/mygo-patches/0003-update-prereleases.patch Origin: AudioSmith third_party/mygo-patches/0008-update-url-prereleases.patch Origin: Auralink third_party/mygo-patches/0006-prereleases.patch Origin: VoxtMini third_party/mygo-patches/0006-prereleases.patch |
| `cb3b7899` | updater: Updater.SetTransport, and downloads that resume where they broke off | Act third_party/mygo-patches/0076-updater-transport.patch Origin: Dicta third_party/mygo-patches/0007-updater-transport.patch Origin: AudioSmith third_party/mygo-patches/0007-updater-client-and-resume.patch |
| `4ce61bab` | updater: Updater.Configured | VoxtMini third_party/mygo-patches/0009-updater-configured.patch |
| `9a9696ff` | plugins/updater: Options.Found | VoxtMini third_party/mygo-patches/0010-updater-found.patch |
| `276f18e3` | actnow/upchain: the update chain check from an app's published version | (fork maintenance) |
| `179c844e` | mygo: OnContinueActivity — Spotlight results opened, Handoffs | Inboxly third_party/mygo-patches/0030-continue-activity.patch |
| `61700957` | mygo: SetSystemGuard — ask before changing anything outside the app | Inboxly third_party/mygo-patches/0031-system-guard.patch |
| `23334fba` | app: WasLaunchedToOpen, and a launch the system left behind stays behind | Act third_party/mygo-patches/0031-launched-to-open.patch Origin: Act third_party/mygo-patches/0040-launched-to-open-not-restore.patch Origin: Act third_party/mygo-patches/0077-quit-stops-loop.patch (launch part) Origin: Act third_party/mygo-patches/0078-accessory-launch-quiet.patch |
| `3118407c` | shell: TrashItem does not wait for the main thread on macOS | Act third_party/mygo-patches/0050-trash-off-main-thread.patch |
| `5e2b31c3` | ui: Context.AccessibilityActive, for what a view builds for assistive technology | Inboxly third_party/mygo-patches/0041-accessibility-active.patch |
| `a1370ded` | window: GiveUpKey; a click on a window takes the keyboard from a panel | Act third_party/mygo-patches/0074-click-takes-key-from-panel.patch Origin: Act third_party/mygo-patches/0075-give-up-key.patch |
| `855f16cc` | window: SpaceInfo, and panels rejoin every Space as they show | Inboxly third_party/mygo-patches/0043-window-spaces.patch |
| `37234afe` | internal/uimigrate: regenerate uiapi.go for the merged ui API | (fork maintenance) |
| `d9689083` | ui: ExtendsChoice, a sidebar item whose ⌘- and ⇧-clicks extend the app's choice | Act third_party/mygo-patches/0070-sidebar-extends-choice.patch |
| `7a067754` | ui: Element.LiveValue, a slider painting a value that moves between builds | Act third_party/mygo-patches/0072-slider-live-value.patch |
| `76406553` | ui: Element.PressedWithin, a press held on the element or inside it | Act third_party/mygo-patches/0073-pressed-within.patch |
| `be61d6c4` | ui: Element.Expandable and ExpandAsked, for items that show or hide others | Act third_party/mygo-patches/0021-expandable-items.patch Origin: Inboxly third_party/mygo-patches/0021-expandable-items.patch |
| `3f8763ab` | ui, ui/markdown: highlights in read text (marked passages) | Act third_party/mygo-patches/0039-text-highlights.patch |
| `1f1fdb72` | markdown/parser: bound the empty cells a table adds | Act third_party/mygo-patches/0045-table-padding-bound.patch |
| `5aaefac7` | ui/richtext: EditorOptions.OnPaste, a paste hook for files and pictures | Inboxly third_party/mygo-patches/0033-richtext-paste-hook.patch |
| `0997523a` | ui: MenuItem.Command, an item doing an editing command on the menu's text | Inboxly third_party/mygo-patches/0035-richtext-context-menu.patch |
| `e1f4fcf8` | ui/richtext: resize a picture by its corners | Inboxly third_party/mygo-patches/0036-richtext-image-resize.patch |
| `e0e4069f` | ui/richtext: an editor paints only in its box | Inboxly third_party/mygo-patches/0044-richtext-clip-to-box.patch |
| `b2e66feb` | internal/uimigrate: regenerate uiapi.go for G6's ui API | (fork maintenance) |
| `0e964ffe` | render: Options.Silent skips building every frame to hear its sound | Fino third_party/mygo-patches/0001-render-silent.patch |
| `3ed90dde` | text, ui: SetFontRendering for new systems; Pango lays out unhinted text | Fino third_party/mygo-patches/0002-text-unhinted-layout.patch |
| `2da0867c` | text, ui: a closed Recorder's text engine goes to the next Recorder | Fino third_party/mygo-patches/0004-text-reuse-released-engines.patch |
| `6b8bea67` | ui: Element.CursorImage — a picture as the pointer | Act third_party/mygo-patches/0027-cursor-image.patch Origin: Inboxly third_party/mygo-patches/0027-cursor-image.patch |
| `1952c396` | darwin: build pointerwatch.go only on macOS | (fork maintenance) |
| `9eba6185` | ui, metal: a budget for images' textures (SetTextureBudget) | Act third_party/mygo-patches/0012-texture-budget.patch Origin: Inboxly third_party/mygo-patches/0012-texture-budget.patch Origin: Act third_party/mygo-patches/0032-idle-trim-redraws-at-once.patch Origin: Act third_party/mygo-patches/0066-idle-trim-keeps-shown.patch |
| `db895067` | ui: Context.PixelScale, the screen's pixels per DIP | Act third_party/mygo-patches/0023-pixel-scale.patch |
| `3d4bdb2a` | ui: a single-line input's text and placeholder stay in its content box | Act third_party/mygo-patches/0024-input-content-clip.patch Origin: Inboxly third_party/mygo-patches/0024-input-content-clip.patch |
| `b188cd64` | ui: Element.AlignFirstLines, a row lining up its children's first lines | Act third_party/mygo-patches/0060-align-first-lines.patch Origin: HereNow third_party/mygo-patches/0009-align-baselines.patch |
| `cd60e8ea` | internal/scene: Repack places rectangles whatever their order | AudioSmith third_party/mygo-patches/0005-atlas-stable-repack.patch |
| `f21b73db` | ui: test that a Tester draws as in a fresh process | AudioSmith third_party/mygo-patches/0006-forget-glyphs.patch (not carried) |
| `e723800a` | ui: a Tester draws a frame's pixels when they are asked for | Act third_party/mygo-patches/0034-tester-draws-when-asked.patch |
| `6f1d273e` | text, ui: register a variable font at a point of its axes | Act third_party/mygo-patches/0017-register-font-variation.patch |
| `46dd7ca9` | ui: Content.FontSmoothing turns font smoothing off for a window's text | Auralink third_party/mygo-patches/0005-font-smoothing-off.patch Origin: VoxtMini third_party/mygo-patches/0005-font-smoothing-off.patch |
| `7ca35efe` | gpu: clips only scaled and moved are cut in the shader, not masked | Act third_party/mygo-patches/0064-posed-clips-in-shader.patch |
| `3e92a44b` | metal: per-frame timing and GPU time (MYGO_METAL_TIMING) | Act third_party/mygo-patches/0015-metal-frame-timing.patch Origin: Inboxly third_party/mygo-patches/0015-metal-frame-timing.patch Origin: Act third_party/mygo-patches/0065-metal-gpu-time.patch |
| `d5b2680b` | ui: test that After pauses with WindowHidden while the window is hidden | AudioSmith third_party/mygo-patches/0014-after-waits-while-occluded.patch (not carried) |
| `3eb39cef` | internal/uimigrate: the ui API with G5's additions | (fork maintenance) |
| `afc3cde8` | ui: pad node to a whole number of cache lines after the fork's fields | (fork maintenance) |
| `bd10f07a` | render: Options.RateLimits and Options.Colorimetry | Onda (fino) request 2026-11-22 |
| `15e2e842` | render: ffmpeg converts the frames with BT.709's matrix and keeps the tags | Onda (fino) request 2026-11-22 |
| `c10cb353` | ui: a layer inside another layer gives its subtree back the outer one's transforms | Onda (fino) report 2026-11-22, Fino docs/v2/m1-wp2-mygo-layer.md |
| `bc0e602d` | ui: Theme.TooltipRest and TooltipFadeIn/Out: a tip waits for the pointer (or the focus) to rest, a short warm-up, scrolls/presses/drags hide it; fades in and out, not under ReduceMotion. Off by default: other apps unchanged (TestTooltipDefaultUnchanged) | Act request 2026-11-22 (thread #429) |
| `9117b3dc` | ui.SetLowLatency: display sync off (macOS) while the app wants edits shown at once; Metal latency diagnostics | Onda (fino) WP4 request 2026-11-23, Fino docs/v2/m2-perf.md §6 |
| `8c7e5ad0` | ui: a list (or table) row takes the press of an element in it that only drags (Drag/DragData, no clicks of its own): a click, Cmd/Shift-click, double click choose and submit the row; moving past the threshold drags the element's value (rowAroundDragger, TestRowAroundDragger) | Act thread #492 (the list view's names dragged out took the rows' clicks) |
| `2d25a97d` | windows: plain transparent shadowless panels and DirectComposition surfaces stay free of Windows 11 DWM rounded corners, border, and shadow (`DWMWCP_DONOTROUND`, `DWMWA_COLOR_NONE`, `DWMNCRP_DISABLED`, `CornerRadius` 0); opaque, shadowed, vibrant, rounded, and flyout windows unchanged (`frame_policy.go`, `TestTransparentFrameDWM`, `TestTransparentSurfacePanelStylesAndCorners`) | Act thread #621 (Windows 11 transparent shadowless top shelf panel) |
