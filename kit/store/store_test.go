package store

import (
	"os"
	"path/filepath"
	"testing"
)

// testStore runs the Store contract against st.
func testStore(t *testing.T, st Store) {
	t.Helper()
	if _, err := st.Load("a"); !IsNotExist(err) {
		t.Fatalf("Load before Save: %v, want not exist", err)
	}
	b := []byte("one")
	if err := st.Save("a", b); err != nil {
		t.Fatal(err)
	}
	b[0] = 'X' // the store keeps its own copy
	if got, err := st.Load("a"); err != nil || string(got) != "one" {
		t.Fatalf("Load = %q, %v; want one", got, err)
	}
	if err := st.Save("a", []byte("two")); err != nil {
		t.Fatal(err)
	}
	if got, _ := st.Load("a"); string(got) != "two" {
		t.Fatalf("after replacing: %q", got)
	}
	for _, bad := range []string{"", ".", "..", "x/y", `x\y`} {
		if st.Save(bad, nil) == nil {
			t.Errorf("Save(%q) accepted", bad)
		}
		if _, err := st.Load(bad); err == nil || IsNotExist(err) {
			t.Errorf("Load(%q): %v, want a bad-name error", bad, err)
		}
	}
}

func TestMemory(t *testing.T) { testStore(t, Memory()) }

func TestDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "app") // created on the first Save
	testStore(t, Dir(dir))
	if got, _ := Dir(dir).Load("a"); string(got) != "two" {
		t.Fatalf("a second Dir on the same directory sees %q", got)
	}
	es, _ := os.ReadDir(dir)
	if len(es) != 1 {
		t.Fatalf("temporary files left behind: %v", es)
	}
}
