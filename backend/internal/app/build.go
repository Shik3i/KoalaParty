package app

// These values are replaced by release builds through Go linker flags.
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

type BuildInformation struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"buildDate"`
}

func CurrentBuildInformation() BuildInformation {
	return BuildInformation{Version: Version, Commit: Commit, BuildDate: BuildDate}
}
