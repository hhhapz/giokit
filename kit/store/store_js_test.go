package store

import (
	"syscall/js"
	"testing"
)

func TestWeb(t *testing.T) {
	js.Global().Set("localStorage", js.Global().Call("eval", `(() => {
		const m = new Map();
		return {
			getItem: k => m.has(k) ? m.get(k) : null,
			setItem: (k, v) => { if (v === "throw") throw new Error("QuotaExceededError"); m.set(k, String(v)); },
		};
	})()`))
	st, err := Open("app")
	if err != nil {
		t.Fatal(err)
	}
	testStore(t, st)
	if v := js.Global().Get("localStorage").Call("getItem", "app/a").String(); v != "two" {
		t.Fatalf(`localStorage "app/a" = %q`, v)
	}
	if st.Save("a", []byte("throw")) == nil {
		t.Fatal("a throwing setItem should be an error, not a panic")
	}
	js.Global().Delete("localStorage")
	if _, err := Open("app"); err == nil {
		t.Fatal("Open without localStorage should fail")
	}
}
