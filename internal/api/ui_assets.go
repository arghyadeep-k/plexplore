package api

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed assets/leaflet/* assets/leaflet/images/* assets/app/*
var embeddedUIAssets embed.FS

func registerUIAssetRoutes(mux *http.ServeMux) {
	assetsFS, err := fs.Sub(embeddedUIAssets, "assets")
	if err != nil {
		panic("ui assets fs setup failed: " + err.Error())
	}
	handler := http.StripPrefix("/ui/assets/", http.FileServer(http.FS(assetsFS)))
	mux.Handle("GET /ui/assets/{path...}", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setCommonSecurityHeaders(w)
		handler.ServeHTTP(w, r)
	}))
}
