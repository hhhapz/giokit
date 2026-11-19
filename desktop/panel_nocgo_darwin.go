//go:build darwin && !ios && !cgo

package desktop

import "context"

// OpenFile returns ErrUnsupported: the panels need cgo.
func (system) OpenFile(context.Context, FileOptions) ([]string, error) { return nil, ErrUnsupported }

// SaveFile returns ErrUnsupported: the panels need cgo.
func (system) SaveFile(context.Context, SaveOptions) (string, error) { return "", ErrUnsupported }
