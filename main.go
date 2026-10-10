package main

import (
	"embed"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"platypus/internal/steam"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// WebKitGTK's DMA-BUF renderer fails on some drivers (NVIDIA on Wayland:
	// "Error 71 (Protocol error) dispatching to Wayland display"). GPU
	// compositing still works without it, so turn it off unless the user chose.
	if _, set := os.LookupEnv("WEBKIT_DISABLE_DMABUF_RENDERER"); !set {
		_ = os.Setenv("WEBKIT_DISABLE_DMABUF_RENDERER", "1")
	}
	// Create an instance of the app structure
	app := NewApp()

	// Create application with options
	err := wails.Run(&options.App{
		Title:  "Platypus",
		Width:  1024,
		Height: 768,
		AssetServer: &assetserver.Options{
			Assets:     assets,
			Middleware: assetserver.ChainMiddleware(steamArt, app.customArt),
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		Linux:            &linux.Options{WebviewGpuPolicy: gpuPolicy(os.Getenv("PLATYPUS_GPU"))},
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

// gpuPolicy picks WebKitGTK's hardware acceleration. Always composites on the
// GPU, so scrolling doesn't repaint the whole window on the CPU (a free-spinning
// mouse wheel stutters otherwise). Wails' own default, used before, is never.
// PLATYPUS_GPU=never (or ondemand) is the way out on a driver where
// accelerated WebKitGTK misbehaves.
func gpuPolicy(v string) linux.WebviewGpuPolicy {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "ondemand", "on-demand":
		return linux.WebviewGpuPolicyOnDemand
	case "never", "off":
		return linux.WebviewGpuPolicyNever
	default:
		return linux.WebviewGpuPolicyAlways
	}
}
