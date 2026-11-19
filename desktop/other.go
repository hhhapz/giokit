//go:build (!linux && !windows && !darwin) || android || ios

package desktop

import "context"

// system is a system without an implementation: every call returns
// ErrUnsupported.
type system struct{}

func (system) OpenFile(context.Context, FileOptions) ([]string, error) { return nil, ErrUnsupported }
func (system) SaveFile(context.Context, SaveOptions) (string, error)   { return "", ErrUnsupported }
func (system) OpenURL(context.Context, string) error                   { return ErrUnsupported }
