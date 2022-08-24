package version

// BuildDate is the flag that gets set at build time
// Set by go build -ldflags "-X" flag
var BuildDate string

// GitCommit is the flag that gets set at build time
// Set by go build -ldflags "-X" flag
var GitCommit string

// GitTreeState is the flag that gets set at build time
// Set by go build -ldflags "-X" flag
var GitTreeState string

// GitMajor is the flag that gets set at build time
// Set by go build -ldflags "-X" flag
var GitMajor string

// GitMinor is the flag that gets set at build time
// Set by go build -ldflags "-X" flag
var GitMinor string

// GitVersion is the flag that gets set to the git version at build time
// Set by go build -ldflags "-X" flag
var GitVersion = "development"

// GitReleaseCommit is the flag that gets set at build time
// Set by go build -ldflags "-X" flag
var GitReleaseCommit string
