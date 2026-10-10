package main

import (
	"context"
	"embed"
	"net/http"
	"os"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"platypus/internal/app"
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
	application := app.NewApp()

	// Create application with options
	err := wails.Run(&options.App{
		Title:  "Platypus",
		Width:  1024,
		Height: 768,
		AssetServer: &assetserver.Options{
			Assets: assets,
			Middleware: assetserver.ChainMiddleware(app.SteamArt, func(next http.Handler) http.Handler {
				return app.CustomArt(application, next)
			}),
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		Linux:            &linux.Options{WebviewGpuPolicy: gpuPolicy(os.Getenv("PLATYPUS_GPU"))},
		OnStartup: func(ctx context.Context) {
			app.SetHost(application, wailsHost{ctx})
			app.Startup(application, ctx)
		},
		OnShutdown: func(ctx context.Context) { app.Shutdown(application, ctx) },
		Bind: []interface{}{
			application,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
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
