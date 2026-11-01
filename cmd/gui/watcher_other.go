//go:build !windows

package main

func findRunningGame(watchList []GameProcessDef) (*GameProcessDef, string) {
	return nil, ""
}
