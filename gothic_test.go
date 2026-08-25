package goth_fiber

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
	"github.com/markbates/goth"
	"github.com/markbates/goth/providers/faux"
)

// These tests share process-wide state: the SessionStore global and goth's
// provider registry. None of them call t.Parallel() for that reason.

// do issues a request and returns the status, body and any cookies set.
func do(t *testing.T, app *fiber.App, req *http.Request) (int, string, []*http.Cookie) {
	t.Helper()

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("%s: %v", req.URL, err)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("%s: reading body: %v", req.URL, err)
	}

	return resp.StatusCode, string(body), resp.Cookies()
}

// get is do() for a plain GET with optional cookies.
func get(t *testing.T, app *fiber.App, path string, cookies []*http.Cookie) (int, string, []*http.Cookie) {
	t.Helper()

	req := httptest.NewRequest("GET", path, nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}

	return do(t, app, req)
}

// useFauxProvider registers goth's test provider and restores the registry
// afterwards so tests don't leak into each other.
func useFauxProvider(t *testing.T) {
	t.Helper()

	goth.ClearProviders()
	goth.UseProviders(&faux.Provider{})
	t.Cleanup(goth.ClearProviders)
}

func Test_SetState(t *testing.T) {
	app := fiber.New()
	app.Get("/", func(ctx *fiber.Ctx) error {
		return ctx.SendString(SetState(ctx))
	})

	// An explicit state in the query is passed through untouched.
	_, body, _ := get(t, app, "/?state=test-state", nil)
	if body != "test-state" {
		t.Errorf("expected %q, got %q", "test-state", body)
	}

	// Without one, a state is generated.
	_, body, _ = get(t, app, "/", nil)
	if body == "" {
		t.Error("expected a generated state, got empty string")
	}
}

func Test_SetState_GeneratesUniqueUnguessableValues(t *testing.T) {
	app := fiber.New()
	app.Get("/", func(ctx *fiber.Ctx) error {
		return ctx.SendString(SetState(ctx))
	})

	seen := make(map[string]bool)
	for i := 0; i < 10; i++ {
		_, state, _ := get(t, app, "/", nil)

		if seen[state] {
			t.Fatalf("duplicate state generated: %q", state)
		}
		seen[state] = true

		// 64 random bytes, base64 encoded.
		if len(state) < 80 {
			t.Errorf("state is only %d chars, expected the full nonce", len(state))
		}
	}
}

func Test_GetState(t *testing.T) {
	app := fiber.New()
	app.Get("/", func(ctx *fiber.Ctx) error {
		return ctx.SendString(GetState(ctx))
	})

	_, body, _ := get(t, app, "/?state=callback-state", nil)
	if body != "callback-state" {
		t.Errorf("expected %q, got %q", "callback-state", body)
	}
}

func Test_GetProviderName(t *testing.T) {
	report := func(ctx *fiber.Ctx) error {
		name, err := GetProviderName(ctx)
		if err != nil {
			return ctx.Status(fiber.StatusBadRequest).SendString(err.Error())
		}
		return ctx.SendString(name)
	}

	app := fiber.New()
	app.Get("/param/:provider", report)
	app.Get("/", report)

	t.Run("query parameter", func(t *testing.T) {
		_, body, _ := get(t, app, "/?provider=google", nil)
		if body != "google" {
			t.Errorf("expected %q, got %q", "google", body)
		}
	})

	t.Run("url parameter", func(t *testing.T) {
		_, body, _ := get(t, app, "/param/github", nil)
		if body != "github" {
			t.Errorf("expected %q, got %q", "github", body)
		}
	})

	t.Run("request header", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("provider", "gitlab")

		_, body, _ := do(t, app, req)
		if body != "gitlab" {
			t.Errorf("expected %q, got %q", "gitlab", body)
		}
	})

	t.Run("nothing to go on", func(t *testing.T) {
		status, body, _ := get(t, app, "/", nil)
		if status != fiber.StatusBadRequest {
			t.Errorf("expected %d, got %d: %s", fiber.StatusBadRequest, status, body)
		}
		if !strings.Contains(body, "you must select a provider") {
			t.Errorf("unexpected error: %q", body)
		}
	})
}

