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
#   --fork-rev REV       the fork's commit to build the new versions with (default: HEAD)
#   --worktree           the fork's working tree instead, uncommitted changes too
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
v1="" v2="" new_style="" rev=HEAD worktree=false keep=0
while [ $# -gt 0 ]; do
  case $1 in
    --v1) v1=$2; shift ;;
    --v2) v2=$2; shift ;;
    --new-style) new_style=$2; shift ;;
    --fork-rev) rev=$2; shift ;;
    --worktree) worktree=true ;;
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
mkdir -p "$stage/old" "$stage/fork"
set -o pipefail
# The app's vendored MyGo at the tag, as it is.
git -C "$repo" archive "$tag" third_party/mygo | tar -x -C "$stage/old" --strip-components=2
[ -f "$stage/old/go.mod" ] || { echo "$app $tag has no third_party/mygo" >&2; exit 2; }
# The fork.
if $worktree; then
  rsync -a --exclude .git --exclude node_modules --exclude /docs --exclude /docs-zh --exclude /website \
    --exclude /benchmarks --exclude /examples --exclude upchain-out "$fork/" "$stage/fork/"
  forkdesc="$(git -C "$fork" rev-parse --short HEAD) + working tree"
else
  git -C "$fork" archive "$rev" | tar -x -C "$stage/fork" --anchored --exclude docs --exclude docs-zh --exclude website --exclude benchmarks --exclude examples
  forkdesc=$(git -C "$fork" rev-parse --short "$rev")
fi
cp -r "$here" "$stage/upchain"
has_release=0
grep -q 'func VerifyRelease' "$stage/old/internal/update/"*.go && has_release=1
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
KEEP=$keep
EOF
echo "upchain $id: $app $tag ($old, $old_style) → $v1 → $v2 (fork $forkdesc, $new_style) on $host"

root="code/mygo-upd/upchain"
remote="$root/$id"
# The trees go to a cache on the Mac, by content, each sent against the
# newest one already there (rsync's --link-dest: only what changed crosses
# the network); mac.sh copies them into the run.
forkkey="fork-$(cd "$stage/fork" && find . -type f -print0 | sort -z | xargs -0 sha1sum | sha1sum | cut -c1-12)"
oldkey="old-$feed-$tag"
ssh_mac() { ssh -o BatchMode=yes -o ConnectTimeout=60 -o ServerAliveInterval=30 "$host" "$@"; }
sync_cache() { # sync_cache <local dir> <key> <kind>
  local base
  base=$(ssh_mac "cd '$root/cache' 2>/dev/null && ls -td $3-*/ 2>/dev/null | head -1" || true)
  if ssh_mac "test -f '$root/cache/$2/.complete'"; then return; fi
  rsync -a --delete --timeout=300 ${base:+--link-dest="../${base%/}"} "$1/" "$host:$root/cache/$2/"
  ssh_mac "touch '$root/cache/$2/.complete'; cd '$root/cache' && ls -td $3-*/ | tail -n +5 | xargs rm -rf"
}
ssh_mac "mkdir -p '$remote' '$root/cache'"
sync_cache "$stage/fork" "$forkkey" fork
sync_cache "$stage/old" "$oldkey" old
printf 'FORK_CACHE=%s\nOLD_CACHE=%s\n' "../cache/$forkkey" "../cache/$oldkey" >>"$stage/run.env"
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
