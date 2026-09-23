package filesystem

import "embed"

// Static is a filesystem to serve static files
//
//go:embed ca
var CA embed.FS
