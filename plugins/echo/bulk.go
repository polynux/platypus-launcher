package main

// A large library for measuring the UI at scale (ECHO_GAMES=1500). Nothing here
// is a protocol feature: echo only lists more games, and serves their covers
// itself so the launcher loads real images without a network.
//
//	ECHO_GAMES         number of extra games to list (default 0: none)
//	ECHO_GAMES_PORT    port for the local cover server (default: any free one)
//
// Every fourth bulk game is installed (it points at a folder that exists), the
// rest are owned only, like a store library next to a few local installs. The
// covers are 600x900 JPEGs from a handful of variants: close to what a store's
// CDN sends, so decode and paint costs are realistic.

import (
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
)

const coverVariants = 12

var (
	bulkOnce sync.Once
	bulkBase string
	bulkMu   sync.Mutex
	bulkJPEG = map[[3]int][]byte{}
)

// bulkCount is how many extra games ECHO_GAMES asks for.
func bulkCount() int {
	n, err := strconv.Atoi(strings.TrimSpace(os.Getenv("ECHO_GAMES")))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

func bulkID(i int) string { return fmt.Sprintf("bulk-%04d", i) }

// bulkGames lists the extra games as library.list entries.
func bulkGames() []map[string]any {
	n := bulkCount()
	if n == 0 {
		return nil
	}
	bulkOnce.Do(startCoverServer)
	list := make([]map[string]any, 0, n)
	for i := 0; i < n; i++ {
		installed := i%4 == 0
		e := map[string]any{
			"id": bulkID(i), "title": fmt.Sprintf("Bulk Game %04d %s", i, bulkWords[i%len(bulkWords)]),
			"installed": installed, "installable": !installed, "version": "1.0", "native": i%7 == 0,
			"sizeBytes": int64(1+i%40) << 28,
		}
		if bulkBase != "" {
			e["coverUrl"] = fmt.Sprintf("%s/cover/%d.jpg", bulkBase, i%coverVariants)
		}
		if installed {
			e["installPath"] = os.TempDir()
		}
		list = append(list, e)
	}
	return list
}

var bulkWords = []string{"of the Deep", "Reborn", "Legacy", "Chronicles", "Tactics", "Odyssey", "Frontier", "Rising", "Echoes", "Tides"}

func startCoverServer() {
	addr := "127.0.0.1:" + os.Getenv("ECHO_GAMES_PORT")
	if os.Getenv("ECHO_GAMES_PORT") == "" {
		addr = "127.0.0.1:0"
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return
	}
	bulkBase = "http://" + ln.Addr().String()
	mux := http.NewServeMux()
	mux.HandleFunc("/cover/", func(w http.ResponseWriter, r *http.Request) {
		v, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/cover/"), ".jpg"))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		_, _ = w.Write(coverJPEG(v))
	})
	mux.HandleFunc("/shot/", func(w http.ResponseWriter, r *http.Request) {
		v, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/shot/"), ".jpg"))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		_, _ = w.Write(gradientJPEG(v, 1280, 720))
	})
	go func() { _ = http.Serve(ln, mux) }()
}

// coverJPEG is a 600x900 gradient with some structure, so it does not compress to nothing.
func coverJPEG(v int) []byte { return gradientJPEG(v, 600, 900) }

// gradientJPEG is a w x h gradient for variant v (covers and screenshots).
func gradientJPEG(v, w, h int) []byte {
	key := [3]int{v, w, h}
	bulkMu.Lock()
	defer bulkMu.Unlock()
	if b, ok := bulkJPEG[key]; ok {
		return b
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r := uint8((x*255/w + v*37) % 256)
			g := uint8((y*255/h + v*91) % 256)
			b := uint8(((x ^ y) + v*13) % 256)
			if (x/40+y/40)%2 == 0 {
				r, g = g/2, r/2
			}
			img.Set(x, y, color.RGBA{r, g, b, 255})
		}
	}
	var out sliceWriter
	_ = jpeg.Encode(&out, img, &jpeg.Options{Quality: 82})
	bulkJPEG[key] = out.b
	return out.b
}

type sliceWriter struct{ b []byte }

func (s *sliceWriter) Write(p []byte) (int, error) { s.b = append(s.b, p...); return len(p), nil }