// The fallback path: with no provider named anywhere, an existing session for
// a registered provider identifies it.
func Test_GetProviderName_FallsBackToAnExistingSession(t *testing.T) {
	useFauxProvider(t)

	app := fiber.New()
	app.Get("/begin", func(ctx *fiber.Ctx) error {
		return StoreInSession("faux", "stored-session-value", ctx)
	})
	app.Get("/whoami", func(ctx *fiber.Ctx) error {
		name, err := GetProviderName(ctx)
		if err != nil {
			return ctx.Status(fiber.StatusBadRequest).SendString(err.Error())
		}
		return ctx.SendString(name)
	})

	_, _, cookies := get(t, app, "/begin", nil)
	if len(cookies) == 0 {
		t.Fatal("expected a session cookie")
	}

	// Note there is no ?provider=, no :provider and no header here.
	_, body, _ := get(t, app, "/whoami", cookies)
	if body != "faux" {
		t.Errorf("expected the stored session to identify the provider, got %q", body)
	}
}

func Test_GetContextWithProvider(t *testing.T) {
	app := fiber.New()
	app.Get("/", func(ctx *fiber.Ctx) error {
		ctx = GetContextWithProvider(ctx, "twitter")

		name, err := GetProviderName(ctx)
		if err != nil {
			return ctx.Status(fiber.StatusBadRequest).SendString(err.Error())
		}
		return ctx.SendString(name)
	})

	status, body, _ := get(t, app, "/", nil)
	if status != fiber.StatusOK || body != "twitter" {
		t.Errorf("GetContextWithProvider did not make the provider visible to GetProviderName: status %d, body %q", status, body)
	}
}

