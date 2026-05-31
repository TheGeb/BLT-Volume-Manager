package version

var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

func String() string {
	if Version != "dev" {
		return Version + " (" + Commit + ", " + Date + ")"
	}
	return Version
}
