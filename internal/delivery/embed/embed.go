// Package embed отвечает за встраивание ресурсов статического фронтенда внутрь исполняемого файла Go.
package embed

import "embed"

// WebUI содержит встроенную файловую систему с index.html
//
//go:embed index.html
var WebUI embed.FS
