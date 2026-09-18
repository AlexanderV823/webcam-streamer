// Package embed отвечает за встраивание ресурсов статического фронтенда внутрь исполняемого файла Go.
package embed

import "embed"

// WebUI содержит встроенную файловую систему с index.html и style.css.
// Линтер примет маску для нескольких типов файлов.
//
//go:embed index.html style.css
var WebUI embed.FS
