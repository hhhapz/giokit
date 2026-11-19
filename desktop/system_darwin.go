//go:build darwin && !ios

package desktop

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// system is macOS: NSOpenPanel and NSSavePanel through cgo
// (panel_darwin.go; ErrUnsupported when built without cgo), and open(1)
// for links.
type system struct{}

// OpenURL runs open(1), which hands url to its default browser, mail
// program or file viewer, and waits for open itself (not the application)
// to exit. Ending ctx kills open. checkLink makes sure url is a link, so
// open cannot read it as a flag or a program.
func (system) OpenURL(ctx context.Context, url string) error {
	if err := checkLink(url); err != nil {
		return err
	}
	if out, err := exec.CommandContext(ctx, "open", url).CombinedOutput(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("desktop: open %q: %w: %s", url, err, strings.TrimSpace(string(out)))
	}
	return nil
}
