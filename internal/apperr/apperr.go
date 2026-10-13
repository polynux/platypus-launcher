// Package apperr carries user-facing core errors to the UI in a
// translatable form (issue #39, phase 3).
//
// Errors from the Go core used to reach the UI as plain English strings.
// A coded error keeps that English text as its message (logs and older
// shells are unchanged) and adds a stable dotted code plus string params,
// which corerpc puts into the JSON-RPC error's data field. The UI looks
// up errors:<code> in its catalog and falls back to the message.
package apperr

import (
	"errors"
	"maps"
)

// Error is a user-facing error: Code names it (settings.folder.notAbsolute),
// Params carries already-formatted values (paths, titles), Msg is the
// English text (log text and UI fallback), and Err is the wrapped cause so
// errors.Is and errors.As keep working.
type Error struct {
	Code   string
	Params map[string]string
	Msg    string
	Err    error
}

// New makes a user-facing error with an explicit English message.
func New(code, msg string, params map[string]string) *Error {
	return &Error{Code: code, Params: params, Msg: msg}
}

// Wrap makes a user-facing error out of a cause: the message stays the
// cause's text, so logs and older shells see exactly what they did before.
func Wrap(code string, err error, params map[string]string) *Error {
	return &Error{Code: code, Params: params, Msg: err.Error(), Err: err}
}

func (e *Error) Error() string {
	if e.Msg != "" {
		return e.Msg
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Code
}

// Unwrap exposes the cause, so errors.Is and errors.As see through.
func (e *Error) Unwrap() error { return e.Err }

// CodeOf reports the outermost coded error in err's chain, or "".
func CodeOf(err error) string {
	var ae *Error
	if errors.As(err, &ae) {
		return ae.Code
	}
	return ""
}

// ParamsOf reports the outermost coded error's params in err's chain, or nil.
func ParamsOf(err error) map[string]string {
	var ae *Error
	if errors.As(err, &ae) {
		return maps.Clone(ae.Params)
	}
	return nil
}
