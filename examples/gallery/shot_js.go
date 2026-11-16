package main

import "errors"

func renderShot(*gallery, string) error {
	return errors.New("-shot needs a headless GPU; it is not available in the browser")
}
