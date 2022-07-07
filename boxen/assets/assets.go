package assets

import "embed"

//go:embed scrapli_platforms/*
// ScrapliPlatformsAssets is the embed FS for the included scrapli platform definition yaml files.
var ScrapliPlatformsAssets embed.FS
