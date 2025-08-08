package version

// These values are injected at build time via -ldflags.
// Defaults are for local dev builds.
var (
    Version = "dev"
    Commit  = ""
    Date    = ""
)


