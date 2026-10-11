package main

// The Echo test store's website (issue #17): a tiny local shop served by the
// plugin itself, so the embedded store view can be tried with no network.
//
//	ECHO_STORE_PORT    port for the local store page (default 18427, the
//	                   address plugins/echo/plugin.json declares)
//
// The storefront is three pages: an index (/store/) with links to try
// navigation, a game page (/store/game/<id>), and a page outside the
// declared domains (served on 127.0.0.2, linked from the index) to show the
// address strip's warning tint. A "popup" link (target=_blank) tries the
// popup policy. Nothing loads off-machine.

import (
	"fmt"
	"html"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
)

// defaultStorePort is the port plugins/echo/plugin.json declares.
const defaultStorePort = "18427"

var (
	storeOnce   sync.Once
	storeBase   string // "" when the port is taken: no local storefront then
	outsideBase string // the outside-domains demo page, "" when unavailable
)

func storeBaseURL() string {
	storeOnce.Do(startStoreServer)
	return storeBase
}

func startStoreServer() {
	port := os.Getenv("ECHO_STORE_PORT")
	if strings.TrimSpace(port) == "" {
		port = defaultStorePort
	}
	ln, err := net.Listen("tcp", "127.0.0.1:"+port)
	if err != nil {
		return
	}
	storeBase = "http://" + ln.Addr().String()
	// The outside-domains demo: another loopback address, so navigating to
	// it needs no network but leaves the declared domains (127.0.0.1).
	if oln, err := net.Listen("tcp", "127.0.0.2:0"); err == nil {
		outsideBase = "http://" + oln.Addr().String() + "/outside"
		omux := http.NewServeMux()
		omux.HandleFunc("/outside", func(w http.ResponseWriter, r *http.Request) {
			writeStorePage(w, "Outside the Echo store",
				`<p>This page is outside the store's declared domains, so the address strip warns about it.</p>`+
					`<p><a href="`+html.EscapeString(storeBase+`/store/`)+`">Back to the Echo store</a></p>`)
		})
		go func() { _ = http.Serve(oln, omux) }()
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/store/", storeHandler)
	go func() { _ = http.Serve(ln, mux) }()
}

func storeHandler(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/store/")
	switch {
	case rest == "" || rest == "/":
		links := `<ul>`
		for _, id := range order {
			links += `<li><a href="` + html.EscapeString(storeBase+`/store/game/`+id) + `">` +
				html.EscapeString(games[id].title) + `</a></li>`
		}
		links += `</ul>`
		links += `<p><a href="` + html.EscapeString(storeBase+`/store/page2`) + `">More games</a> · ` +
			`<a href="` + html.EscapeString(storeBase+`/store/popup`) + `" target="_blank" rel="opener">Open a popup</a></p>`
		if outsideBase != "" {
			links += `<p><a href="` + html.EscapeString(outsideBase) + `">Leave the store's domains (warning demo)</a></p>`
		}
		writeStorePage(w, "Echo Test Store",
			`<p>A tiny local shop for trying the embedded store view. No network involved.</p>`+links)
	case rest == "page2" || rest == "/page2":
		writeStorePage(w, "Echo Test Store — more",
			`<p>There is nothing else. Well, almost.</p>`+
				`<p><a href="`+html.EscapeString(storeBase+`/store/`)+`">Back to the store front</a></p>`)
	case rest == "popup" || rest == "/popup":
		writeStorePage(w, "Echo popup",
			`<p>A popup the store opened. It stays in the store's session, e.g. for login or checkout.</p>`)
	case strings.HasPrefix(rest, "game/"):
		id := strings.TrimPrefix(rest, "game/")
		g, ok := games[id]
		if !ok {
			http.NotFound(w, r)
			return
		}
		writeStorePage(w, g.title,
			`<p>A tiny fake game from the Echo test store.</p>`+
				`<p>Size: `+html.EscapeString(fmt.Sprintf("%d MiB", g.size>>20))+`</p>`+
				`<p><a href="`+html.EscapeString(storeBase+`/store/`)+`">Back to the store front</a></p>`)
	default:
		http.NotFound(w, r)
	}
}

func writeStorePage(w http.ResponseWriter, title, body string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, "<!doctype html><html><head><meta charset=utf-8><title>%s</title></head>"+
		"<body><h1>%s</h1>%s</body></html>\n",
		html.EscapeString(title), html.EscapeString(title), body)
}
