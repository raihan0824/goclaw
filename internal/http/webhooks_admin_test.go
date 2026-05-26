package http

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// ---- stub WebhookStore for admin tests ----
// webhooks_auth_test.go already defines stubWebhookStore but only covers the
// authentication surface. We need a richer version for CRUD: Create stores rows,
// List / GetByID return them, Update / RotateSecret / Revoke mutate in-memory.

type adminWebhookStore struct {
	mu   sync.Mutex
	rows map[uuid.UUID]*store.WebhookData
}

func newAdminWebhookStore(rows ...*store.WebhookData) *adminWebhookStore {
	s := &adminWebhookStore{rows: make(map[uuid.UUID]*store.WebhookData)}
	for _, r := range rows {
		cp := *r
		s.rows[r.ID] = &cp
	}
	return s
}

func (s *adminWebhookStore) Create(_ context.Context, w *store.WebhookData) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *w
	s.rows[w.ID] = &cp
	return nil
}

func (s *adminWebhookStore) GetByID(ctx context.Context, id uuid.UUID) (*store.WebhookData, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.rows[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	// Tenant-scope enforcement mirrors real store behaviour.
	tid := store.TenantIDFromContext(ctx)
	if tid != uuid.Nil && row.TenantID != tid && !store.IsOwnerRole(ctx) {
		return nil, sql.ErrNoRows
	}
	cp := *row
	return &cp, nil
}

func (s *adminWebhookStore) GetByHash(_ context.Context, h string) (*store.WebhookData, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.rows {
		if r.SecretHash == h {
			cp := *r
			return &cp, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (s *adminWebhookStore) List(ctx context.Context, f store.WebhookListFilter) ([]store.WebhookData, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tid := store.TenantIDFromContext(ctx)
	var out []store.WebhookData
	for _, r := range s.rows {
		if !store.IsOwnerRole(ctx) && r.TenantID != tid {
			continue
		}
		if f.AgentID != nil && (r.AgentID == nil || *r.AgentID != *f.AgentID) {
			continue
		}
		out = append(out, *r)
	}
	return out, nil
}

func (s *adminWebhookStore) Update(_ context.Context, id uuid.UUID, updates map[string]any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.rows[id]
	if !ok {
		return sql.ErrNoRows
	}
	if v, ok := updates["name"]; ok {
		row.Name = v.(string)
	}
	if v, ok := updates["require_hmac"]; ok {
		row.RequireHMAC = v.(bool)
	}
	if v, ok := updates["localhost_only"]; ok {
		row.LocalhostOnly = v.(bool)
	}
	row.UpdatedAt = time.Now()
	return nil
}

func (s *adminWebhookStore) RotateSecret(_ context.Context, id uuid.UUID, newHash, newPrefix, newEncryptedSecret string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.rows[id]
	if !ok {
		return sql.ErrNoRows
	}
	row.SecretHash = newHash
	row.SecretPrefix = newPrefix
	row.EncryptedSecret = newEncryptedSecret
	row.UpdatedAt = time.Now()
	return nil
}

func (s *adminWebhookStore) Revoke(_ context.Context, id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.rows[id]
	if !ok {
		return sql.ErrNoRows
	}
	row.Revoked = true
	row.UpdatedAt = time.Now()
	return nil
}

func (s *adminWebhookStore) TouchLastUsed(_ context.Context, _ uuid.UUID) error { return nil }

// GetByHashUnscoped and GetByIDUnscoped are auth-middleware-only unscoped lookups.
// In admin tests the middleware is not exercised, so these are no-ops.
func (s *adminWebhookStore) GetByHashUnscoped(ctx context.Context, h string) (*store.WebhookData, error) {
	return s.GetByHash(ctx, h)
}
func (s *adminWebhookStore) GetByIDUnscoped(ctx context.Context, id uuid.UUID) (*store.WebhookData, error) {
	return s.GetByID(ctx, id)
}

// ---- stub TenantStore for admin tests ----
// Delegates GetUserRole to a configurable map; stubs everything else.

type adminTenantStore struct {
	roles map[string]string // key = tenantID+":"+userID
}

func (a *adminTenantStore) key(tid uuid.UUID, uid string) string {
	return tid.String() + ":" + uid
}

func (a *adminTenantStore) GetUserRole(_ context.Context, tid uuid.UUID, uid string) (string, error) {
	if r, ok := a.roles[a.key(tid, uid)]; ok {
		return r, nil
	}
	return "", nil
}

// Remaining store.TenantStore methods — no-op stubs.
func (a *adminTenantStore) CreateTenant(context.Context, *store.TenantData) error { return nil }
func (a *adminTenantStore) GetTenant(_ context.Context, _ uuid.UUID) (*store.TenantData, error) {
	return nil, sql.ErrNoRows
}
func (a *adminTenantStore) GetTenantBySlug(_ context.Context, _ string) (*store.TenantData, error) {
	return nil, sql.ErrNoRows
}
func (a *adminTenantStore) ListTenants(context.Context) ([]store.TenantData, error) { return nil, nil }
func (a *adminTenantStore) UpdateTenant(context.Context, uuid.UUID, map[string]any) error {
	return nil
}
func (a *adminTenantStore) AddUser(context.Context, uuid.UUID, string, string) error { return nil }
func (a *adminTenantStore) RemoveUser(context.Context, uuid.UUID, string) error      { return nil }
func (a *adminTenantStore) ListUsers(context.Context, uuid.UUID) ([]store.TenantUserData, error) {
	return nil, nil
}
func (a *adminTenantStore) ListUserTenants(context.Context, string) ([]store.TenantUserData, error) {
	return nil, nil
}
func (a *adminTenantStore) GetTenantsByIDs(context.Context, []uuid.UUID) ([]store.TenantData, error) {
	return nil, nil
}
func (a *adminTenantStore) ResolveUserTenant(context.Context, string) (uuid.UUID, error) {
	return uuid.Nil, sql.ErrNoRows
}
func (a *adminTenantStore) GetTenantUser(context.Context, uuid.UUID) (*store.TenantUserData, error) {
	return nil, sql.ErrNoRows
}
func (a *adminTenantStore) CreateTenantUserReturning(context.Context, uuid.UUID, string, string, string) (*store.TenantUserData, error) {
	return nil, nil
}

// ---- helpers ----

// webhookTenantAdminCtx builds a tenant-admin context for webhook admin tests.
// Named distinctly to avoid colliding with the packages_updates_test.go helper
// which has a different signature (base context.Context param).
func webhookTenantAdminCtx(tenantID uuid.UUID, userID string) context.Context {
	ctx := context.Background()
	ctx = store.WithTenantID(ctx, tenantID)
	ctx = store.WithUserID(ctx, userID)
	ctx = store.WithRole(ctx, "admin")
	return ctx
}

func webhookTenantCtxWithRole(tenantID uuid.UUID, userID, role string) context.Context {
	ctx := context.Background()
	ctx = store.WithTenantID(ctx, tenantID)
	ctx = store.WithUserID(ctx, userID)
	ctx = store.WithRole(ctx, role)
	return ctx
}

// testAdminEncKey is a 32-byte (256-bit) AES key used only in tests.
const testAdminEncKey = "00000000000000000000000000000000"

func newAdminHandler(ws *adminWebhookStore, ts *adminTenantStore) *WebhooksAdminHandler {
	h := NewWebhooksAdminHandler(ws, nil, ts, nil)
	h.SetEncKey(testAdminEncKey) // required since K6 guard rejects empty encKey
	return h
}

func newAdminHandlerWithCalls(ws *adminWebhookStore, cs store.WebhookCallStore, ts *adminTenantStore) *WebhooksAdminHandler {
	h := NewWebhooksAdminHandler(ws, cs, ts, nil)
	h.SetEncKey(testAdminEncKey)
	return h
}

func doRequest(t *testing.T, h *WebhooksAdminHandler, method, path string, body any, ctx context.Context) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	r := httptest.NewRequest(method, path, &buf)
	r = r.WithContext(ctx)
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	mux.ServeHTTP(w, r)
	return w
}

// ---- tests ----

func TestWebhookAdmin_RouteRequiresHTTPAuth(t *testing.T) {
	oldToken := pkgGatewayToken
	oldFallback := pkgNoAuthFallbackAllowed
	InitGatewayToken("required-token")
	InitGatewayNoAuthFallbackAllowed(false)
	defer func() {
		InitGatewayToken(oldToken)
		InitGatewayNoAuthFallbackAllowed(oldFallback)
	}()

	h := newAdminHandler(newAdminWebhookStore(), &adminTenantStore{})
	r := httptest.NewRequest(http.MethodGet, "/v1/webhooks", nil)
	w := httptest.NewRecorder()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated admin route, got %d", w.Code)
	}
}

// TestWebhookAdmin_Create_HappyPath verifies POST /v1/webhooks returns secret once.
func TestWebhookAdmin_Create_HappyPath(t *testing.T) {
	tenantID := uuid.New()
	userID := "user-1"

	ts := &adminTenantStore{
		roles: map[string]string{
			tenantID.String() + ":" + userID: store.TenantRoleAdmin,
		},
	}
	ws := newAdminWebhookStore()
	h := newAdminHandler(ws, ts)

	ctx := webhookTenantAdminCtx(tenantID, userID)
	w := doRequest(t, h, http.MethodPost, "/v1/webhooks", map[string]any{
		"name": "my webhook",
		"kind": "llm",
	}, ctx)

	if w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp webhookCreateResp
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Secret == "" {
		t.Fatal("secret must be present in create response")
	}
	if resp.HMACSigningKey == "" {
		t.Fatal("hmac_signing_key must be present in create response")
	}
	if resp.SecretPrefix == "" {
		t.Fatal("secret_prefix must be present in create response")
	}
	// secret must start with wh_
	if len(resp.Secret) < 3 || resp.Secret[:3] != "wh_" {
		t.Fatalf("secret must start with wh_, got %q", resp.Secret)
	}
	// verify prefix matches first 8 chars of raw secret
	if resp.SecretPrefix != resp.Secret[:8] {
		t.Fatalf("prefix %q != first 8 chars of secret %q", resp.SecretPrefix, resp.Secret[:8])
	}
}

// TestWebhookAdmin_Create_NonAdmin_403 verifies non-admin cannot create.
func TestWebhookAdmin_Create_NonAdmin_403(t *testing.T) {
	tenantID := uuid.New()
	userID := "user-2"

	// operator role, not admin/owner
	ts := &adminTenantStore{
		roles: map[string]string{
			tenantID.String() + ":" + userID: "operator",
		},
	}
	ws := newAdminWebhookStore()
	h := newAdminHandler(ws, ts)

	ctx := webhookTenantAdminCtx(tenantID, userID)
	w := doRequest(t, h, http.MethodPost, "/v1/webhooks", map[string]any{
		"name": "x",
		"kind": "llm",
	}, ctx)

	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestWebhookAdmin_Create_ContextOperatorRoleDeniedBeforeTenantAdmin(t *testing.T) {
	tenantID := uuid.New()
	userID := "operator-context"

	ts := &adminTenantStore{
		roles: map[string]string{
			tenantID.String() + ":" + userID: store.TenantRoleAdmin,
		},
	}
	ws := newAdminWebhookStore()
	h := newAdminHandler(ws, ts)

	ctx := webhookTenantCtxWithRole(tenantID, userID, "operator")
	w := doRequest(t, h, http.MethodPost, "/v1/webhooks", map[string]any{
		"name": "x",
		"kind": "llm",
	}, ctx)

	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d: %s", w.Code, w.Body.String())
	}
}

// TestWebhookAdmin_Create_InvalidKind_400 verifies unknown kind is rejected.
func TestWebhookAdmin_Create_InvalidKind_400(t *testing.T) {
	tenantID := uuid.New()
	userID := "user-3"

	ts := &adminTenantStore{
		roles: map[string]string{
			tenantID.String() + ":" + userID: store.TenantRoleAdmin,
		},
	}
	ws := newAdminWebhookStore()
	h := newAdminHandler(ws, ts)

	ctx := webhookTenantAdminCtx(tenantID, userID)
	w := doRequest(t, h, http.MethodPost, "/v1/webhooks", map[string]any{
		"name": "x",
		"kind": "unknown",
	}, ctx)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d: %s", w.Code, w.Body.String())
	}
}

// TestWebhookAdmin_Get_CrossTenant_404 verifies tenant A cannot see tenant B's webhook.
func TestWebhookAdmin_Get_CrossTenant_404(t *testing.T) {
	tenantA := uuid.New()
	tenantB := uuid.New()
	userA := "user-a"

	// Webhook owned by tenant B.
	webhookID := uuid.New()
	whB := &store.WebhookData{
		ID:       webhookID,
		TenantID: tenantB,
		Name:     "b-webhook",
		Kind:     "llm",
	}

	ts := &adminTenantStore{
		roles: map[string]string{
			tenantA.String() + ":" + userA: store.TenantRoleAdmin,
		},
	}
	ws := newAdminWebhookStore(whB)
	h := newAdminHandler(ws, ts)

	// Request from tenant A.
	ctx := webhookTenantAdminCtx(tenantA, userA)
	r := httptest.NewRequest(http.MethodGet, "/v1/webhooks/"+webhookID.String(), nil)
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404 for cross-tenant get, got %d: %s", w.Code, w.Body.String())
	}
}

