package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestExec_WorkingDir_InsideAllowedSkillsPath verifies that after
// AllowPaths(skillsDir) is wired, an exec call with working_dir pointing
// inside that skills directory is accepted by the validator. Without the
// fix, this returned "access denied: path outside workspace" because the
// shell tool passed nil to allowedWriteWithTeamWorkspace.
func TestExec_WorkingDir_InsideAllowedSkillsPath(t *testing.T) {
	wsDir := t.TempDir()
	skillsDir := t.TempDir() // simulate /app/data/skills-store

	// Create a nested skill dir + a no-op script so the command actually runs.
	skillSubdir := filepath.Join(skillsDir, "openrouter-analytics", "1")
	if err := os.MkdirAll(skillSubdir, 0o755); err != nil {
		t.Fatalf("setup skill dir: %v", err)
	}

	exec := NewExecTool(wsDir, true) // restrict = true; the strict path
	exec.AllowPaths(skillsDir)        // simulates gateway_tools_wiring.AllowPaths

	result := exec.Execute(context.Background(), map[string]any{
		"command":     "true", // any command that exits 0; we only care about path validation
		"working_dir": skillSubdir,
	})

	if result.IsError && strings.Contains(result.ForLLM, "path outside workspace") {
		t.Fatalf("working_dir inside AllowPaths-registered skill dir must not be rejected as outside workspace; got: %s", result.ForLLM)
	}
}

// TestExec_WorkingDir_OutsideEverything_StillDenied confirms the fix doesn't
// accidentally turn the validator into a no-op — a path outside both the
// workspace and the allowed prefixes must still return the access-denied error.
func TestExec_WorkingDir_OutsideEverything_StillDenied(t *testing.T) {
	wsDir := t.TempDir()
	skillsDir := t.TempDir()

	exec := NewExecTool(wsDir, true)
	exec.AllowPaths(skillsDir)

	// /tmp is not the workspace and not the skill dir.
	stranger := t.TempDir()

	result := exec.Execute(context.Background(), map[string]any{
		"command":     "true",
		"working_dir": stranger,
	})

	if !result.IsError {
		t.Fatalf("working_dir outside workspace and outside AllowPaths must be rejected; got non-error result")
	}
	if !strings.Contains(result.ForLLM, "path outside workspace") {
		t.Errorf("expected 'path outside workspace' error, got: %s", result.ForLLM)
	}
}

// TestExec_WorkingDir_NoAllowPathsCall_KeepsStrictDefault confirms that an
// ExecTool built without AllowPaths still rejects skill-dir-shaped paths.
// Guards against accidentally widening the default behavior — the fix only
// takes effect when the caller (gateway_tools_wiring.go) opts in.
func TestExec_WorkingDir_NoAllowPathsCall_KeepsStrictDefault(t *testing.T) {
	wsDir := t.TempDir()
	skillsDir := t.TempDir()
	skillSubdir := filepath.Join(skillsDir, "any-skill", "1")
	if err := os.MkdirAll(skillSubdir, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}

	exec := NewExecTool(wsDir, true)
	// No AllowPaths call — strict default.

	result := exec.Execute(context.Background(), map[string]any{
		"command":     "true",
		"working_dir": skillSubdir,
	})

	if !result.IsError {
		t.Fatal("without AllowPaths, a skill-dir working_dir must still be rejected")
	}
	if !strings.Contains(result.ForLLM, "path outside workspace") {
		t.Errorf("expected 'path outside workspace', got: %s", result.ForLLM)
	}
}

// TestExec_AllowPaths_AccumulatesAcrossCalls verifies the AllowPaths setter
// appends rather than replacing — gateway_tools_wiring.go calls it twice
// (once with skillsAllowPaths, once with userAllowPaths), and both lists
// must remain valid.
func TestExec_AllowPaths_AccumulatesAcrossCalls(t *testing.T) {
	exec := NewExecTool(t.TempDir(), true)
	exec.AllowPaths("/a/path/one")
	exec.AllowPaths("/a/path/two", "/a/path/three")

	if len(exec.allowedPrefixes) != 3 {
		t.Fatalf("AllowPaths should append; got %d prefixes, want 3", len(exec.allowedPrefixes))
	}
}
