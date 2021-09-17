package version

var (
	// NOTE: The $Format strings are replaced during 'git archive' thanks to the
	// companion .gitattributes file containing 'export-subst' in this same
	// directory.  See also https://git-scm.com/docs/gitattributes
	gitVersion   = "unreleased" // "v0.0.0-master+$Format:%h$"
	gitCommit    = ""           // sha1 from git, output of $(git rev-parse HEAD)
	gitTreeState = ""           // state of git tree, either "clean" or "dirty"

	buildDate   = "" // build date in ISO8601 format, output of $(date -u +'%Y-%m-%dT%H:%M:%SZ')
	environment = "local"
)

// Info represents metadata about current running instance
type Info struct {
	BuildDate    string `json:"BUILD_DATE"`
	Environment  string `json:"ENVIRONMENT"`
	GitCommit    string `json:"GIT_COMMIT"`
	GitTreeState string `json:"GIT_TREE_STATE"`
	GitVersion   string `json:"GIT_VERSION"`
}

// GetInfo is a helper function that retrieves metadata about the currently running instance
func GetInfo() Info {
	return Info{
		GitVersion:   gitVersion,
		GitCommit:    gitCommit,
		GitTreeState: gitTreeState,
		BuildDate:    buildDate,
		Environment:  environment,
	}
}