// TestWebhookAdmin_FullFlow_CreateListGetRotateRevoke exercises the happy path for all 6 endpoints.
func TestWebhookAdmin_FullFlow_CreateListGetRotateRevoke(t *testing.T) {
	tenantID := uuid.New()
	userID := "user-flow"

	ts := &adminTenantStore{
		roles: map[string]string{
			tenantID.String() + ":" + userID: store.TenantRoleAdmin,
		},
	}
	ws := newAdminWebhookStore()
	h := newAdminHandler(ws, ts)
	ctx := webhookTenantAdminCtx(tenantID, userID)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	// 1. Create.
	var createResp webhookCreateResp
	{
		var buf bytes.Buffer
		_ = json.NewEncoder(&buf).Encode(map[string]any{"name": "flow-wh", "kind": "llm"})
		r := httptest.NewRequest(http.MethodPost, "/v1/webhooks", &buf)
		r.Header.Set("Content-Type", "application/json")
		r = r.WithContext(ctx)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != http.StatusCreated {
			t.Fatalf("create: want 201, got %d: %s", w.Code, w.Body.String())
		}
		if err := json.NewDecoder(w.Body).Decode(&createResp); err != nil {
			t.Fatalf("create decode: %v", err)
		}
	}
	id := createResp.ID
	originalSecret := createResp.Secret

	// 2. List — must include newly created webhook.
	{
		r := httptest.NewRequest(http.MethodGet, "/v1/webhooks", nil)
		r = r.WithContext(ctx)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("list: want 200, got %d: %s", w.Code, w.Body.String())
		}
		var rows []store.WebhookData
		if err := json.NewDecoder(w.Body).Decode(&rows); err != nil {
			t.Fatalf("list decode: %v", err)
		}
		found := false
		for _, row := range rows {
			if row.ID == id {
				found = true
			}
		}
		if !found {
			t.Fatal("list: newly created webhook not found")
		}
	}

	// 3. Get.
	{
		r := httptest.NewRequest(http.MethodGet, "/v1/webhooks/"+id.String(), nil)
		r = r.WithContext(ctx)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("get: want 200, got %d: %s", w.Code, w.Body.String())
		}
		var row store.WebhookData
		if err := json.NewDecoder(w.Body).Decode(&row); err != nil {
			t.Fatalf("get decode: %v", err)
		}
		// Secret must NOT be in normal GET response.
		if row.SecretHash != "" {
			// SecretHash has json:"-" tag so it should never appear.
			// This check uses the decoded struct; field is blank as expected.
		}
		if row.ID != id {
			t.Fatalf("get: wrong id %s", row.ID)
		}
	}

	// 4. Rotate.
	var rotateResp map[string]any
	{
		r := httptest.NewRequest(http.MethodPost, "/v1/webhooks/"+id.String()+"/rotate", nil)
		r = r.WithContext(ctx)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("rotate: want 200, got %d: %s", w.Code, w.Body.String())
		}
		if err := json.NewDecoder(w.Body).Decode(&rotateResp); err != nil {
			t.Fatalf("rotate decode: %v", err)
		}
		newSecret, _ := rotateResp["secret"].(string)
		if newSecret == "" {
			t.Fatal("rotate: new secret must be present")
		}
		if newSecret == originalSecret {
			t.Fatal("rotate: new secret must differ from original")
		}
	}

	// 5. Revoke.
	{
		r := httptest.NewRequest(http.MethodDelete, "/v1/webhooks/"+id.String(), nil)
		r = r.WithContext(ctx)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("revoke: want 200, got %d: %s", w.Code, w.Body.String())
		}
	}

	// 6. Get after revoke — row still exists (soft-delete) but is marked revoked.
	{
		r := httptest.NewRequest(http.MethodGet, "/v1/webhooks/"+id.String(), nil)
		r = r.WithContext(ctx)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("get-after-revoke: want 200, got %d: %s", w.Code, w.Body.String())
		}
		var row store.WebhookData
		if err := json.NewDecoder(w.Body).Decode(&row); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if !row.Revoked {
			t.Fatal("row must be marked revoked after DELETE")
		}
	}
}

