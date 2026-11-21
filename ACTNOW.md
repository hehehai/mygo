# ActNow's changes to MyGo

Base: upstream `v0.10.0`. Each change is one commit (or a short series) whose message names its origin: the app and patch file it replaces (`<App> third_party/mygo-patches/NNNN-name.patch`).

## Sync with upstream

    git fetch upstream && git push origin upstream/main:upstream
    git rebase --onto <new upstream tag> <old upstream tag> main   # on a branch, re-test, then fast-forward main
    git tag vX.Y.Z-actnow.1 && git push origin vX.Y.Z-actnow.1     # tags are immutable once fetched through the Go proxy

## Changes

(filled in as patches land)
