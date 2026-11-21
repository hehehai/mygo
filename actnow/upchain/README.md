# The update chain check

Every app that moves to this fork must keep updating the versions its users
run now, whose updaters are MyGo 0.5.5 with the app's own patches. This
check builds an app's published version with the app's vendored MyGo, as it
is, and the next versions with the fork, and has the old version's updater
take them, on mac-air, in the background.

    actnow/upchain/upchain.sh <App> <old tag> [--fork-rev REV] [--v1 V] [--v2 V] [--new-style url-beta|prereleases] [--keep]
    actnow/upchain/upchain.sh Pik v0.1.0-beta.3
    actnow/upchain/upchain.sh Dicta v0.1.0-beta.9

Run it on the Linux box, from this repository. The app's repository is
`~/helper/<App>` (`UPCHAIN_REPOS`); its line in [apps.conf](apps.conf) gives
the feed name, the target and how its releases are built (the old style and
the new). The fork's commit (`--fork-rev`, default `HEAD`) must be pushed to
github.com/hehehai/mygo: the Mac fetches it from there. Of the app's
vendored MyGo only the packages the probe and `cmd/mygo` build from go to the
Mac, once per app and tag (`~/code/mygo-upd/upchain/cache/`).

On the Mac, `mac.sh` runs under `lockf -k /tmp/mygo-mac.lock`, with nohup, in
a process group of its own; `upchain.sh` looks for its end every 150 s
(`UPCHAIN_POLL`) and brings back `report.txt`, `run.log`, `logs/` and
`requests.log` to `./upchain-out/<run>/` (`UPCHAIN_OUT`). It exits 0 when
every check passed. Connections that fail are tried again every 30 s, 10
times.

What it checks:

1. The old version (`<old tag>`): a probe app of the app's name and version,
   built by the old MyGo with a test key pair and the updates of the old
   style, published in a local HTTPS feed laid out as actnow.dev's
   (`/updates/<app>/`, `beta/`).
2. The next beta (V1) and the release (V2), built by the fork, which reads
   the feed to make deltas; each passes the checks of actnow.dev's release
   pipeline (file names, URLs in the channel's directory, sizes, signatures)
   and has release signatures.
3. The old version's updater checks and installs them into a copy of the
   old bundle: by delta, in full, falling back from a bad delta; when it
   checks release signatures, it refuses a manifest without them and an
   archive offered as another version; when it has `SetPrereleases`, it
   takes the beta with it on and not with it off. Each install is the new
   archive file for file, passes `codesign --verify --deep --strict`, and
   runs as the new version.

What it does not do: start an app's UI (the probe sets its name and version
and never touches AppKit; `LSUIElement`), change HOME, use a keychain (ad hoc
signatures, no notarization, no disk images, `ACTNOW_NO_KEYCHAIN=1`). A
watchdog stops the run's process group if a SecurityAgent prompt or a
`security(1)` process shows up. The build tools and probes trust the run's
test CA through a file added to scratch copies only (`inject/`); nothing of
the updaters or of the build changes.

Files: `upchain.sh` (Linux), `mac.sh` (Mac), `tool/` (the feed server, the
site's publishing and checks; standard library only), `probe/main.go.in`,
`inject/`, `apps.conf`. A module of its own (`go.mod`), not part of MyGo's.
