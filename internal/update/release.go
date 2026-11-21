package update

import (
	"crypto/ed25519"
	"errors"
	"fmt"
	"strings"
)

// Release signatures. The signature of an archive (Manifest.Signature)
// covers its SHA-256 alone, not the version the manifest says it is, so
// whoever controls the feed, or the network, could offer any archive ever
// signed as any version: an old one with a known hole as newer, or a beta
// to the users of releases. A release signature (Manifest.Release,
// Delta.Release) signs ReleaseMessage: the digest with the version it
// makes, the channel of that version and, for a delta, the version it
// updates from. `mygo build` writes both signatures; the updater checks the
// release signature only and refuses a manifest without one.

// Channels: a version with a pre-release (1.3.0-beta.1, 1.3.0-rc.2) is
// published as a beta, others as stable.
const (
	ChannelStable = "stable"
	ChannelBeta   = "beta"
)

// IsPrerelease reports whether a version such as 1.3.0-beta.1 is a
// prerelease, which Compare orders before its release.
func IsPrerelease(version string) bool {
	version, _, _ = strings.Cut(strings.TrimPrefix(version, "v"), "+")
	_, pre, _ := strings.Cut(version, "-")
	return pre != ""
}

// Channel is the channel version is published as.
func Channel(version string) string {
	if IsPrerelease(version) {
		return ChannelBeta
	}
	return ChannelStable
}

// Takes reports whether an app of version current takes an update to
// offered: a newer version, which is a release unless the app runs a
// prerelease itself or its user chose prereleases (prereleases). An app of
// a beta takes the next beta and the release after it; an app of a release
// takes betas only while prereleases is on.
func Takes(current, offered string, prereleases bool) bool {
	return Compare(offered, current) > 0 &&
		(Channel(offered) == ChannelStable || Channel(current) == ChannelBeta || prereleases)
}

// ReleaseMessage is what a release signature signs: the digest of an
// archive, or of a delta from version from ("" for an archive), with the
// version it makes and that version's channel.
func ReleaseMessage(version, from string, digest []byte) []byte {
	return fmt.Appendf(nil, "mygo-update-release\nversion=%s\nchannel=%s\nfrom=%s\nsha256=%x\n",
		strings.TrimPrefix(version, "v"), Channel(version), strings.TrimPrefix(from, "v"), digest)
}

// SignRelease signs ReleaseMessage.
func SignRelease(key ed25519.PrivateKey, version, from string, digest []byte) string {
	return Sign(key, ReleaseMessage(version, from, digest))
}

// VerifyRelease checks a release signature.
func VerifyRelease(key ed25519.PublicKey, version, from string, digest []byte, signature string) error {
	if signature == "" {
		return errors.New("the update is not signed for its version")
	}
	if err := Verify(key, ReleaseMessage(version, from, digest), signature); err != nil {
		return errors.New("the update is not signed with the app's key for its version")
	}
	return nil
}
