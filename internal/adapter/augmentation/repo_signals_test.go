package augmentation

import (
	"io/fs"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestCollectSignalsHoldOnThisRepository pins that every repository signal is true for this
// repository. The signal rules search for Go identifiers as text, so renaming an identifier
// without updating its search term silently turns a signal false; the temp-dir fixtures in
// the other tests cannot see that.
func TestCollectSignalsHoldOnThisRepository(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	repo := Repository{ListDocs: func(root string) []string {
		var docs []string
		_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if entry.IsDir() && entry.Name() == ".git" {
				return filepath.SkipDir
			}
			if strings.HasSuffix(path, ".md") {
				docs = append(docs, path)
			}
			return nil
		})
		return docs
	}}
	signals := repo.CollectSignals(root, 0, []string{"self-augment"}, "genius")
	value := reflect.ValueOf(signals)
	for i := 0; i < value.NumField(); i++ {
		if value.Field(i).Kind() == reflect.Bool && !value.Field(i).Bool() {
			t.Errorf("signal %s is false for this repository", value.Type().Field(i).Name)
		}
	}
}
