package booster

import "os"

func exePath() string {
	p, _ := os.Executable()
	return p
}
