package types

// RunnerEnv is what a game's runner choices add to its launch: the env vars, and
// the wrappers the launch puts in front of the game.
type RunnerEnv struct {
	Env      map[string]string `json:"env"`
	Wrappers []string          `json:"wrappers"`
}
