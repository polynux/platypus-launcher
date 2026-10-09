package plugins

import (
	"context"
	"io"

	"hydra-launcher/internal/jsonrpc"
)

// DefaultTransport is what a manifest that names none (every legacy "api": 1
// manifest) uses.
const DefaultTransport = "jsonrpc-stdio"

// Conn is a live link to one plugin run: requests out, notifications in
// (delivered to the NotifyFunc given to Connect), closed when the run ends.
// *jsonrpc.Conn is the first implementation.
type Conn interface {
	// Call sends a request and decodes its result. An error answered by the
	// plugin is a *jsonrpc.Error whatever the wire format.
	Call(ctx context.Context, method string, params, result any) error
	// Done is closed when the link is gone.
	Done() <-chan struct{}
	Close()
}

// Transport is how Hydra talks to a plugin process. The manifest selects it
// by name (protocol.transport); a new wire format is a new implementation
// registered here, not a change to the process lifecycle, the capability
// gate or anything above them. Today there is one: JSON-RPC over the plugin's
// stdin and stdout, one JSON object per line.
type Transport interface {
	// Name is the value of protocol.transport in a manifest.
	Name() string
	// Connect attaches to a freshly started plugin: r is what the plugin
	// writes (its stdout), w what it reads (its stdin).
	Connect(r io.Reader, w io.Writer, notify jsonrpc.NotifyFunc) Conn
}

var transports = map[string]Transport{}

// RegisterTransport makes a transport selectable from manifests.
func registerTransport(t Transport) { transports[t.Name()] = t }

// LookupTransport finds a transport by its manifest name.
func LookupTransport(name string) (Transport, bool) {
	t, ok := transports[name]
	return t, ok
}

type stdioJSONRPC struct{}

func (stdioJSONRPC) Name() string { return DefaultTransport }

func (stdioJSONRPC) Connect(r io.Reader, w io.Writer, notify jsonrpc.NotifyFunc) Conn {
	return jsonrpc.NewConn(r, w, notify)
}

func init() { registerTransport(stdioJSONRPC{}) }
