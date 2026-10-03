//go:build !embedui

package web

import "io/fs"

// UI returns nil in builds without the embedui tag; Vite serves the frontend.
func UI() fs.FS { return nil }