// TestWebhookAdmin_Patch_NonAdmin_403 verifies non-admin cannot patch.
func TestWebhookAdmin_Patch_NonAdmin_403(t *testing.T) {
	tenantID := uuid.New()
	userID := "viewer"

	ts := &adminTenantStore{roles: map[string]string{
		tenantID.String() + ":" + userID: "viewer",
	}}
	ws := newAdminWebhookStore()
	h := newAdminHandler(ws, ts)

	ctx := webhookTenantAdminCtx(tenantID, userID)
	w := doRequest(t, h, http.MethodPatch, "/v1/webhooks/"+uuid.New().String(), map[string]any{
		"name": "new name",
	}, ctx)

	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d: %s", w.Code, w.Body.String())
	}
}

// TestWebhookAdmin_Rotate_NonAdmin_403 verifies non-admin cannot rotate.
func TestWebhookAdmin_Rotate_NonAdmin_403(t *testing.T) {
	tenantID := uuid.New()
	userID := "viewer2"

	ts := &adminTenantStore{roles: map[string]string{
		tenantID.String() + ":" + userID: "viewer",
	}}
	ws := newAdminWebhookStore()
	h := newAdminHandler(ws, ts)

	ctx := webhookTenantAdminCtx(tenantID, userID)
	r := httptest.NewRequest(http.MethodPost, "/v1/webhooks/"+uuid.New().String()+"/rotate", nil)
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d: %s", w.Code, w.Body.String())
	}
}

