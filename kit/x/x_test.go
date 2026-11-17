package x

import (
	"testing"
)

func TestLogBufferRingAndFilter(t *testing.T) {
	b := LogBuffer{Cap: 5}
	for i := 0; i < 8; i++ {
		lvl := LogInfo
		if i%2 == 0 {
			lvl = LogError
		}
		b.Append(LogLine{Level: lvl, Text: "line " + string(rune('0'+i))})
	}
	if b.Len() != 5 || b.At(0).Text != "line 3" || b.At(4).Text != "line 7" {
		t.Fatalf("ring: len %d first %q last %q", b.Len(), b.At(0).Text, b.At(4).Text)
	}
	v := LogViewState{MinLevel: LogError}
	if n := v.Count(&b); n != 2 { // lines 4, 6
		t.Fatalf("error lines = %d", n)
	}
	b.Append(LogLine{Level: LogError, Text: "line 8"}, LogLine{Level: LogError, Text: "line 9"})
	if n := v.Count(&b); n != 3 { // 4 fell out; 6, 8, 9
		t.Fatalf("after eviction = %d", n)
	}
	v.Filter = "LINE 9"
	if n := v.Count(&b); n != 1 {
		t.Fatalf("text filter = %d", n)
	}
}
