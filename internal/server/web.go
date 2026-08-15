package server

import (
	"embed"
	"io/fs"
	"net/http"
)

func webHandler(webFS embed.FS) http.Handler {
	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		panic(err)
	}
	return http.FileServer(http.FS(sub))
}
