#!/bin/bash
# The Mac half of the update chain check (README.md), run by upchain.sh in
# a run directory it copied over: R/upchain (these tools) and R/run.env,
# whose OLD_CACHE (the app's vendored MyGo at the old tag, as it is) and
# FORK_CACHE (MyGo's fork) it copies into R/old and R/fork. It never starts an app's UI, never takes the foreground,
# never changes HOME and uses no keychain (ad hoc signatures, no
# notarization, a test key pair and a test CA trusted by the probes and
# build tools only); it stops its own process group if a keychain or
# authorization prompt (SecurityAgent) shows up. upchain.sh runs it under
# lockf -k /tmp/mygo-mac.lock, so the Mac's tests run one at a time.
#
#   bash mac.sh <run directory>
set -uo pipefail
shopt -s nullglob

R=$(cd "$1" && pwd)
# shellcheck source=/dev/null
source "$R/run.env"
export PATH="$HOME/.local/go/bin:/usr/bin:/bin:/usr/sbin:/sbin:$PATH"
export ACTNOW_NO_KEYCHAIN=1 GOFLAGS=-mod=mod GOTOOLCHAIN=local CGO_ENABLED=0
unset MYGO_UPDATER_PRIVATE_KEY MYGO_ENV MYGO_DEV
W="$R/w"
REPORT="$R/report.txt"
rm -rf "$W"
mkdir -p "$W"/{bin,keys,logs,feed/updates/"$FEED"}
touch "$R/.metadata_never_index" # keep Spotlight (and Launch Services) away from the test bundles
: >"$REPORT"
PGID=$(ps -o pgid= $$ | tr -d ' ')
FAILS=0

say() { printf '%s\n' "$*" | tee -a "$REPORT"; }
pass() { say "PASS $*"; }
fail() { say "FAIL $*"; FAILS=$((FAILS + 1)); }
die() { say "ABORT $*"; cleanup; exit 2; }

# The watchdog: a new SecurityAgent (a keychain or authorization prompt)
# or a security(1) process of ours stops the whole process group.
baseline=$(pgrep -x SecurityAgent | tr '\n' ' ')
(
  while sleep 1; do
    for p in $(pgrep -x SecurityAgent); do
      case " $baseline " in *" $p "*) ;; *)
        echo "ABORT SecurityAgent $p appeared: stopping process group $PGID" >>"$REPORT"
        kill -TERM -- "-$PGID"; exit 0 ;;
      esac
    done
    if pgrep -g "$PGID" -x security >/dev/null 2>&1; then
      echo "ABORT security(1) ran in process group $PGID: stopping it" >>"$REPORT"
      kill -TERM -- "-$PGID"; exit 0
    fi
  done
) &
WATCHDOG=$!

SERVER=
cleanup() {
  [ -n "$SERVER" ] && kill "$SERVER" 2>/dev/null
  kill "$WATCHDOG" 2>/dev/null
  if [ "${KEEP:-0}" != 1 ]; then
    rm -rf "$W/p0" "$W/p1" "$W/p2" "$W/inst" "$W/ref" "$W/bin" "$R/old" "$R/fork"
  fi
}
trap 'cleanup' EXIT
trap 'say "ABORT signal"; exit 3' TERM INT

run() { # run <log name> <command…>: the command's output goes to logs/<name>.log
  local name=$1; shift
  if ! "$@" >"$W/logs/$name.log" 2>&1; then
    tail -30 "$W/logs/$name.log" | sed 's/^/    /' >>"$REPORT"
    return 1
  fi
}

say "upchain $APP: $OLD_VERSION (old MyGo, $OLD_STYLE) → $V1 → $V2 (fork, $NEW_STYLE), $TGT"
say "mac: $(sw_vers -productVersion), $(go version | cut -d' ' -f3)"

# --- The trees, from the cache upchain.sh sent them to.
rm -rf "$R/old" "$R/fork"
cp -R "$R/$OLD_CACHE" "$R/old" && cp -R "$R/$FORK_CACHE" "$R/fork" || die "copying the trees from the cache"

