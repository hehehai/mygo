#!/bin/bash
# The update chain check (README.md): from the published old version of an
# app, built by its vendored MyGo as it is, to versions built by MyGo's
# fork, on mac-air, in the background. Run it on the Linux box from the
# fork's repository:
#
#   actnow/upchain/upchain.sh <App> <old tag> [options]
#
#   --v1 VERSION         the next beta (default: the old one + 1, e.g. 0.1.0-beta.10)
#   --v2 VERSION         the release after it (default: the old one's, e.g. 0.1.0)
#   --new-style STYLE    url-beta or prereleases (default: apps.conf)
#   --fork-rev REV       the fork's commit to build the new versions with
#                        (default: HEAD; it must be pushed to github.com/hehehai/mygo,
#                        which the Mac fetches it from)
#   --keep               keep the run's files on the Mac
#   --host HOST          the Mac (default: mac-air)
#
# The report and the logs come back to $UPCHAIN_OUT (default
# ./upchain-out)/<run>; it exits 0 when every check passed.
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
fork=$(git -C "$here" rev-parse --show-toplevel)
helper=${UPCHAIN_REPOS:-$HOME/helper}
host=mac-air
app=${1:?usage: upchain.sh <App> <old tag> [options]}
tag=${2:?usage: upchain.sh <App> <old tag> [options]}
shift 2
v1="" v2="" new_style="" rev=HEAD keep=0
while [ $# -gt 0 ]; do
  case $1 in
    --v1) v1=$2; shift ;;
    --v2) v2=$2; shift ;;
    --new-style) new_style=$2; shift ;;
    --fork-rev) rev=$2; shift ;;
    --keep) keep=1 ;;
    --host) host=$2; shift ;;
    *) echo "unknown option $1" >&2; exit 2 ;;
  esac
  shift
done

read -r _ feed target old_style conf_new_style < <(awk -v a="$app" '$1 == a' "$here/apps.conf") ||
  { echo "$app is not in apps.conf" >&2; exit 2; }
new_style=${new_style:-$conf_new_style}
repo="$helper/$app"
git -C "$repo" rev-parse -q --verify "$tag^{commit}" >/dev/null || { echo "$repo has no tag $tag" >&2; exit 2; }
config=$(git -C "$repo" show "$tag:mygo.json")
name=$(jq -r .name <<<"$config")
old=$(jq -r .version <<<"$config")
[ "v$old" = "$tag" ] || echo "note: $tag builds version $old" >&2

# The next beta and the release: 0.1.0-beta.9 → 0.1.0-beta.10, 0.1.0;
# 1.2.3 → 1.2.4-beta.1, 1.2.4.
if [[ $old == *-* ]]; then
  pre=${old#*-}
  n=${pre##*.}
  [[ $n =~ ^[0-9]+$ ]] && v1=${v1:-${old%.*}.$((n + 1))} || v1=${v1:-$old.1}
  v2=${v2:-${old%%-*}}
else
  IFS=. read -r a b c <<<"$old"
  v1=${v1:-$a.$b.$((c + 1))-beta.1}
  v2=${v2:-$a.$b.$((c + 1))}
fi

id="$feed-$tag-$(date +%m%d-%H%M%S)"
stage=$(mktemp -d "/tmp/upchain-$id.XXXX")
trap 'rm -rf "$stage"' EXIT
mkdir -p "$stage/old" "$stage/full"
# The app's vendored MyGo at the tag, as it is: only the files of the
# packages the probe and cmd/mygo build from (uploads to the Mac are slow).
git -C "$repo" archive "$tag" third_party/mygo | tar -x -C "$stage/full" --strip-components=2
[ -f "$stage/full/go.mod" ] || { echo "$app $tag has no third_party/mygo" >&2; exit 2; }
(
  cd "$stage/full"
  export GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 GOFLAGS=-mod=mod
  go list -deps -f '{{if .Module}}{{if eq .Module.Path "github.com/egoist/mygo"}}{{.Dir}}|{{join .EmbedFiles "|"}}{{end}}{{end}}' . ./cmd/mygo
) | while IFS='|' read -r dir embeds; do
  rel=${dir#"$stage/full"}; rel=${rel#/}
  mkdir -p "$stage/old/$rel"
  find "$dir" -maxdepth 1 -type f ! -name '*_test.go' -exec cp {} "$stage/old/$rel/" \;
  for e in $embeds; do [ -n "$e" ] && mkdir -p "$(dirname "$stage/old/$rel/$e")" && cp "$dir/$e" "$stage/old/$rel/$e"; done
done
cp "$stage/full/go.mod" "$stage/full/go.sum" "$stage/old/"
grep -q 'func VerifyRelease' "$stage/full/internal/update/"*.go && has_release=1 || has_release=0
grep -q 'func (u \*UpdaterModule) SetPrereleases' "$stage/full/"updater*.go && has_pre=1 || has_pre=0
rm -rf "$stage/full"
# The fork: a commit on GitHub, which the Mac fetches.
sha=$(git -C "$fork" rev-parse "$rev^{commit}")
git -C "$fork" fetch -q origin
git -C "$fork" branch -r --contains "$sha" | grep -q 'origin/' || { echo "$rev ($sha) is not pushed to origin: push it first" >&2; exit 2; }
forkdesc=$(git -C "$fork" rev-parse --short "$sha")
cp -r "$here" "$stage/upchain"
platform=${target/-//}
cat >"$stage/run.env" <<EOF
APP=$app
FEED=$feed
NAME='$name'
TGT=$target
PLATFORM=$platform
OLD_VERSION=$old
V1=$v1
V2=$v2
OLD_STYLE=$old_style
NEW_STYLE=$new_style
OLD_HAS_RELEASE=$has_release
OLD_HAS_PRE=$has_pre
FORK_SHA=$sha
KEEP=$keep
EOF
echo "upchain $id: $app $tag ($old, $old_style) → $v1 → $v2 (fork $forkdesc, $new_style) on $host"

root="code/mygo-upd/upchain"
remote="$root/$id"
# The old tree goes to a cache on the Mac (once per app and tag); the fork
# is fetched there from GitHub by mac.sh.
oldkey="old-$feed-$tag"
ssh_mac() { ssh -o BatchMode=yes -o ConnectTimeout=60 -o ServerAliveInterval=30 "$host" "$@"; }
ssh_mac "mkdir -p '$remote' '$root/cache'"
if ! ssh_mac "test -f '$root/cache/$oldkey/.complete'"; then
  rsync -a --delete --timeout=300 "$stage/old/" "$host:$root/cache/$oldkey/"
  ssh_mac "touch '$root/cache/$oldkey/.complete'"
fi
printf 'OLD_CACHE=%s\n' "../cache/$oldkey" >>"$stage/run.env"
rsync -a --timeout=300 "$stage/upchain" "$stage/run.env" "$host:$remote/"
status=0
# lockf: one Mac test at a time; setpgrp: the run's own process group,
# which its watchdog stops.
ssh_mac "cd '$remote' && lockf -k /tmp/mygo-mac.lock perl -e 'setpgrp(0,0); exec @ARGV' bash upchain/mac.sh ." || status=$?
out="${UPCHAIN_OUT:-$PWD/upchain-out}/$id"
mkdir -p "$out"
rsync -a "$host:$remote/report.txt" "$host:$remote/w/logs" "$host:$remote/w/requests.log" "$out/" 2>/dev/null || true
if [ "$keep" = 0 ]; then
  ssh_mac "rm -rf '$remote'"
fi
echo "report: $out/report.txt"
cat "$out/report.txt" 2>/dev/null || true
exit $status
