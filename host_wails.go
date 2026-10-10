//go:build !core

package main

import (
	"context"
	"strings"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"platypus/internal/app"
)

// wailsHost is the host of the Wails window; ctx is the one Wails passes to
// OnStartup.
type wailsHost struct{ ctx context.Context }

var _ app.Host = wailsHost{}

func (h wailsHost) Emit(event string, data ...any) { wruntime.EventsEmit(h.ctx, event, data...) }

func (h wailsHost) Infof(format string, args ...any) { wruntime.LogInfof(h.ctx, format, args...) }

func (h wailsHost) Warnf(format string, args ...any) { wruntime.LogWarningf(h.ctx, format, args...) }

func (h wailsHost) Errorf(format string, args ...any) { wruntime.LogErrorf(h.ctx, format, args...) }

func (h wailsHost) OpenFile(d app.FileDialog) (string, error) {
	o := wruntime.OpenDialogOptions{Title: d.Title}
	for _, f := range d.Filters {
		pats := make([]string, len(f.Extensions))
		for i, e := range f.Extensions {
			pats[i] = "*." + e
		}
		pattern := strings.Join(pats, ";")
		o.Filters = append(o.Filters, wruntime.FileFilter{DisplayName: f.Name + " (" + strings.ReplaceAll(pattern, ";", ", ") + ")", Pattern: pattern})
	}
	return wruntime.OpenFileDialog(h.ctx, o)
}

func (h wailsHost) OpenDirectory(d app.DirectoryDialog) (string, error) {
	return wruntime.OpenDirectoryDialog(h.ctx, wruntime.OpenDialogOptions{
		Title:                d.Title,
		DefaultDirectory:     d.Default,
		CanCreateDirectories: d.CanCreate,
	})
}
