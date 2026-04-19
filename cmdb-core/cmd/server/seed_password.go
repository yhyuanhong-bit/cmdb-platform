package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// seedPasswordDirEnv lets operators override the directory where the
// seeded admin password is persisted. Defaults to /tmp which matches
// the deployment convention captured in
// docs/reports/audit-2026-04-19/REMEDIATION-ROADMAP.md §0.6. If /tmp is
// not writable (e.g. read-only container filesystem), the helper falls
// back to the current working directory.
const seedPasswordDirEnv = "SEED_PASSWORD_DIR"

// writeSeedPasswordToFile persists the freshly seeded admin password to
// a 0600 file on disk and returns the absolute path. The password is
// NEVER logged — callers must only log the returned path and the
// username. On failure the caller should Fatal without including the
// password in any log field.
//
// File layout:
//
//	/tmp/seed-admin-password-<RFC3339-safe-timestamp>.txt
//	----
//	Username: <username>
//	Password: <password>
func writeSeedPasswordToFile(password, username string) (string, error) {
	dir := os.Getenv(seedPasswordDirEnv)
	if dir == "" {
		dir = "/tmp"
	}

	// Ensure the directory exists and is usable. If not, fall back to
	// the current working directory so startup does not crash on
	// read-only /tmp mounts.
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		if cwd, cwdErr := os.Getwd(); cwdErr == nil {
			dir = cwd
		} else {
			return "", fmt.Errorf("seed password: target dir %q unusable and cwd lookup failed: %w", dir, cwdErr)
		}
	}

	// RFC3339 contains ':' which is fine on Linux but awkward in paths;
	// swap to '-' to stay portable and shell-friendly.
	ts := time.Now().UTC().Format(time.RFC3339)
	ts = sanitizeTimestamp(ts)

	path := filepath.Join(dir, fmt.Sprintf("seed-admin-password-%s.txt", ts))

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return "", fmt.Errorf("seed password: open %s: %w", path, err)
	}
	defer f.Close()

	payload := fmt.Sprintf("Username: %s\nPassword: %s\n", username, password)
	if _, err := f.WriteString(payload); err != nil {
		// Best-effort cleanup so we don't leave a half-written file on disk.
		_ = os.Remove(path)
		return "", fmt.Errorf("seed password: write %s: %w", path, err)
	}

	return path, nil
}

// sanitizeTimestamp replaces characters that are awkward in file paths
// (currently just ':') with '-'. Kept separate so the timestamp format
// itself remains RFC3339 for anyone parsing filenames later.
func sanitizeTimestamp(ts string) string {
	out := make([]byte, 0, len(ts))
	for i := 0; i < len(ts); i++ {
		c := ts[i]
		if c == ':' {
			out = append(out, '-')
			continue
		}
		out = append(out, c)
	}
	return string(out)
}