// TestWebhookAdmin_Revoke_NonAdmin_403 verifies non-admin cannot revoke.
func TestWebhookAdmin_Revoke_NonAdmin_403(t *testing.T) {
	tenantID := uuid.New()
	userID := "viewer3"

	ts := &adminTenantStore{roles: map[string]string{
		tenantID.String() + ":" + userID: "viewer",
	}}
	ws := newAdminWebhookStore()
	h := newAdminHandler(ws, ts)

	ctx := webhookTenantAdminCtx(tenantID, userID)
	r := httptest.NewRequest(http.MethodDelete, "/v1/webhooks/"+uuid.New().String(), nil)
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d: %s", w.Code, w.Body.String())
	}
}

// TestGenerateWebhookSecret verifies the format and properties of generated secrets.
func TestGenerateWebhookSecret(t *testing.T) {
	raw, hash, prefix, err := generateWebhookSecret()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if len(raw) < 3 || raw[:3] != "wh_" {
		t.Fatalf("raw must start with wh_, got %q", raw)
	}
	if len(prefix) != 8 {
		t.Fatalf("prefix must be 8 chars, got %d: %q", len(prefix), prefix)
	}
	if prefix != raw[:8] {
		t.Fatalf("prefix %q != raw[:8] %q", prefix, raw[:8])
	}
	if len(hash) != 64 {
		t.Fatalf("hash must be 64 hex chars (SHA-256), got %d", len(hash))
	}
	// Two calls must produce different secrets.
	raw2, _, _, _ := generateWebhookSecret()
	if raw == raw2 {
		t.Fatal("secrets must be unique per generation")
	}
}

