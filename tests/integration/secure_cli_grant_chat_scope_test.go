//go:build integration

package integration

// Per-grant chat scoping tests.
// Verifies:
//   1. (binary, agent, chat=NULL) + (binary, agent, chat=X) can coexist (new unique constraint)
//   2. Two grants with chat=NULL for same (binary, agent) still violate uniqueness (backwards-compat)
//   3. LookupByBinary returns the most-specific enabled grant: chat-specific > NULL default
//   4. LookupByBinary with empty/unmatched chatID falls back to the NULL default grant
//   5. LookupByBinary on a non-global binary with only a chat-specific grant returns
//      that grant only when the chat matches (otherwise blocked)

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/store"
	"github.com/nextlevelbuilder/goclaw/internal/store/pg"
)

// seedRestrictedBinary creates a non-global (is_global=false) binary so grants
// are required to access it. Mirrors seedSecureCLI but flips is_global.
func seedRestrictedBinary(t *testing.T, db *sql.DB, tenantID uuid.UUID) uuid.UUID {
	t.Helper()
	binaryID := uuid.New()
	name := "test-restricted-" + binaryID.String()[:8]
	_, err := db.Exec(
		`INSERT INTO secure_cli_binaries (id, tenant_id, binary_name, encrypted_env, description, enabled, is_global)
		 VALUES ($1, $2, $3, $4, 'test restricted CLI', true, false)`,
		binaryID, tenantID, name, []byte(`{}`),
	)
	if err != nil {
		t.Fatalf("seed restricted binary: %v", err)
	}
	t.Cleanup(func() {
		db.Exec("DELETE FROM secure_cli_agent_grants WHERE binary_id = $1", binaryID)
		db.Exec("DELETE FROM secure_cli_binaries WHERE id = $1", binaryID)
	})
	return binaryID
}

// createGrantWithEnv inserts a grant with the given chat_id (nil = NULL) and
// plaintext env. Returns the grant ID. Env is encrypted by UpdateGrantEnv.
func createGrantWithEnv(t *testing.T, gs *pg.PGSecureCLIAgentGrantStore, ctx context.Context,
	binaryID, agentID uuid.UUID, chatID *string, env map[string]string) uuid.UUID {
	t.Helper()
	g := &store.SecureCLIAgentGrant{
		BinaryID: binaryID,
		AgentID:  agentID,
		ChatID:   chatID,
		Enabled:  true,
	}
	if err := gs.Create(ctx, g); err != nil {
		t.Fatalf("create grant (chat=%v): %v", chatID, err)
	}
	if len(env) > 0 {
		envJSON, _ := json.Marshal(env)
		if err := gs.UpdateGrantEnv(ctx, g.ID, envJSON); err != nil {
			t.Fatalf("set grant env: %v", err)
		}
	}
	return g.ID
}

// envFromLookup decodes the resolved env from a LookupByBinary result.
func envFromLookup(t *testing.T, cred *store.SecureCLIBinary) map[string]string {
	t.Helper()
	if cred == nil || len(cred.EncryptedEnv) == 0 {
		return nil
	}
	var m map[string]string
	if err := json.Unmarshal(cred.EncryptedEnv, &m); err != nil {
		t.Fatalf("unmarshal env: %v", err)
	}
	return m
}

// TestGrantChatScope_UniquenessAllowsCoexistence ensures the new unique index
// allows one default grant and many chat-specific grants per (binary, agent, tenant).
func TestGrantChatScope_UniquenessAllowsCoexistence(t *testing.T) {
	t.Parallel()

	db := testDB(t)
	tenantID, agentID := seedTenantAgent(t, db)
	binaryID := seedSecureCLI(t, db, tenantID)
	ctx := store.WithTenantID(context.Background(), tenantID)
	gs := pg.NewPGSecureCLIAgentGrantStore(db, testEncryptionKey)

	// Default (chat_id = NULL) grant.
	createGrantWithEnv(t, gs, ctx, binaryID, agentID, nil, map[string]string{"K": "default"})

	// Chat-specific grant (chat_id = "group-A") — must NOT collide.
	chatA := "group-A"
	createGrantWithEnv(t, gs, ctx, binaryID, agentID, &chatA, map[string]string{"K": "A"})

	// Another chat-specific grant — also fine.
	chatB := "group-B"
	createGrantWithEnv(t, gs, ctx, binaryID, agentID, &chatB, map[string]string{"K": "B"})

	// Second NULL grant for the same (binary, agent, tenant) must violate the
	// COALESCE(chat_id, '') uniqueness — backwards-compatible: one default per agent.
	dup := &store.SecureCLIAgentGrant{
		BinaryID: binaryID,
		AgentID:  agentID,
		ChatID:   nil,
		Enabled:  true,
	}
	if err := gs.Create(ctx, dup); err == nil {
		t.Fatal("expected unique-violation creating a second NULL-chat grant; got no error")
	} else if !strings.Contains(strings.ToLower(err.Error()), "duplicate") &&
		!strings.Contains(strings.ToLower(err.Error()), "unique") {
		t.Fatalf("unique-violation expected, got: %v", err)
	}
}

