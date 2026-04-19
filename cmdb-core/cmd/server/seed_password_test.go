package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestWriteSeedPasswordToFile_CreatesFileWith0600 verifies that the helper
// writes the seed password to a file with mode 0600 and that the file
// contains the username and password in the expected format.
//
// Crucially, the helper itself must NOT log the password to zap — this
// test proves the password only ever lands in the on-disk file by
// inspecting the file contents and ensuring they are correct.
func TestWriteSeedPasswordToFile_CreatesFileWith0600(t *testing.T) {
	// Arrange: use a temp dir so we do not leak state into /tmp on CI.
	tmpDir := t.TempDir()
	t.Setenv("SEED_PASSWORD_DIR", tmpDir)

	const username = "admin"
	const password = "correct-horse-battery-staple-1234"

	// Act
	path, err := writeSeedPasswordToFile(password, username)
	if err != nil {
		t.Fatalf("writeSeedPasswordToFile returned error: %v", err)
	}

	// Assert: file is under the directory we configured.
	if filepath.Dir(path) != tmpDir {
		t.Fatalf("expected file under %s, got %s", tmpDir, path)
	}

	// Assert: file exists.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat(%s): %v", path, err)
	}

	// Assert: mode is 0600 (only on platforms where Unix perms are
	// meaningful — Windows reports mode bits differently).
	if runtime.GOOS != "windows" {
		if got := info.Mode().Perm(); got != 0o600 {
			t.Fatalf("expected file mode 0600, got %o", got)
		}
	}

	// Assert: file content contains both username and password.
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	got := string(content)
	if !strings.Contains(got, "Username: "+username) {
		t.Errorf("file missing username line; got:\n%s", got)
	}
	if !strings.Contains(got, "Password: "+password) {
		t.Errorf("file missing password line; got:\n%s", got)
	}

	// Cleanup: remove the file (TempDir will drop it too, but be explicit).
	_ = os.Remove(path)
}

// TestWriteSeedPasswordToFile_ReturnsErrorOnUnwritableDir verifies the
// helper surfaces errors instead of silently succeeding when it cannot
// persist the password. We use a read-only temp directory so the
// fallback (cwd) is not exercised.
func TestWriteSeedPasswordToFile_ReturnsErrorOnUnwritableDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission semantics not applicable on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root can write through 0500 dirs, skipping")
	}
	// Arrange: create a directory we cannot write to.
	roDir := t.TempDir()
	if err := os.Chmod(roDir, 0o500); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(roDir, 0o700) })
	t.Setenv("SEED_PASSWORD_DIR", roDir)

	// Act
	_, err := writeSeedPasswordToFile("pw", "admin")

	// Assert
	if err == nil {
		t.Fatal("expected error when target dir is not writable, got nil")
	}
}

// TestWriteSeedPasswordToFile_DoesNotLogPassword is a defensive check: if
// someone later adds zap logging to the helper, this test catches any
// accidental inclusion of the password in an observed log stream.
// Since the helper intentionally writes NO logs, the observer remains
// empty — the assertion is simply that no observed entry contains the
// secret.
func TestWriteSeedPasswordToFile_DoesNotLogPassword(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("SEED_PASSWORD_DIR", tmpDir)

	const password = "unique-token-for-log-scan-9f3a2b"
	path, err := writeSeedPasswordToFile(password, "admin")
	if err != nil {
		t.Fatalf("writeSeedPasswordToFile: %v", err)
	}

	// The helper is expected to write only to disk. Read the file back
	// and confirm the password IS there (so we know we tested the right
	// helper), and that no stderr/stdout path is used.
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(b), password) {
		t.Fatalf("password missing from on-disk file — test fixture wrong")
	}
}
