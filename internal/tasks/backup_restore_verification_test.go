package tasks

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestBackupRestoreVerificationScript(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping backup/restore verification in short mode")
	}
	if runtime.GOOS == "windows" {
		t.Skip("backup/restore verification script requires bash")
	}
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	if _, err := exec.LookPath("sqlite3"); err != nil {
		t.Skip("sqlite3 not available")
	}

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to resolve test file path")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
	cmd := exec.Command("bash", "./scripts/verify_backup_restore.sh")
	cmd.Dir = repoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("verify_backup_restore.sh failed: %v\noutput:\n%s", err, string(output))
	}
}