func Test_BeginAuthHandler(t *testing.T) {
	useFauxProvider(t)

	app := fiber.New()
	app.Get("/auth/:provider", BeginAuthHandler)

	req := httptest.NewRequest("GET", "/auth/faux", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != fiber.StatusTemporaryRedirect {
		t.Fatalf("expected %d, got %d", fiber.StatusTemporaryRedirect, resp.StatusCode)
	}
	if resp.Header.Get("Location") == "" {
		t.Error("expected a Location header on the redirect")
	}
	if len(resp.Cookies()) == 0 {
		t.Error("expected the provider session to be stored in a cookie")
	}
}

func Test_BeginAuthHandler_UnknownProvider(t *testing.T) {
	goth.ClearProviders()
	t.Cleanup(goth.ClearProviders)

	app := fiber.New()
	app.Get("/auth/:provider", BeginAuthHandler)

	status, _, _ := get(t, app, "/auth/nope", nil)
	if status != fiber.StatusBadRequest {
		t.Errorf("expected %d, got %d", fiber.StatusBadRequest, status)
	}
}

func Test_BeginAuthHandler_EmptyProvider(t *testing.T) {
	useFauxProvider(t)

	app := fiber.New()
	app.Get("/auth/:provider", BeginAuthHandler)

	status, _, _ := get(t, app, "/auth/", nil)
	if status == fiber.StatusOK || status == fiber.StatusTemporaryRedirect {
		t.Errorf("expected an error for an empty provider, got %d", status)
	}
}

func Test_GetAuthURL(t *testing.T) {
	useFauxProvider(t)

	app := fiber.New()
	app.Get("/url/:provider", func(ctx *fiber.Ctx) error {
		u, err := GetAuthURL(ctx)
		if err != nil {
			return ctx.Status(fiber.StatusBadRequest).SendString(err.Error())
		}
		return ctx.SendString(u)
	})

	status, body, _ := get(t, app, "/url/faux", nil)
	if status != fiber.StatusOK {
		t.Fatalf("expected 200, got %d: %s", status, body)
	}

	parsed, err := url.Parse(body)
	if err != nil {
		t.Fatalf("auth URL is not a URL: %v", err)
	}
	if parsed.Query().Get("state") == "" {
		t.Error("expected the auth URL to carry a state parameter")
	}
}

// completeUserAuthApp wires the two halves of the flow against the faux
// provider and returns the app plus the callback route pattern.
func completeUserAuthApp(t *testing.T, opts ...CompleteUserAuthOptions) *fiber.App {
	t.Helper()

	app := fiber.New()
	app.Get("/auth/:provider", BeginAuthHandler)
	app.Get("/callback/:provider", func(ctx *fiber.Ctx) error {
		user, err := CompleteUserAuth(ctx, opts...)
		if err != nil {
			return ctx.Status(fiber.StatusBadRequest).SendString(err.Error())
		}
		return ctx.JSON(user)
	})

	return app
}

// beginAuth runs the first leg and returns the state the provider was given
// plus the session cookies.
func beginAuth(t *testing.T, app *fiber.App) (string, []*http.Cookie) {
	t.Helper()

	req := httptest.NewRequest("GET", "/auth/faux", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusTemporaryRedirect {
		t.Fatalf("begin auth: expected a redirect, got %d", resp.StatusCode)
	}

	location, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		t.Fatalf("begin auth: Location is not a URL: %v", err)
	}

	state := location.Query().Get("state")
	if state == "" {
		t.Fatal("begin auth: no state on the redirect URL")
	}

	cookies := resp.Cookies()
	if len(cookies) == 0 {
		t.Fatal("begin auth: no session cookie")
	}

	return state, cookies
}

func Test_CompleteUserAuth(t *testing.T) {
	useFauxProvider(t)

	app := completeUserAuthApp(t)
	state, cookies := beginAuth(t, app)

	status, body, _ := get(t, app, "/callback/faux?code=test-code&state="+url.QueryEscape(state), cookies)
	if status != fiber.StatusOK {
		t.Fatalf("expected 200, got %d: %s", status, body)
	}

	// faux sets an access token in Authorize, which is what lets the second
	// FetchUser succeed.
	if !strings.Contains(body, `"Provider":"faux"`) {
		t.Errorf("expected the faux user, got %s", body)
	}
	if !strings.Contains(body, `"AccessToken":"access"`) {
		t.Errorf("expected the token from Authorize, got %s", body)
	}
}

func Test_CompleteUserAuth_RejectsAMismatchedState(t *testing.T) {
	useFauxProvider(t)

	app := completeUserAuthApp(t)
	_, cookies := beginAuth(t, app)

	// The state the provider handed back is not the one we stored.
	status, body, _ := get(t, app, "/callback/faux?code=test-code&state=not-the-original", cookies)
	if status != fiber.StatusBadRequest {
		t.Fatalf("expected the callback to be rejected, got %d: %s", status, body)
	}
	if !strings.Contains(body, "state token mismatch") {
		t.Errorf("expected a state mismatch error, got %q", body)
	}
}

func Test_CompleteUserAuth_WithoutASession(t *testing.T) {
	useFauxProvider(t)

	app := completeUserAuthApp(t)

	// No cookies, so nothing was ever stored for this provider.
	status, body, _ := get(t, app, "/callback/faux?code=test-code&state=anything", nil)
	if status != fiber.StatusBadRequest {
		t.Fatalf("expected the callback to be rejected, got %d: %s", status, body)
	}
	if !strings.Contains(body, "could not find a matching session for this request") {
		t.Errorf("unexpected error: %q", body)
	}
}

// ShouldLogout defaults to true, which destroys the session. Opting out leaves
// it readable, which is what issue #45 was about.
func Test_CompleteUserAuth_ShouldLogout(t *testing.T) {
	for _, tc := range []struct {
		name            string
		opts            []CompleteUserAuthOptions
		sessionSurvives bool
	}{
		{name: "default destroys the session", opts: nil, sessionSurvives: false},
		{name: "ShouldLogout false keeps it", opts: []CompleteUserAuthOptions{{ShouldLogout: false}}, sessionSurvives: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			useFauxProvider(t)

			app := completeUserAuthApp(t, tc.opts...)
			app.Get("/peek", func(ctx *fiber.Ctx) error {
				if _, err := GetFromSession("faux", ctx); err != nil {
					return ctx.Status(fiber.StatusNotFound).SendString(err.Error())
				}
				return ctx.SendString("present")
			})

			state, cookies := beginAuth(t, app)

			status, body, after := get(t, app, "/callback/faux?code=c&state="+url.QueryEscape(state), cookies)
			if status != fiber.StatusOK {
				t.Fatalf("callback failed: %d: %s", status, body)
			}

			if len(after) > 0 {
				cookies = after
			}

			status, body, _ = get(t, app, "/peek", cookies)
			if tc.sessionSurvives && status != fiber.StatusOK {
				t.Errorf("expected the session to survive, got %d: %s", status, body)
			}
			if !tc.sessionSurvives && status == fiber.StatusOK {
				t.Error("expected the session to be destroyed, but it was still readable")
			}
		})
	}
}