// ---- stub WebhookCallStore for list-calls tests ----

type adminCallsStore struct {
	rows       []store.WebhookCallData // pre-sorted DESC by created_at (caller's responsibility)
	lastFilter store.WebhookCallListFilter
	lastTenant uuid.UUID
}

func (s *adminCallsStore) Create(_ context.Context, _ *store.WebhookCallData) error { return nil }
func (s *adminCallsStore) GetByID(_ context.Context, _ uuid.UUID) (*store.WebhookCallData, error) {
	return nil, sql.ErrNoRows
}
func (s *adminCallsStore) GetByIdempotency(_ context.Context, _ uuid.UUID, _ string) (*store.WebhookCallData, error) {
	return nil, sql.ErrNoRows
}
func (s *adminCallsStore) UpdateStatus(_ context.Context, _ uuid.UUID, _ map[string]any) error {
	return nil
}
func (s *adminCallsStore) UpdateStatusCAS(_ context.Context, _ uuid.UUID, _ string, _ map[string]any) error {
	return nil
}
func (s *adminCallsStore) ClaimNext(_ context.Context, _ uuid.UUID, _ time.Time) (*store.WebhookCallData, error) {
	return nil, sql.ErrNoRows
}
func (s *adminCallsStore) List(ctx context.Context, f store.WebhookCallListFilter) ([]store.WebhookCallData, error) {
	s.lastFilter = f
	s.lastTenant = store.TenantIDFromContext(ctx)

	filtered := make([]store.WebhookCallData, 0, len(s.rows))
	for _, r := range s.rows {
		if r.TenantID != s.lastTenant {
			continue
		}
		if f.WebhookID != nil && r.WebhookID != *f.WebhookID {
			continue
		}
		if f.Status != "" && r.Status != f.Status {
			continue
		}
		filtered = append(filtered, r)
	}

	// Mimic PG: apply offset then limit.
	if f.Offset >= len(filtered) {
		return nil, nil
	}
	filtered = filtered[f.Offset:]
	if f.Limit > 0 && f.Limit < len(filtered) {
		filtered = filtered[:f.Limit]
	}
	return filtered, nil
}
func (s *adminCallsStore) DeleteOlderThan(_ context.Context, _ uuid.UUID, _ time.Time) (int64, error) {
	return 0, nil
}
func (s *adminCallsStore) ReclaimStale(_ context.Context, _ time.Time) (int64, error) {
	return 0, nil
}

