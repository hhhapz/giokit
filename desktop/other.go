//go:build !linux || android

package desktop

import "context"

type system struct{}

func (system) OpenFile(context.Context, FileOptions) ([]string, error) { return nil, ErrUnsupported }
func (system) SaveFile(context.Context, SaveOptions) (string, error)   { return "", ErrUnsupported }
func (system) OpenURL(context.Context, string) error                   { return ErrUnsupported }
