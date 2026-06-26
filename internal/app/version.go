package app

var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

func VersionString() string {
	if Version != "dev" {
		return Version + " (" + Commit + ", " + Date + ")"
	}
	return Version
}
