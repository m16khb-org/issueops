package installcli

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func assertManagedCommandFixtureContract(t *testing.T) {
	t.Helper()
	assertManagedCommandFixtureBuildsOnceForIsolatedCopies(t)
	assertManagedCommandFixturePropagatesFirstBuildFailure(t)
	assertManagedCommandFixtureCopiesRealBinary(t)
}

func assertManagedCommandFixtureBuildsOnceForIsolatedCopies(t *testing.T) {
	t.Helper()
	want := []byte("managed command fixture\n")
	wantHash := sha256.Sum256(want)
	builds := 0
	fixture, cleanup, err := newManagedTestCommandFixture(func(target string) error {
		builds++
		if err := os.WriteFile(target, want, 0o755); err != nil {
			return err
		}
		return os.Chmod(target, 0o755)
	})
	if err != nil {
		t.Fatalf("create counted managed command fixture: %v", err)
	}
	defer func() {
		if err := cleanup(); err != nil {
			t.Errorf("clean counted managed command fixture: %v", err)
		}
	}()

	destinations := make([]string, 14)
	for index := range destinations {
		destinations[index] = filepath.Join(t.TempDir(), "issueops")
	}
	var wait sync.WaitGroup
	errorsByCopy := make(chan error, len(destinations))
	for _, destination := range destinations {
		wait.Add(1)
		go func() {
			defer wait.Done()
			errorsByCopy <- fixture.copyTo(destination)
		}()
	}
	wait.Wait()
	close(errorsByCopy)
	for copyErr := range errorsByCopy {
		if copyErr != nil {
			t.Fatalf("copy counted managed command fixture: %v", copyErr)
		}
	}
	if builds != 1 {
		t.Fatalf("managed command fixture builds = %d, want 1 for 14 copies", builds)
	}
	for _, destination := range destinations {
		body, readErr := os.ReadFile(destination)
		info, statErr := os.Stat(destination)
		if readErr != nil || statErr != nil {
			t.Fatalf("read isolated copy %s: readErr=%v statErr=%v", destination, readErr, statErr)
		}
		if gotHash := sha256.Sum256(body); gotHash != wantHash || info.Mode().Perm() != 0o755 {
			t.Fatalf("isolated copy %s: hash=%x wantHash=%x mode=%v", destination, gotHash, wantHash, info.Mode())
		}
	}
	if err := os.WriteFile(destinations[0], []byte("corrupt\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, path := range append([]string{fixture.source}, destinations[1:]...) {
		body, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(body, want) {
			t.Fatalf("corrupting one copy changed %s: body=%q err=%v", path, body, err)
		}
	}
}

func assertManagedCommandFixturePropagatesFirstBuildFailure(t *testing.T) {
	t.Helper()
	wantErr := errors.New("injected first build failure")
	builds := 0
	failedTarget := ""
	runCalled := false
	reported := error(nil)
	exitCode := runWithManagedCommandFixture(
		func(target string) error {
			builds++
			failedTarget = target
			return wantErr
		},
		func(managedCommandFixture) int {
			runCalled = true
			return 0
		},
		func(err error) { reported = err },
	)
	if exitCode == 0 || builds != 1 || runCalled || !errors.Is(reported, wantErr) {
		t.Fatalf("failed build result: exit=%d builds=%d runCalled=%v reported=%v", exitCode, builds, runCalled, reported)
	}
	if _, err := os.Stat(filepath.Dir(failedTarget)); !os.IsNotExist(err) {
		t.Fatalf("failed first build left fixture directory: target=%s err=%v", failedTarget, err)
	}
}

func assertManagedCommandFixtureCopiesRealBinary(t *testing.T) {
	t.Helper()
	first := filepath.Join(t.TempDir(), "issueops")
	second := filepath.Join(t.TempDir(), "issueops")
	buildManagedCommandAt(t, first)
	buildManagedCommandAt(t, second)

	sourceInfo, err := os.Stat(managedTestCommandSource.source)
	if err != nil {
		t.Fatal(err)
	}
	sourceBody, err := os.ReadFile(managedTestCommandSource.source)
	if err != nil {
		t.Fatal(err)
	}
	sourceHash := sha256.Sum256(sourceBody)
	for _, path := range []string{first, second} {
		body, readErr := os.ReadFile(path)
		info, statErr := os.Stat(path)
		if readErr != nil || statErr != nil {
			t.Fatalf("read real managed command copy %s: readErr=%v statErr=%v", path, readErr, statErr)
		}
		if gotHash := sha256.Sum256(body); gotHash != sourceHash || info.Mode().Perm() != sourceInfo.Mode().Perm() || info.Mode().Perm()&0o111 == 0 {
			t.Fatalf("real managed command copy %s: hash=%x wantHash=%x mode=%v wantMode=%v", path, gotHash, sourceHash, info.Mode(), sourceInfo.Mode())
		}
		if os.SameFile(sourceInfo, info) {
			t.Fatalf("real managed command copy %s shares the source inode", path)
		}
	}
	if err := os.WriteFile(first, []byte("corrupt\n"), sourceInfo.Mode().Perm()); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{managedTestCommandSource.source, second} {
		body, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(body, sourceBody) {
			t.Fatalf("corrupting real copy changed %s: equal=%v err=%v", path, bytes.Equal(body, sourceBody), err)
		}
	}
}