// TestGrantChatScope_SpecificWinsOverDefault verifies the resolution order:
// when both a chat-specific grant and a NULL default exist, the chat-specific
// one is returned for matching chatID; the default is returned for empty or
// non-matching chatID.
func TestGrantChatScope_SpecificWinsOverDefault(t *testing.T) {
	t.Parallel()

	db := testDB(t)
	tenantID, agentID := seedTenantAgent(t, db)
	binaryID := seedSecureCLI(t, db, tenantID)
	ctx := store.WithTenantID(context.Background(), tenantID)
	gs := pg.NewPGSecureCLIAgentGrantStore(db, testEncryptionKey)
	cliStore := pg.NewPGSecureCLIStore(db, testEncryptionKey)

	createGrantWithEnv(t, gs, ctx, binaryID, agentID, nil, map[string]string{"K": "default"})
	chatA := "group-A"
	createGrantWithEnv(t, gs, ctx, binaryID, agentID, &chatA, map[string]string{"K": "A"})

	// Fetch the binary name to use in LookupByBinary.
	var name string
	if err := db.QueryRow(`SELECT binary_name FROM secure_cli_binaries WHERE id = $1`, binaryID).Scan(&name); err != nil {
		t.Fatalf("get binary name: %v", err)
	}

	cases := []struct {
		chatID string
		wantK  string
	}{
		{chatA, "A"},        // exact match → chat-specific grant wins
		{"group-B", "default"}, // no chat-B grant → falls back to NULL default
		{"", "default"},     // empty → no chat match, falls back to NULL default
	}
	for _, tc := range cases {
		cred, err := cliStore.LookupByBinary(ctx, name, &agentID, "", tc.chatID)
		if err != nil {
			t.Fatalf("LookupByBinary(chat=%q): %v", tc.chatID, err)
		}
		got := envFromLookup(t, cred)
		if got["K"] != tc.wantK {
			t.Errorf("chat=%q: env[K] = %q, want %q (full env: %v)", tc.chatID, got["K"], tc.wantK, got)
		}
	}
}

// TestGrantChatScope_NonGlobalBlocksWithoutMatchingGrant verifies that a
// non-global binary with ONLY a chat-specific grant blocks access from other
// chats (no fallback to "no grant = allowed").
func TestGrantChatScope_NonGlobalBlocksWithoutMatchingGrant(t *testing.T) {
	t.Parallel()

	db := testDB(t)
	tenantID, agentID := seedTenantAgent(t, db)
	binaryID := seedRestrictedBinary(t, db, tenantID)
	ctx := store.WithTenantID(context.Background(), tenantID)
	gs := pg.NewPGSecureCLIAgentGrantStore(db, testEncryptionKey)
	cliStore := pg.NewPGSecureCLIStore(db, testEncryptionKey)

	chatA := "group-A"
	createGrantWithEnv(t, gs, ctx, binaryID, agentID, &chatA, map[string]string{"K": "A"})

	var name string
	if err := db.QueryRow(`SELECT binary_name FROM secure_cli_binaries WHERE id = $1`, binaryID).Scan(&name); err != nil {
		t.Fatalf("get binary name: %v", err)
	}

	// Matching chat → access allowed, env A returned.
	if cred, err := cliStore.LookupByBinary(ctx, name, &agentID, "", chatA); err != nil {
		t.Fatalf("LookupByBinary(chat=A): %v", err)
	} else if cred == nil {
		t.Fatal("expected grant for chat=A on restricted binary, got nil")
	} else if envFromLookup(t, cred)["K"] != "A" {
		t.Errorf("expected env A, got %v", envFromLookup(t, cred))
	}

	// Non-matching chat → no grant → restricted binary blocked.
	if cred, err := cliStore.LookupByBinary(ctx, name, &agentID, "", "group-other"); err != nil {
		t.Fatalf("LookupByBinary(chat=other): %v", err)
	} else if cred != nil {
		t.Errorf("expected nil (blocked) for restricted binary with no matching grant, got %+v", cred)
	}

	// Empty chat → no match → blocked.
	if cred, err := cliStore.LookupByBinary(ctx, name, &agentID, "", ""); err != nil {
		t.Fatalf("LookupByBinary(chat=empty): %v", err)
	} else if cred != nil {
		t.Errorf("expected nil (blocked) for restricted binary with empty chat, got %+v", cred)
	}
}
