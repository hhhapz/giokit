package desktop

import "testing"

func TestCheckLink(t *testing.T) {
	for uri, ok := range map[string]bool{
		"https://github.com/hhhapz/scry": true, "http://example.com": true,
		"mailto:a@example.com": true, "file:///tmp/x.txt": true, "HTTPS://EXAMPLE.COM": true,
		"-a Calculator": false, "C:\\Windows\\notepad.exe": false, "calc.exe": false,
		"javascript:alert(1)": false, "": false, "/usr/bin/xterm": false,
	} {
		if err := checkLink(uri); (err == nil) != ok {
			t.Errorf("checkLink(%q) = %v, want ok=%v", uri, err, ok)
		}
	}
}
