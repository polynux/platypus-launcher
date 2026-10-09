package stores

import (
	"context"
	"errors"

	"hydra-launcher/internal/jsonrpc"
	"hydra-launcher/internal/plugins"
	"hydra-launcher/internal/types"
)

// Terms (protocol 1.11, the `terms` capability, docs/plugins/protocol.md
// "Terms"): the agreements a store lists in game.details, and the optional
// game.acceptTerms that tells a store the user accepted them.

// Terms asks the plugin for the game's terms. A plugin without game.details,
// or one that lists none, has no terms. The result is not checked against
// the user's acceptance; the caller marks it (types.MarkAccepted).
func (s *Plugin) Terms(ctx context.Context, storeAppID string) ([]types.GameTerm, error) {
	var d struct {
		Terms []plugins.Term `json:"terms"`
	}
	if err := s.m.Call(ctx, s.ID(), "game.details", map[string]any{"id": storeAppID}, &d); err != nil {
		if IsNotOffered(err) {
			return nil, nil
		}
		return nil, err
	}
	return termsFrom(d.Terms), nil
}

// termsFrom normalizes a plugin's terms: entries without an id or version are
// dropped (a version is what acceptance is keyed on), duplicate ids keep the
// first, and text is cut like the rest of the plugin's text.
func termsFrom(in []plugins.Term) []types.GameTerm {
	var out []types.GameTerm
	for _, t := range in {
		if t.ID == "" || t.Version == "" || hasTerm(out, t.ID) || len(out) >= maxTerms {
			continue
		}
		title := clipText(t.Title, maxCopyText)
		if title == "" {
			title = t.ID
		}
		out = append(out, types.GameTerm{ID: t.ID, Title: title, URL: webURL(t.URL), Version: clipText(t.Version, maxCopyText)})
	}
	return out
}

const maxTerms = 16

func hasTerm(list []types.GameTerm, id string) bool {
	for _, t := range list {
		if t.ID == id {
			return true
		}
	}
	return false
}

// AcceptTerms tells the store that the user accepted the terms (game.acceptTerms,
// optional). IsNotOffered is returned for a plugin that doesn't keep acceptance
// server-side; the caller then keeps the record in Hydra alone.
func (s *Plugin) AcceptTerms(ctx context.Context, storeAppID string, terms []types.TermRef) error {
	list := make([]map[string]string, 0, len(terms))
	for _, t := range terms {
		list = append(list, map[string]string{"id": t.ID, "version": t.Version})
	}
	return s.m.Call(ctx, s.ID(), "game.acceptTerms", map[string]any{"id": storeAppID, "terms": list}, nil)
}

// IsNotOffered reports whether err is the plugin's "method not found" answer
// (-32601): an optional method it doesn't implement.
func IsNotOffered(err error) bool {
	var rpcErr *jsonrpc.Error
	return errors.As(err, &rpcErr) && rpcErr.Code == jsonrpc.CodeMethodNotFound
}
