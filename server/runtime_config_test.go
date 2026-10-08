package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func configLookup(values map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) { v, ok := values[k]; return v, ok }
}
func TestRuntimeConfigPublicFailClosed(t *testing.T) {
	for _, values := range []map[string]string{
		{"FREETOWER_ENV": "staging"},
		{"FREETOWER_ENV": "staging", "FREETOWER_PUBLIC_ORIGIN": "http://example.test", "DATABASE_URL": "synthetic"},
		{"FREETOWER_ENV": "staging", "FREETOWER_PUBLIC_ORIGIN": "https://example.test/", "DATABASE_URL": "synthetic"},
		{"FREETOWER_ENV": "unknown"},
		{"FREETOWER_REQUIRE_DATABASE": "true"},
		{"FREETOWER_REQUIRE_DATABASE": "perhaps"},
		{"FREETOWER_ENV": ""},
	} {
		if _, err := loadRuntimeConfig(configLookup(values)); err == nil {
			t.Fatalf("invalid config accepted: %#v", values)
		}
	}
}
func TestRuntimeConfigCanonicalWins(t *testing.T) {
	c, err := loadRuntimeConfig(configLookup(map[string]string{"FREETOWER_ENV": "staging", "FT_ENV": "development", "FREETOWER_PUBLIC_ORIGIN": "https://example.test", "FT_PUBLIC_ORIGIN": "http://localhost:4318", "DATABASE_URL": "synthetic", "FREETOWER_REQUIRE_DATABASE": "false", "FREETOWER_GIT_SHA": "381d7af5dee3e10b0033de31de16e9003a095da6"}))
	if err != nil || !c.Public() || !c.RequireDatabase || c.PublicOrigin != "https://example.test" || c.GitSHA != "381d7af" {
		t.Fatalf("unexpected config: %+v %v", c, err)
	}
	for _, sha := range []string{"", "secret-value", "1234567\n", "abcd"} {
		c, e := loadRuntimeConfig(configLookup(map[string]string{"FREETOWER_GIT_SHA": sha}))
		if e != nil || c.GitSHA != "" {
			t.Fatal("invalid commit leaked")
		}
	}
}
func TestPublicRuntimeOriginsHealthAndCookies(t *testing.T) {
	h := newIdentityHarness(t)
	h.app.runtime = RuntimeConfig{Environment: "staging", PublicOrigin: "https://example.test", RequireDatabase: true, GitSHA: "381d7af"}
	for _, origin := range []string{"", "https://wrong.test", "https://example.test/"} {
		for _, path := range []string{"/api/rooms", "/api/auth/wallet/challenge", "/api/rooms/example/ws"} {
			method := "POST"
			if path == "/api/rooms/example/ws" {
				method = "GET"
			}
			r := httptest.NewRequest(method, path, nil)
			r.Header.Set("Origin", origin)
			w := httptest.NewRecorder()
			h.app.routes().ServeHTTP(w, r)
			if w.Code != http.StatusForbidden {
				t.Fatalf("%s origin %q status %d", path, origin, w.Code)
			}
		}
	}
	w := httptest.NewRecorder()
	h.app.routes().ServeHTTP(w, httptest.NewRequest("GET", "/api/health", nil))
	if w.Code != 503 {
		t.Fatal("public health must fail without DB")
	}
	w = httptest.NewRecorder()
	h.app.identityCookie(w, bindingCookie, "synthetic", 600)
	c := w.Result().Cookies()[0]
	if !c.Secure || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode {
		t.Fatal("unsafe public cookie")
	}
}
func TestPublicIdentityDisablesLegacyFlags(t *testing.T) {
	t.Setenv("FREETOWER_ENV", "staging")
	t.Setenv("FREETOWER_PUBLIC_ORIGIN", "https://example.test")
	t.Setenv("DATABASE_URL", "synthetic")
	t.Setenv("FT_ENV", "development")
	t.Setenv("FT_ENABLE_DEV_IDENTITY", "true")
	t.Setenv("FT_ENABLE_LEGACY_AUTH", "true")
	t.Setenv("FT_ENABLE_OWNERSHIP_ACCEPTANCE", "true")
	t.Setenv("FT_ALLOWED_ORIGINS", "https://other.test")
	c := loadIdentityConfig()
	if c.Development || c.DevEnabled || c.LegacyEnabled || c.OwnershipAcceptanceEnabled || c.Origins["https://other.test"] {
		t.Fatalf("public flags unsafe: %+v", c)
	}
}