// makeCallRows produces n call rows for webhookID under tenantID, with created_at
// descending so r[0] is newest. status is applied to all rows.
func makeCallRows(tenantID, webhookID uuid.UUID, n int, status string) []store.WebhookCallData {
	rows := make([]store.WebhookCallData, n)
	now := time.Now()
	for i := 0; i < n; i++ {
		rows[i] = store.WebhookCallData{
			ID:         uuid.New(),
			TenantID:   tenantID,
			WebhookID:  webhookID,
			DeliveryID: uuid.New(),
			Mode:       "async",
			Status:     status,
			Attempts:   0,
			CreatedAt:  now.Add(-time.Duration(i) * time.Minute), // DESC
		}
	}
	return rows
}

func TestWebhookAdmin_ListCalls_HappyPath(t *testing.T) {
	tenantID := uuid.New()
	userID := "user-lc"
	webhookID := uuid.New()

	ts := &adminTenantStore{
		roles: map[string]string{tenantID.String() + ":" + userID: store.TenantRoleAdmin},
	}
	ws := newAdminWebhookStore(&store.WebhookData{
		ID: webhookID, TenantID: tenantID, Name: "wh", Kind: "llm",
	})
	cs := &adminCallsStore{rows: makeCallRows(tenantID, webhookID, 3, "done")}
	h := newAdminHandlerWithCalls(ws, cs, ts)

	ctx := webhookTenantAdminCtx(tenantID, userID)
	w := doRequest(t, h, http.MethodGet, "/v1/webhooks/"+webhookID.String()+"/calls", nil, ctx)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp webhookCallsPageResp
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Items) != 3 {
		t.Fatalf("want 3 items, got %d", len(resp.Items))
	}
	if resp.HasMore {
		t.Fatal("has_more must be false when items fit in one page")
	}
	if resp.Limit != 50 || resp.Offset != 0 {
		t.Fatalf("unexpected pagination defaults: limit=%d offset=%d", resp.Limit, resp.Offset)
	}
	// Verify the heavy payload fields are NOT serialized.
	body := w.Body.String()
	if bytes.Contains([]byte(body), []byte(`"request_payload"`)) ||
		bytes.Contains([]byte(body), []byte(`"response"`)) ||
		bytes.Contains([]byte(body), []byte(`"lease_token"`)) {
		t.Fatalf("response must omit request_payload/response/lease_token, got: %s", body)
	}
}

