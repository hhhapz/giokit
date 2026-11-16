// Package bench holds rendering baselines for text-heavy UIs (editor, terminal).
// Run: go test -run x -bench . -benchtime 100x ./internal/bench
// Baseline (Nov 2026, RX 6700 XT, 5800X): see docs/roadmap.md "Text-heavy apps".
package bench

import "image"

func imagePt(x, y int) image.Point { return image.Pt(x, y) }
