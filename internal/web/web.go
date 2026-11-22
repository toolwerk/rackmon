// Package web bettet das Dashboard in das Programm ein.
package web

import "embed"

// Files enthält die Dateien des Dashboards unter static/.
//
//go:embed static
var Files embed.FS