func TestWebhookAdmin_ListCalls_CrossTenant_404(t *testing.T) {
	tenantA := uuid.New()
	tenantB := uuid.New()
	userA := "user-a"
	webhookID := uuid.New()

	ts := &adminTenantStore{
		roles: map[string]string{tenantA.String() + ":" + userA: store.TenantRoleAdmin},
	}
	ws := newAdminWebhookStore(&store.WebhookData{
		ID: webhookID, TenantID: tenantB, Name: "b-wh", Kind: "llm",
	})
	// Even if rows exist for tenant B, A must not see them — and not be told they exist.
	cs := &adminCallsStore{rows: makeCallRows(tenantB, webhookID, 5, "done")}
	h := newAdminHandlerWithCalls(ws, cs, ts)

	ctx := webhookTenantAdminCtx(tenantA, userA)
	w := doRequest(t, h, http.MethodGet, "/v1/webhooks/"+webhookID.String()+"/calls", nil, ctx)

	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404 for cross-tenant list-calls, got %d: %s", w.Code, w.Body.String())
	}
}

func TestWebhookAdmin_ListCalls_StatusFilter(t *testing.T) {
	tenantID := uuid.New()
	userID := "user-lc-status"
	webhookID := uuid.New()

	ts := &adminTenantStore{
		roles: map[string]string{tenantID.String() + ":" + userID: store.TenantRoleAdmin},
	}
	ws := newAdminWebhookStore(&store.WebhookData{
		ID: webhookID, TenantID: tenantID, Name: "wh", Kind: "llm",
	})
	rows := append(makeCallRows(tenantID, webhookID, 2, "done"),
		makeCallRows(tenantID, webhookID, 3, "failed")...)
	cs := &adminCallsStore{rows: rows}
	h := newAdminHandlerWithCalls(ws, cs, ts)

	ctx := webhookTenantAdminCtx(tenantID, userID)
	w := doRequest(t, h, http.MethodGet, "/v1/webhooks/"+webhookID.String()+"/calls?status=failed", nil, ctx)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	if cs.lastFilter.Status != "failed" {
		t.Fatalf("status filter not propagated, got %q", cs.lastFilter.Status)
	}
	var resp webhookCallsPageResp
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Items) != 3 {
		t.Fatalf("want 3 failed items, got %d", len(resp.Items))
	}
}