# --- Tools: the helper, and the two MyGo CLIs, which trust the test CA.
run tool bash -c "cd '$R/upchain' && go build -o '$W/bin/upchain' ./tool" || die "building the helper"
for t in old fork; do
  cp "$R/upchain/inject/zz_upchain_ca.go.in" "$R/$t/cmd/mygo/zz_upchain_ca.go"
  run "mygo-$t" bash -c "cd '$R/$t' && go build -o '$W/bin/mygo-$t' ./cmd/mygo" || die "building $t cmd/mygo"
done

# --- The feed: HTTPS on 127.0.0.1 with the run's CA.
echo normal >"$W/mode"
"$W/bin/upchain" serve -root "$W/feed" -ca "$W/ca.pem" -port "$W/port" -log "$W/requests.log" -mode "$W/mode" >"$W/logs/server.log" 2>&1 &
SERVER=$!
for _ in $(seq 50); do [ -s "$W/port" ] && break; sleep 0.1; done
[ -s "$W/port" ] || die "the feed server did not start"
SITE="https://127.0.0.1:$(cat "$W/port")"
BASE="$SITE/updates/$FEED"
export UPCHAIN_CA="$W/ca.pem"
say "feed: $BASE"

run keygen "$W/bin/mygo-fork" keygen -o "$W/keys" || die "keygen"
PUB=$(tr -d '\n' <"$W/keys/mygo-update.pub")

# build <dir> <cli> <version> <style>: the probe app of version, built by
# that MyGo, with the updates of style:
#   url-beta     updates.url is the feed, …/beta for a prerelease (Pik, Dicta,
#                Inboxly, Refto: their release scripts)
#   prereleases  updates.url is the feed, updates.prereleases (the fork;
#                Auralink's and VoxtMini's 0006)
#   convention   updates.url is the feed; prereleases go in beta/ by
#                AudioSmith's 0008, without a setting
build() {
  local dir=$1 cli=$2 version=$3 style=$4 mygo url pre=false
  case $cli in old) mygo="$R/old" ;; *) mygo="$R/fork" ;; esac
  url=$BASE
  case $style in
    url-beta) case $version in *-*) url="$BASE/beta" ;; esac ;;
    prereleases) pre=true ;;
  esac
  rm -rf "$dir"; mkdir -p "$dir"
  sed -e "s|@NAME@|$NAME|" -e "s|@VERSION@|$version|" "$R/upchain/probe/main.go.in" >"$dir/main.go"
  if grep -q 'func (u \*UpdaterModule) SetPrereleases' "$mygo"/updater*.go; then
    sed -i '' 's|//PRE ||' "$dir/main.go"
  fi
  cat >"$dir/go.mod" <<EOF
module upchainprobe

go 1.27.1

require github.com/egoist/mygo v0.5.5

replace github.com/egoist/mygo => $mygo
EOF
  local prekey=""
  [ "$pre" = true ] && prekey=', "prereleases": true'
  cat >"$dir/mygo.json" <<EOF
{
  "name": "$NAME",
  "identifier": "dev.actnow.upchain.$FEED",
  "version": "$version",
  "out": "build",
  "macos": {"minimumSystemVersion": "13.0", "infoPlist": {"LSUIElement": true}},
  "updates": {"publicKey": "$PUB", "url": "$url", "privateKey": "$W/keys/mygo-update.key"$prekey}
}
EOF
  run "tidy-$version" bash -c "cd '$dir' && go mod tidy" || return 1
  run "build-$version" bash -c "cd '$dir' && '$W/bin/mygo-$cli' build -sign - -skip-dmg -skip-notarize -platform '$PLATFORM'"
}

