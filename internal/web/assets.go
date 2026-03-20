package web

import (
	"embed"
	"io/fs"
)

//go:embed all:static
var staticFS embed.FS

var Static fs.FS

func init() {
	var err error
	Static, err = fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}
}