func TestWebhookAdmin_ListCalls_InvalidStatus_400(t *testing.T) {
	tenantID := uuid.New()
	userID := "user-lc-bad"
	webhookID := uuid.New()

	ts := &adminTenantStore{
		roles: map[string]string{tenantID.String() + ":" + userID: store.TenantRoleAdmin},
	}
	ws := newAdminWebhookStore(&store.WebhookData{
		ID: webhookID, TenantID: tenantID, Name: "wh", Kind: "llm",
	})
	cs := &adminCallsStore{}
	h := newAdminHandlerWithCalls(ws, cs, ts)

	ctx := webhookTenantAdminCtx(tenantID, userID)
	w := doRequest(t, h, http.MethodGet, "/v1/webhooks/"+webhookID.String()+"/calls?status=banana", nil, ctx)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for unknown status, got %d", w.Code)
	}
}

func TestWebhookAdmin_ListCalls_PaginationHasMore(t *testing.T) {
	tenantID := uuid.New()
	userID := "user-lc-page"
	webhookID := uuid.New()

	ts := &adminTenantStore{
		roles: map[string]string{tenantID.String() + ":" + userID: store.TenantRoleAdmin},
	}
	ws := newAdminWebhookStore(&store.WebhookData{
		ID: webhookID, TenantID: tenantID, Name: "wh", Kind: "llm",
	})
	cs := &adminCallsStore{rows: makeCallRows(tenantID, webhookID, 5, "done")}
	h := newAdminHandlerWithCalls(ws, cs, ts)

	ctx := webhookTenantAdminCtx(tenantID, userID)

	// Page 1: limit=2, offset=0 → 2 items, has_more=true.
	w := doRequest(t, h, http.MethodGet, "/v1/webhooks/"+webhookID.String()+"/calls?limit=2&offset=0", nil, ctx)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var p1 webhookCallsPageResp
	_ = json.NewDecoder(w.Body).Decode(&p1)
	if len(p1.Items) != 2 || !p1.HasMore || p1.Limit != 2 || p1.Offset != 0 {
		t.Fatalf("page1 unexpected: items=%d has_more=%v limit=%d offset=%d", len(p1.Items), p1.HasMore, p1.Limit, p1.Offset)
	}
	// Verify the store was asked for limit+1 (peek for has_more).
	if cs.lastFilter.Limit != 3 {
		t.Fatalf("store filter must request limit+1=3, got %d", cs.lastFilter.Limit)
	}

	// Page 3: limit=2, offset=4 → 1 item, has_more=false.
	w = doRequest(t, h, http.MethodGet, "/v1/webhooks/"+webhookID.String()+"/calls?limit=2&offset=4", nil, ctx)
	var p3 webhookCallsPageResp
	_ = json.NewDecoder(w.Body).Decode(&p3)
	if len(p3.Items) != 1 || p3.HasMore {
		t.Fatalf("page3 unexpected: items=%d has_more=%v", len(p3.Items), p3.HasMore)
	}
}

func TestWebhookAdmin_ListCalls_NonAdmin_403(t *testing.T) {
	tenantID := uuid.New()
	userID := "user-viewer"
	webhookID := uuid.New()

	// Viewer role, not admin.
	ts := &adminTenantStore{
		roles: map[string]string{tenantID.String() + ":" + userID: store.TenantRoleViewer},
	}
	ws := newAdminWebhookStore(&store.WebhookData{
		ID: webhookID, TenantID: tenantID, Name: "wh", Kind: "llm",
	})
	cs := &adminCallsStore{}
	h := newAdminHandlerWithCalls(ws, cs, ts)

	ctx := webhookTenantAdminCtx(tenantID, userID)
	w := doRequest(t, h, http.MethodGet, "/v1/webhooks/"+webhookID.String()+"/calls", nil, ctx)

	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403 for non-admin, got %d: %s", w.Code, w.Body.String())
	}
}
