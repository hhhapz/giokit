// Package bench holds rendering baselines for text-heavy UIs (editor, terminal).
// Run: go test -run x -bench . -benchtime 100x ./internal/bench
package bench

import "image"

func imagePt(x, y int) image.Point { return image.Pt(x, y) }