func TestRuntimeConfigAbsentEnvironmentDoesNotEnableDevelopmentAuth(t *testing.T) {
	t.Setenv("FT_ENV", "")
	t.Setenv("FT_ENABLE_DEV_IDENTITY", "true")
	t.Setenv("FT_ENABLE_LEGACY_AUTH", "true")
	t.Setenv("FT_ENABLE_OWNERSHIP_ACCEPTANCE", "true")
	c := loadIdentityConfig()
	if c.Development || c.DevEnabled || c.LegacyEnabled || c.OwnershipAcceptanceEnabled {
		t.Fatal("implicit development authority enabled")
	}
}
func TestRequiredDatabaseRejectsJSONApp(t *testing.T) {
	h := newIdentityHarness(t)
	if _, err := newAppWithConfig(h.app.store, t.TempDir(), RuntimeConfig{Environment: "staging", RequireDatabase: true}); err == nil {
		t.Fatal("public JSON fallback allowed")
	}
}
func TestPublicRuntimeHealthFieldsAndRateLimit(t *testing.T) {
	h := newIdentityHarness(t)
	h.app.runtime = RuntimeConfig{Environment: "staging", PublicOrigin: "https://example.test", GitSHA: "381d7af"}
	w := httptest.NewRecorder()
	h.app.routes().ServeHTTP(w, httptest.NewRequest("GET", "/api/health", nil))
	if w.Body.String() != "{\"commit\":\"381d7af\",\"database\":false,\"environment\":\"staging\",\"ok\":true}\n" {
		t.Fatalf("unexpected health fields: %s", w.Body.String())
	}
	if w.Header().Get("Content-Security-Policy") == "" || w.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatal("security headers absent")
	}
	for i := 0; i < 181; i++ {
		w = httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/api/identity/config", nil)
		h.app.routes().ServeHTTP(w, r)
	}
	if w.Code != 429 {
		t.Fatal("identity query unlimited")
	}
	w = httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/health", nil)
	h.app.routes().ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatal("health rate limited")
	}
}

func TestShutdownRejectsNewWebsocketsAndIsIdempotent(t *testing.T) {
	h := newIdentityHarness(t)
	h.app.shutdown()
	h.app.shutdown()
	w := httptest.NewRecorder()
	h.app.websocket(w, httptest.NewRequest("GET", "/api/rooms/example/ws", nil))
	if w.Code != 503 {
		t.Fatal("shutdown accepted new websocket")
	}
}

func TestPublicRuntimeRejectsInternalOrigins(t *testing.T) {
	for _, origin := range []string{"https://localhost", "https://127.0.0.1", "https://[::1]", "https://192.168.1.1", "https://service.railway.internal", "https://railway.internal"} {
		if _, err := loadRuntimeConfig(configLookup(map[string]string{"FREETOWER_ENV": "staging", "FREETOWER_PUBLIC_ORIGIN": origin, "DATABASE_URL": "synthetic"})); err == nil {
			t.Fatalf("internal origin accepted: %s", origin)
		}
	}
	if _, err := loadRuntimeConfig(configLookup(map[string]string{"FREETOWER_ENV": "staging", "FREETOWER_PUBLIC_ORIGIN": "https://freetower.up.railway.app", "DATABASE_URL": "synthetic"})); err != nil {
		t.Fatal(err)
	}
}
