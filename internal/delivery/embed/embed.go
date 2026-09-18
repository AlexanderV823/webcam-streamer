package embed

import "embed"

// WebUI содержит встроенную файловую систему с index.html
//
//go:embed index.html
var WebUI embed.FS
