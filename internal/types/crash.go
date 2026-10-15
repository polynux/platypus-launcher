package types

// CrashHint is one suggestion for a game's last abnormal run (issue #46). Rule
// names the pattern that matched; the UI words it. Param is the file name the
// log line named, when it named one. Verb is a winetricks verb that fixes it
// (known verbs only). Action is one-click help the UI offers: "runner" or
// "anticheat", or "" for an explanation only.
type CrashHint struct {
	Rule   string `json:"rule"`
	Param  string `json:"param,omitempty"`
	Verb   string `json:"verb,omitempty"`
	Action string `json:"action,omitempty"`
}

// CrashHints is the suggestion set for a game's last abnormal run. Log is the
// session log the hints were read from. Winetricks says whether winetricks is
// on this machine, so the UI offers the install buttons only then.
type CrashHints struct {
	Log        string      `json:"log"`
	Winetricks bool        `json:"winetricks"`
	Hints      []CrashHint `json:"hints"`
}