# reference <version> <dir>: the bundle of the version's archive, as it
# installs.
reference() {
  rm -rf "$W/ref/$1"; mkdir -p "$W/ref/$1"
  tar -xzf "$2"/*-"$1"-"$TGT".tar.gz -C "$W/ref/$1"
}

# install_old: a fresh install of the old version, from its archive (as
# from its disk image), in inst/.
install_old() {
  rm -rf "$W/inst"; mkdir -p "$W/inst"
  ditto "$W/ref/$OLD_VERSION/$NAME.app" "$W/inst/$NAME.app"
  EXE="$W/inst/$NAME.app/Contents/MacOS/$(plutil -extract CFBundleExecutable raw "$W/inst/$NAME.app/Contents/Info.plist")"
}

# probe <mode> <prereleases 0|1> <check|install>: the installed old app
# checks (and installs) with the feed in mode; its output in $OUT, the
# feed's requests meanwhile in $REQ.
probe() {
  echo "$1" >"$W/mode"
  local mark; mark=$(wc -l <"$W/requests.log")
  OUT=$(UPCHAIN_PROBE=$3 UPCHAIN_PRERELEASES=$2 "$EXE" 2>&1)
  REQ=$(tail -n +"$((mark + 1))" "$W/requests.log")
  echo "$OUT" >>"$W/logs/probes.log"; echo "$REQ" >>"$W/logs/probes.log"
  echo normal >"$W/mode"
}
field() { sed -n "s/^$1=//p" <<<"$OUT" | head -1; }
got() { grep -c "$1" <<<"$REQ"; }

# installed <version> <what>: the bundle in inst/ is the version's archive,
# whole, its signature holds, and it runs.
installed() {
  local app="$W/inst/$NAME.app" v
  v=$(plutil -extract CFBundleShortVersionString raw "$app/Contents/Info.plist" 2>/dev/null)
  if [ "$v" != "$1" ]; then fail "$2: the bundle says $v, not $1"; return; fi
  if ! diff -r "$W/ref/$1/$NAME.app" "$app" >"$W/logs/diff.log" 2>&1; then fail "$2: the bundle differs from the archive of $1 (logs/diff.log)"; return; fi
  if ! codesign --verify --deep --strict "$app" >>"$W/logs/codesign.log" 2>&1; then fail "$2: codesign --verify fails"; return; fi
  if [ "$("$EXE" version 2>/dev/null)" != "$1" ]; then fail "$2: the installed app does not run as $1"; return; fi
  pass "$2"
}

# scenario <name> <mode> <pre> <want version> <how: delta|archive|fallback|refused|none>
scenario() {
  local name=$1 mode=$2 pre=$3 want=$4 how=$5
  install_old
  probe "$mode" "$pre" install
  local check; check=$(field check)
  case $how in
    none)
      if [ "$check" = none ] || { [ "$check" = error ] && grep -qi 'not published\|404' <<<"$(field error)"; }; then
        pass "$name: nothing offered ($check $(field error))"
      else fail "$name: check=$check offered=$(field offered) error=$(field error)"; fi
      return ;;
  esac
  if [ "$check" != offered ] || [ "$(field offered)" != "$want" ]; then
    fail "$name: check=$check offered=$(field offered) error=$(field error), want $want"; return
  fi
  case $how in
    refused)
      if [ "$(field install)" = error ] && grep -q signed <<<"$(field error)" && [ "$(plutil -extract CFBundleShortVersionString raw "$W/inst/$NAME.app/Contents/Info.plist")" = "$OLD_VERSION" ]; then
        pass "$name: refused ($(field error)), the app unchanged"
      else fail "$name: install=$(field install) error=$(field error)"; fi
      return ;;
  esac
  if [ "$(field install)" != ok ]; then fail "$name: install=$(field install) error=$(field error)"; return; fi
  local deltas archives
  deltas=$(got "\.delta 200"); archives=$(got "\.tar\.gz 200")
  case $how in
    delta) [ "$deltas" = 1 ] && [ "$archives" = 0 ] || { fail "$name: $deltas delta and $archives archive downloads, want the delta only"; return; } ;;
    archive) [ "$deltas" = 0 ] && [ "$archives" = 1 ] || { fail "$name: $deltas delta and $archives archive downloads, want the archive only"; return; } ;;
    fallback) [ "$deltas" = 1 ] && [ "$archives" = 1 ] || { fail "$name: $deltas delta and $archives archive downloads, want the delta then the archive"; return; } ;;
    any) ;;
  esac
  installed "$want" "$name: $OLD_VERSION → $want ($how, $(field downloaded) bytes)"
}

# --- The old version, built by the app's old MyGo, published.
build "$W/p0" old "$OLD_VERSION" "$OLD_STYLE" || die "building $OLD_VERSION with the old MyGo"
OUT0="$W/p0/build/$TGT"
reference "$OLD_VERSION" "$OUT0"
run publish0 "$W/bin/upchain" publish -feed "$W/feed/updates/$FEED" "$OUT0"/*.tar.gz "$OUT0"/update-*.json || die "publishing $OLD_VERSION"
say "built $OLD_VERSION: $(cd "$OUT0" && ls *.tar.gz update-*.json | tr '\n' ' ')"

# --- The next beta, built by the fork from the published old version.
build "$W/p1" fork "$V1" "$NEW_STYLE" || die "building $V1 with the fork"
OUT1="$W/p1/build/$TGT"
reference "$V1" "$OUT1"
if run sitecheck1 "$W/bin/upchain" sitecheck -site "$SITE" -app "$FEED" -version "$V1" -pub "$PUB" -dir "$OUT1" -release; then
  pass "$V1: the site's checks (names, URLs in $(case $V1 in *-*) echo beta/ ;; *) echo the releases\' directory ;; esac), sizes, signatures, release signatures)"
else fail "$V1: the site's checks"; fi
grep -q "\"from\": \"$OLD_VERSION\"" "$OUT1/update-$TGT.json" && pass "$V1 has a delta from $OLD_VERSION" || fail "$V1 has no delta from $OLD_VERSION"
run publish1 "$W/bin/upchain" publish -feed "$W/feed/updates/$FEED" "$OUT1"/*.tar.gz "$OUT1"/*.delta "$OUT1"/update-*.json || die "publishing $V1"

# The old app's channel for a beta: url-beta builds read beta/ only;
# the others read beta/ with prereleases on.
PRE=0
case $OLD_STYLE in prereleases | convention) PRE=1 ;; esac
scenario "beta by delta" normal $PRE "$V1" delta
scenario "beta in full" nodelta $PRE "$V1" archive
scenario "beta, delta fails" baddelta $PRE "$V1" fallback
if [ "$OLD_HAS_RELEASE" = 1 ]; then
  scenario "no release signature" norelease $PRE "$V1" refused
  scenario "relabelled version" relabel $PRE 99.0.0 refused
fi
if [ "$PRE" = 1 ]; then
  scenario "beta not offered with prereleases off" normal 0 "" none
fi

# --- The release, built by the fork, published in both channels.
build "$W/p2" fork "$V2" "$NEW_STYLE" || die "building $V2 with the fork"
OUT2="$W/p2/build/$TGT"
reference "$V2" "$OUT2"
if run sitecheck2 "$W/bin/upchain" sitecheck -site "$SITE" -app "$FEED" -version "$V2" -pub "$PUB" -dir "$OUT2" -release; then
  pass "$V2: the site's checks"
else fail "$V2: the site's checks"; fi
run publish2 "$W/bin/upchain" publish -feed "$W/feed/updates/$FEED" "$OUT2"/*.tar.gz "$OUT2"/*.delta "$OUT2"/update-*.json || die "publishing $V2"
say "$V2 deltas from: $(sed -n 's/.*"from": "\(.*\)".*/\1/p' "$OUT2/update-$TGT.json" | tr '\n' ' ')"
scenario "release from the beta channel" normal $PRE "$V2" any
if [ "$PRE" = 1 ]; then
  scenario "release with prereleases off" normal 0 "$V2" any
fi

say "$( [ $FAILS = 0 ] && echo "OK: all passed" || echo "FAILED: $FAILS" )"
[ $FAILS = 0 ]
