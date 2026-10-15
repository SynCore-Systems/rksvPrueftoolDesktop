// Package assets holds the large binaries embedded into the executable.
// scripts/build.sh downloads them (pinned versions and checksums) next to
// this file before compiling; they are never committed.
package assets

// JRE is the zipped Java 8 runtime (Windows builds only, else empty).
var JRE []byte

// BMFEngine is the official BMF verification zip, embedded only in the
// internal build (build tag "bundled"); empty in the public build.
var BMFEngine []byte
