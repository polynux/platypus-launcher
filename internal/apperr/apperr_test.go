package apperr

import (
	"errors"
	"fmt"
	"testing"
)

func TestNewKeepsTheMessage(t *testing.T) {
	err := New("settings.year.invalid", "release year must be between 1970 and 2027, got 3000", map[string]string{"max": "2027"})
	if err.Error() != "release year must be between 1970 and 2027, got 3000" {
		t.Errorf("Error() = %q", err.Error())
	}
	if CodeOf(err) != "settings.year.invalid" {
		t.Errorf("CodeOf = %q", CodeOf(err))
	}
	if ParamsOf(err)["max"] != "2027" {
		t.Errorf("ParamsOf = %v", ParamsOf(err))
	}
}

func TestWrapKeepsTheCauseText(t *testing.T) {
	cause := errors.New("boom")
	err := Wrap("settings.workdir.invalid", cause, nil)
	if err.Error() != "boom" {
		t.Errorf("Error() = %q", err.Error())
	}
	if CodeOf(err) != "settings.workdir.invalid" {
		t.Errorf("CodeOf = %q", CodeOf(err))
	}
}

func TestIsAndAsSeeThrough(t *testing.T) {
	sentinel := errors.New("sentinel")
	wrapped := fmt.Errorf("outer: %w", Wrap("game.running", sentinel, map[string]string{"title": "X"}))
	if !errors.Is(wrapped, sentinel) {
		t.Error("errors.Is does not see through the coded error")
	}
	var ae *Error
	if !errors.As(wrapped, &ae) || ae.Code != "game.running" {
		t.Errorf("errors.As = %v", ae)
	}
	// The outermost code wins.
	outer := Wrap("move.noSpace", wrapped, nil)
	if CodeOf(outer) != "move.noSpace" {
		t.Errorf("CodeOf = %q", CodeOf(outer))
	}
}

func TestPlainErrorsHaveNoCode(t *testing.T) {
	if CodeOf(errors.New("plain")) != "" {
		t.Error("plain error reports a code")
	}
	if CodeOf(nil) != "" {
		t.Error("nil error reports a code")
	}
	if ParamsOf(errors.New("plain")) != nil {
		t.Error("plain error reports params")
	}
}
