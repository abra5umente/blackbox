package frontend

import "embed"

// Assets contains the frontend build output served by Wails.
//go:embed dist/*
var Assets embed.FS