func Test_Logout(t *testing.T) {
	app := fiber.New()
	app.Get("/store", func(ctx *fiber.Ctx) error {
		return StoreInSession("user", "john", ctx)
	})
	app.Get("/logout", func(ctx *fiber.Ctx) error {
		if err := Logout(ctx); err != nil {
			return ctx.Status(fiber.StatusInternalServerError).SendString(err.Error())
		}
		return ctx.SendString("logged out")
	})
	app.Get("/get", func(ctx *fiber.Ctx) error {
		v, err := GetFromSession("user", ctx)
		if err != nil {
			return ctx.Status(fiber.StatusNotFound).SendString(err.Error())
		}
		return ctx.SendString(v)
	})

	_, _, cookies := get(t, app, "/store", nil)

	status, body, afterLogout := get(t, app, "/logout", cookies)
	if status != fiber.StatusOK {
		t.Fatalf("logout failed: %d: %s", status, body)
	}
	if len(afterLogout) > 0 {
		cookies = afterLogout
	}

	status, body, _ = get(t, app, "/get", cookies)
	if status != fiber.StatusNotFound {
		t.Errorf("expected the value to be gone after logout, got %d: %s", status, body)
	}
}

func Test_SessionStoreIsInitialised(t *testing.T) {
	if SessionStore == nil {
		t.Error("SessionStore should be set up in init()")
	}
}

func Test_CustomSessionStore(t *testing.T) {
	original := SessionStore
	t.Cleanup(func() { SessionStore = original })

	SessionStore = session.New(session.Config{
		KeyLookup:      "cookie:custom_session",
		CookieHTTPOnly: true,
	})

	app := fiber.New()
	app.Get("/store", func(ctx *fiber.Ctx) error {
		return StoreInSession("k", "v", ctx)
	})
	app.Get("/get", func(ctx *fiber.Ctx) error {
		v, err := GetFromSession("k", ctx)
		if err != nil {
			return ctx.Status(fiber.StatusNotFound).SendString(err.Error())
		}
		return ctx.SendString(v)
	})

	status, body, cookies := get(t, app, "/store", nil)
	if status != fiber.StatusOK {
		t.Fatalf("store failed: %d: %s", status, body)
	}

	var found bool
	for _, c := range cookies {
		if c.Name == "custom_session" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected the custom cookie name to be used, got %v", cookies)
	}

	status, body, _ = get(t, app, "/get", cookies)
	if status != fiber.StatusOK || body != "v" {
		t.Errorf("round trip through the custom store failed: %d: %s", status, body)
	}
}

func Test_StoreAndGetFromSession(t *testing.T) {
	app := fiber.New()
	app.Get("/set", func(ctx *fiber.Ctx) error {
		if err := StoreInSession("faux", "hello", ctx); err != nil {
			return ctx.Status(fiber.StatusInternalServerError).SendString(err.Error())
		}
		return ctx.SendString("ok")
	})
	app.Get("/get", func(ctx *fiber.Ctx) error {
		v, err := GetFromSession("faux", ctx)
		if err != nil {
			return ctx.Status(fiber.StatusNotFound).SendString(err.Error())
		}
		return ctx.SendString(v)
	})

	status, body, cookies := get(t, app, "/set", nil)
	if status != fiber.StatusOK {
		t.Fatalf("set: expected 200, got %d: %s", status, body)
	}
	if len(cookies) == 0 {
		t.Fatal("set: expected a session cookie")
	}

	status, body, _ = get(t, app, "/get", cookies)
	if status != fiber.StatusOK {
		t.Fatalf("get: expected 200, got %d: %s", status, body)
	}
	if body != "hello" {
		t.Fatalf("get: expected %q, got %q", "hello", body)
	}
}

func Test_MultipleSessionValues(t *testing.T) {
	app := fiber.New()
	app.Get("/store", func(ctx *fiber.Ctx) error {
		for k, v := range map[string]string{"key1": "value1", "key2": "value2", "key3": "value3"} {
			if err := StoreInSession(k, v, ctx); err != nil {
				return ctx.Status(fiber.StatusInternalServerError).SendString(err.Error())
			}
		}
		return ctx.SendString("stored")
	})
	app.Get("/get", func(ctx *fiber.Ctx) error {
		var out []string
		for _, k := range []string{"key1", "key2", "key3"} {
			v, err := GetFromSession(k, ctx)
			if err != nil {
				return ctx.Status(fiber.StatusNotFound).SendString(k + ": " + err.Error())
			}
			out = append(out, v)
		}
		return ctx.SendString(strings.Join(out, ","))
	})

	_, _, cookies := get(t, app, "/store", nil)

	status, body, _ := get(t, app, "/get", cookies)
	if status != fiber.StatusOK {
		t.Fatalf("expected 200, got %d: %s", status, body)
	}
	if body != "value1,value2,value3" {
		t.Errorf("expected %q, got %q", "value1,value2,value3", body)
	}
}

