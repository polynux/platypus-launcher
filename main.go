package main

import (
	"embed"
	"net/http"
	"path/filepath"
	"strings"

	"hydra-launcher/internal/steam"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Create an instance of the app structure
	app := NewApp()

	// Create application with options
	err := wails.Run(&options.App{
		Title:  "hydra-launcher",
		Width:  1024,
		Height: 768,
		AssetServer: &assetserver.Options{
			Assets:     assets,
			Middleware: assetserver.ChainMiddleware(steamArt, app.customArt),
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}

// steamArt serves images from Steam's local library cache under
// steam.ArtURLPrefix. Newer titles only have hashed CDN URLs, but the Steam
// client has already downloaded their art.
func steamArt(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rel, ok := strings.CutPrefix(r.URL.Path, steam.ArtURLPrefix)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}
		root := steam.Root()
		clean := filepath.Clean("/" + rel) // blocks ../ escapes
		ext := strings.ToLower(filepath.Ext(clean))
		if root == "" || (ext != ".jpg" && ext != ".png") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "max-age=86400")
		http.ServeFile(w, r, filepath.Join(root, "appcache/librarycache", clean))
	})
}
