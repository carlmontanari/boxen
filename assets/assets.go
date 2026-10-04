package assets

import "embed"

// Assets contains the included profiles and their embedded companion files.
//
//go:embed profiles/*
var Assets embed.FS
