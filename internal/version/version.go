package version

// These are set via -ldflags -X at build time by Makefile / GoReleaser / PKGBUILD.
var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

func String() string {
	if Commit == "unknown" {
		return Version
	}
	return Version + " (" + Commit + " " + Date + ")"
}
