package assets

import "embed"

// Assets is the embedded assets objects for the included profile yaml data.
//
//go:embed profiles/*.yaml
var Assets embed.FS