// Real provider sessions (Apple, Microsoft) carry sizeable tokens, which is
// what the gzip round trip is for.
func Test_LargeSessionData(t *testing.T) {
	large := strings.Repeat("abcdefghijklmnopqrstuvwxyz", 200)

	app := fiber.New()
	app.Get("/store", func(ctx *fiber.Ctx) error {
		return StoreInSession("token", large, ctx)
	})
	app.Get("/get", func(ctx *fiber.Ctx) error {
		v, err := GetFromSession("token", ctx)
		if err != nil {
			return ctx.Status(fiber.StatusNotFound).SendString(err.Error())
		}
		return ctx.SendString(v)
	})

	status, body, cookies := get(t, app, "/store", nil)
	if status != fiber.StatusOK {
		t.Fatalf("store failed: %d: %s", status, body)
	}

	status, body, _ = get(t, app, "/get", cookies)
	if status != fiber.StatusOK {
		t.Fatalf("get failed: %d: %s", status, body)
	}
	if body != large {
		t.Errorf("large value corrupted: got %d bytes, want %d", len(body), len(large))
	}
}

func Test_SpecialCharactersInSessionData(t *testing.T) {
	special := `{"token":"eyJhbGciOiJSUzI1NiIs...","refresh":"abc123==","user":{"name":"José García","email":"test@例え.jp"}}`

	app := fiber.New()
	app.Get("/store", func(ctx *fiber.Ctx) error {
		return StoreInSession("special", special, ctx)
	})
	app.Get("/get", func(ctx *fiber.Ctx) error {
		v, err := GetFromSession("special", ctx)
		if err != nil {
			return ctx.Status(fiber.StatusNotFound).SendString(err.Error())
		}
		return ctx.SendString(v)
	})

	_, _, cookies := get(t, app, "/store", nil)

	status, body, _ := get(t, app, "/get", cookies)
	if status != fiber.StatusOK {
		t.Fatalf("get failed: %d: %s", status, body)
	}
	if body != special {
		t.Errorf("special characters corrupted:\n got: %s\nwant: %s", body, special)
	}
}

// The "not found" error used to swallow every underlying failure. It should
// still carry the historical prefix, but now also say which key was missing.
func Test_GetFromSession_MissingKeyNamesTheKey(t *testing.T) {
	app := fiber.New()
	app.Get("/get", func(ctx *fiber.Ctx) error {
		_, err := GetFromSession("faux", ctx)
		if err != nil {
			return ctx.Status(fiber.StatusNotFound).SendString(err.Error())
		}
		return ctx.SendString("found")
	})

	status, body, _ := get(t, app, "/get", nil)
	if status != fiber.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", status, body)
	}
	if !strings.HasPrefix(body, "could not find a matching session for this request") {
		t.Fatalf("lost the historical error prefix: %q", body)
	}
	if !strings.Contains(body, `"faux"`) {
		t.Fatalf("error does not name the missing key: %q", body)
	}
}

// A non-string value under the provider key used to panic on an unchecked type
// assertion. It must come back as an error naming the actual type instead.
func Test_GetFromSession_NonStringValue(t *testing.T) {
	app := fiber.New()
	app.Get("/set-bad", func(ctx *fiber.Ctx) error {
		sess, err := SessionStore.Get(ctx)
		if err != nil {
			return ctx.Status(fiber.StatusInternalServerError).SendString(err.Error())
		}
		sess.Set("faux", 42)
		if err := sess.Save(); err != nil {
			return ctx.Status(fiber.StatusInternalServerError).SendString(err.Error())
		}
		return ctx.SendString("ok")
	})
	app.Get("/get", func(ctx *fiber.Ctx) error {
		_, err := GetFromSession("faux", ctx)
		if err != nil {
			return ctx.Status(fiber.StatusNotFound).SendString(err.Error())
		}
		return ctx.SendString("found")
	})

	status, body, cookies := get(t, app, "/set-bad", nil)
	if status != fiber.StatusOK {
		t.Fatalf("set-bad: expected 200, got %d: %s", status, body)
	}

	status, body, _ = get(t, app, "/get", cookies)
	if status != fiber.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", status, body)
	}
	if !strings.Contains(body, "not a string") {
		t.Fatalf("expected a type mismatch error, got %q", body)
	}
}
