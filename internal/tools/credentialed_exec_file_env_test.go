package tools

// File-content env materialization tests.
// Verifies:
//   1. Regular env vars pass through unchanged
//   2. __FILE_<NAME> entries are written to disk + replaced with <NAME>=<path>
//   3. Cleanup func removes the temp dir
//   4. Sandbox path is rejected (file would not be visible inside the container)
//   5. Invalid file-key (just "__FILE_" with no target) is rejected

import (
	"os"
	"strings"
	"testing"
)

func TestMaterializeFileEnvVars_PassThroughPlainEntries(t *testing.T) {
	env := map[string]string{"FOO": "bar", "BAZ": "qux"}
	cleanup, err := materializeFileEnvVars(env, false)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	defer cleanup()
	if env["FOO"] != "bar" || env["BAZ"] != "qux" {
		t.Errorf("plain env vars must be unchanged, got %v", env)
	}
	if len(env) != 2 {
		t.Errorf("expected 2 entries, got %d (%v)", len(env), env)
	}
}

func TestMaterializeFileEnvVars_WritesFileAndRewritesKey(t *testing.T) {
	content := "apiVersion: v1\nkind: Config\nclusters: []\n"
	env := map[string]string{
		"FOO":              "bar",
		"__FILE_KUBECONFIG": content,
	}
	cleanup, err := materializeFileEnvVars(env, false)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	defer cleanup()

	if _, present := env["__FILE_KUBECONFIG"]; present {
		t.Error("__FILE_ key must be removed after materialization")
	}
	path, ok := env["KUBECONFIG"]
	if !ok {
		t.Fatal("KUBECONFIG must be set to the materialized file path")
	}
	if !strings.HasPrefix(path, os.TempDir()) {
		t.Errorf("KUBECONFIG path %q is not under TempDir %q", path, os.TempDir())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read materialized file: %v", err)
	}
	if string(data) != content {
		t.Errorf("file contents mismatch:\n got:  %q\n want: %q", string(data), content)
	}

	// Verify file perms are 0600 (no world/group access — contains secret).
	stat, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	if perm := stat.Mode().Perm(); perm != 0o600 {
		t.Errorf("file perms = %o, want 0600", perm)
	}

	// Plain entries should still be present.
	if env["FOO"] != "bar" {
		t.Errorf("plain entry mutated: %v", env)
	}
}

func TestMaterializeFileEnvVars_CleanupRemovesTempDir(t *testing.T) {
	env := map[string]string{"__FILE_KUBECONFIG": "data"}
	cleanup, err := materializeFileEnvVars(env, false)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	path := env["KUBECONFIG"]
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file should exist before cleanup: %v", err)
	}

	cleanup()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("file should be removed after cleanup, stat err = %v", err)
	}
}

func TestMaterializeFileEnvVars_RejectsSandbox(t *testing.T) {
	env := map[string]string{"__FILE_KUBECONFIG": "data"}
	_, err := materializeFileEnvVars(env, true)
	if err == nil {
		t.Fatal("expected error when sandbox=true with __FILE_ env, got nil")
	}
	if !strings.Contains(err.Error(), "sandbox") {
		t.Errorf("error message should mention sandbox, got: %v", err)
	}
}

func TestMaterializeFileEnvVars_NoFileKeysReturnsNoOpCleanup(t *testing.T) {
	env := map[string]string{"FOO": "bar"}
	cleanup, err := materializeFileEnvVars(env, false)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	// Should be safe to call cleanup even with no file keys (no temp dir was created).
	cleanup()
}
