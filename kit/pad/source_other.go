//go:build (!linux && !windows && !js && !(darwin && cgo)) || android || ios

package pad

// platformSource is nil where giokit has no controller support yet:
// Pads then reports no controllers.
func platformSource() Source { return nil }
