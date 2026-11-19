package desktop

import (
	"fmt"
	"net/url"
	"strings"
)

// linkSchemes are the schemes OpenURL hands to the system on platforms
// that open links through a shell call (Windows, macOS): a link, never a
// program path or a command-line flag.
var linkSchemes = map[string]bool{"http": true, "https": true, "mailto": true, "file": true}

// checkLink reports whether uri is an absolute link OpenURL may open.
func checkLink(uri string) error {
	u, err := url.Parse(uri)
	if err != nil {
		return fmt.Errorf("desktop: open %q: %w", uri, err)
	}
	if !linkSchemes[strings.ToLower(u.Scheme)] {
		return fmt.Errorf("desktop: open %q: only http, https, mailto and file links", uri)
	}
	return nil
}
