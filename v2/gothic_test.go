package goth_fiber

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/session"
	"github.com/markbates/goth"
	"github.com/markbates/goth/providers/faux"
)

func Test_SetState(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/", func(c fiber.Ctx) error {
		state := SetState(c)
		return c.SendString(state)
	})

	// Test with state in query
	req := httptest.NewRequest("GET", "/?state=test-state", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "test-state" {
		t.Errorf("expected state to be 'test-state', got '%s'", string(body))
	}

	// Test without state - should generate random state
	req = httptest.NewRequest("GET", "/", nil)
	resp, err = app.Test(req)
	if err != nil {
		t.Fatal(err)
	}

	body, _ = io.ReadAll(resp.Body)
	if len(body) == 0 {
		t.Error("expected generated state, got empty string")
	}
}

func Test_GetState(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/", func(c fiber.Ctx) error {
		state := GetState(c)
		return c.SendString(state)
	})

	req := httptest.NewRequest("GET", "/?state=callback-state", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "callback-state" {
		t.Errorf("expected state to be 'callback-state', got '%s'", string(body))
	}
}

func Test_GetProviderName(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	// Test with query parameter
	app.Get("/query", func(c fiber.Ctx) error {
		name, err := GetProviderName(c)
		if err != nil {
			return c.Status(400).SendString(err.Error())
		}
		return c.SendString(name)
	})

	req := httptest.NewRequest("GET", "/query?provider=google", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "google" {
		t.Errorf("expected provider to be 'google', got '%s'", string(body))
	}

	// Test with URL param
	app.Get("/param/:provider", func(c fiber.Ctx) error {
		name, err := GetProviderName(c)
		if err != nil {
			return c.Status(400).SendString(err.Error())
		}
		return c.SendString(name)
	})

	req = httptest.NewRequest("GET", "/param/github", nil)
	resp, err = app.Test(req)
	if err != nil {
		t.Fatal(err)
	}

	body, _ = io.ReadAll(resp.Body)
	if string(body) != "github" {
		t.Errorf("expected provider to be 'github', got '%s'", string(body))
	}

	// Test with no provider - should error
	req = httptest.NewRequest("GET", "/query", nil)
	resp, err = app.Test(req)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != 400 {
		t.Errorf("expected status 400, got %d", resp.StatusCode)
	}
}

func Test_GetContextWithProvider(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/", func(c fiber.Ctx) error {
		// GetContextWithProvider sets the provider in Locals
		c = GetContextWithProvider(c, "twitter")
		// GetProviderName should now be able to find it
		name, err := GetProviderName(c)
		if err != nil {
			return c.Status(400).SendString(err.Error())
		}
		return c.SendString(name)
	})

	req := httptest.NewRequest("GET", "/", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "twitter" {
		t.Errorf("expected provider to be 'twitter', got '%s'", string(body))
	}
}

func Test_BeginAuthHandler(t *testing.T) {
	// Setup faux provider
	goth.ClearProviders()
	goth.UseProviders(&faux.Provider{})

	app := fiber.New()
	app.Get("/auth/:provider", BeginAuthHandler)

	req := httptest.NewRequest("GET", "/auth/faux", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}

	// Should redirect to auth URL
	if resp.StatusCode != fiber.StatusTemporaryRedirect {
		t.Errorf("expected status %d, got %d", fiber.StatusTemporaryRedirect, resp.StatusCode)
	}

	location := resp.Header.Get("Location")
	if location == "" {
		t.Error("expected redirect location header")
	}
}

func Test_BeginAuthHandler_InvalidProvider(t *testing.T) {
	goth.ClearProviders()

	app := fiber.New()
	app.Get("/auth/:provider", BeginAuthHandler)

	req := httptest.NewRequest("GET", "/auth/invalid", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("expected status %d, got %d", fiber.StatusBadRequest, resp.StatusCode)
	}
}

func Test_Logout(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/logout", func(c fiber.Ctx) error {
		if err := Logout(c); err != nil {
			return c.Status(500).SendString(err.Error())
		}
		return c.SendString("logged out")
	})

	req := httptest.NewRequest("GET", "/logout", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func Test_StoreAndGetFromSession(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	// Store in session
	app.Get("/store", func(c fiber.Ctx) error {
		err := StoreInSession("test-key", "test-value", c)
		if err != nil {
			return c.Status(500).SendString(err.Error())
		}
		return c.SendString("stored")
	})

	// Get from session
	app.Get("/get", func(c fiber.Ctx) error {
		value, err := GetFromSession("test-key", c)
		if err != nil {
			return c.Status(404).SendString(err.Error())
		}
		return c.SendString(value)
	})

	// First store a value
	req := httptest.NewRequest("GET", "/store", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("expected status 200 on store, got %d", resp.StatusCode)
	}

	// Get the session cookie
	cookies := resp.Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected session cookie to be set")
	}

	// Now retrieve the value with the same session
	req = httptest.NewRequest("GET", "/get", nil)
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}

	resp, err = app.Test(req)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("expected status 200 on get, got %d: %s", resp.StatusCode, string(body))
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "test-value" {
		t.Errorf("expected value to be 'test-value', got '%s'", string(body))
	}
}

func Test_GetFromSession_NotFound(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/get", func(c fiber.Ctx) error {
		_, err := GetFromSession("nonexistent", c)
		if err != nil {
			return c.Status(404).SendString(err.Error())
		}
		return c.SendString("found")
	})

	req := httptest.NewRequest("GET", "/get", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != 404 {
		t.Errorf("expected status 404, got %d", resp.StatusCode)
	}
}

func Test_GetAuthURL(t *testing.T) {
	// Setup faux provider
	goth.ClearProviders()
	goth.UseProviders(&faux.Provider{})

	app := fiber.New()
	app.Get("/url/:provider", func(c fiber.Ctx) error {
		url, err := GetAuthURL(c)
		if err != nil {
			return c.Status(400).SendString(err.Error())
		}
		return c.SendString(url)
	})

	req := httptest.NewRequest("GET", "/url/faux", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("expected status 200, got %d: %s", resp.StatusCode, string(body))
	}

	body, _ := io.ReadAll(resp.Body)
	if len(body) == 0 {
		t.Error("expected auth URL, got empty string")
	}
}

func Test_SessionManagerNotNil(t *testing.T) {
	if SessionManager == nil {
		t.Error("SessionManager should be initialized in init()")
	}
}

// useFauxProvider registers goth's test provider and clears the registry
// afterwards so tests don't leak into each other.
func useFauxProvider(t *testing.T) {
	t.Helper()

	goth.ClearProviders()
	goth.UseProviders(&faux.Provider{})
	t.Cleanup(goth.ClearProviders)
}

// completeUserAuthApp wires both halves of the flow against the faux provider.
func completeUserAuthApp(opts ...CompleteUserAuthOptions) *fiber.App {
	app := fiber.New()
	app.Get("/auth/:provider", BeginAuthHandler)
	app.Get("/callback/:provider", func(c fiber.Ctx) error {
		user, err := CompleteUserAuth(c, opts...)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).SendString(err.Error())
		}
		return c.JSON(user)
	})

	return app
}

// beginAuth runs the first leg and returns the state the provider was handed
// plus the session cookies, so the callback can present the real one.
func beginAuth(t *testing.T, app *fiber.App) (string, []*http.Cookie) {
	t.Helper()

	resp, err := app.Test(httptest.NewRequest("GET", "/auth/faux", nil))
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

// callback replays the provider's redirect back to us.
func callback(t *testing.T, app *fiber.App, query string, cookies []*http.Cookie) (int, string, []*http.Cookie) {
	t.Helper()

	req := httptest.NewRequest("GET", "/callback/faux?"+query, nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}

	return resp.StatusCode, string(body), resp.Cookies()
}

func Test_CompleteUserAuth(t *testing.T) {
	useFauxProvider(t)

	app := completeUserAuthApp()
	state, cookies := beginAuth(t, app)

	status, body, _ := callback(t, app, "code=test-code&state="+url.QueryEscape(state), cookies)
	if status != fiber.StatusOK {
		t.Fatalf("expected 200, got %d: %s", status, body)
	}

	// faux's Authorize sets the access token, which is what lets the second
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

	app := completeUserAuthApp()
	_, cookies := beginAuth(t, app)

	// Not the state we stored.
	status, body, _ := callback(t, app, "code=test-code&state=not-the-original", cookies)
	if status != fiber.StatusBadRequest {
		t.Fatalf("expected the callback to be rejected, got %d: %s", status, body)
	}
	if !strings.Contains(body, "state token mismatch") {
		t.Errorf("expected a state mismatch error, got %q", body)
	}
}

func Test_CompleteUserAuth_WithoutASession(t *testing.T) {
	useFauxProvider(t)

	app := completeUserAuthApp()

	// No cookies, so nothing was ever stored for this provider.
	status, body, _ := callback(t, app, "code=test-code&state=anything", nil)
	if status != fiber.StatusBadRequest {
		t.Fatalf("expected the callback to be rejected, got %d: %s", status, body)
	}
	if !strings.Contains(body, "could not find a matching session for this request") {
		t.Errorf("unexpected error: %q", body)
	}
}

// ShouldLogout defaults to true, which destroys the session. Opting out leaves
// it readable. That difference is what issue #45 was about.
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

			app := completeUserAuthApp(tc.opts...)
			app.Get("/peek", func(c fiber.Ctx) error {
				if _, err := GetFromSession("faux", c); err != nil {
					return c.Status(fiber.StatusNotFound).SendString(err.Error())
				}
				return c.SendString("present")
			})

			state, cookies := beginAuth(t, app)

			status, body, after := callback(t, app, "code=c&state="+url.QueryEscape(state), cookies)
			if status != fiber.StatusOK {
				t.Fatalf("callback failed: %d: %s", status, body)
			}
			if len(after) > 0 {
				cookies = after
			}

			req := httptest.NewRequest("GET", "/peek", nil)
			for _, c := range cookies {
				req.AddCookie(c)
			}
			resp, err := app.Test(req)
			if err != nil {
				t.Fatal(err)
			}

			if tc.sessionSurvives && resp.StatusCode != fiber.StatusOK {
				t.Errorf("expected the session to survive, got %d", resp.StatusCode)
			}
			if !tc.sessionSurvives && resp.StatusCode == fiber.StatusOK {
				t.Error("expected the session to be destroyed, but it was still readable")
			}
		})
	}
}

func Test_LargeSessionData(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	// Simulate large OAuth token data (like Apple/Microsoft tokens)
	largeData := make([]byte, 4096)
	for i := range largeData {
		largeData[i] = byte('A' + (i % 26))
	}
	largeString := string(largeData)

	app.Get("/store-large", func(c fiber.Ctx) error {
		err := StoreInSession("large-token", largeString, c)
		if err != nil {
			return c.Status(500).SendString(err.Error())
		}
		return c.SendString("stored")
	})

	app.Get("/get-large", func(c fiber.Ctx) error {
		value, err := GetFromSession("large-token", c)
		if err != nil {
			return c.Status(404).SendString(err.Error())
		}
		return c.SendString(value)
	})

	// Store large data
	req := httptest.NewRequest("GET", "/store-large", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("failed to store large data: %s", string(body))
	}

	cookies := resp.Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected session cookie")
	}

	// Retrieve large data
	req = httptest.NewRequest("GET", "/get-large", nil)
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}

	resp, err = app.Test(req)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("failed to get large data: %s", string(body))
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != largeString {
		t.Errorf("large data mismatch: got %d bytes, expected %d bytes", len(body), len(largeString))
	}
}

func Test_MultipleSessionValues(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	app.Get("/store-multiple", func(c fiber.Ctx) error {
		if err := StoreInSession("key1", "value1", c); err != nil {
			return c.Status(500).SendString(err.Error())
		}
		if err := StoreInSession("key2", "value2", c); err != nil {
			return c.Status(500).SendString(err.Error())
		}
		if err := StoreInSession("key3", "value3", c); err != nil {
			return c.Status(500).SendString(err.Error())
		}
		return c.SendString("stored")
	})

	app.Get("/get-multiple", func(c fiber.Ctx) error {
		v1, err := GetFromSession("key1", c)
		if err != nil {
			return c.Status(404).SendString("key1: " + err.Error())
		}
		v2, err := GetFromSession("key2", c)
		if err != nil {
			return c.Status(404).SendString("key2: " + err.Error())
		}
		v3, err := GetFromSession("key3", c)
		if err != nil {
			return c.Status(404).SendString("key3: " + err.Error())
		}
		return c.SendString(v1 + "," + v2 + "," + v3)
	})

	// Store multiple values
	req := httptest.NewRequest("GET", "/store-multiple", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}

	cookies := resp.Cookies()

	// Retrieve all values
	req = httptest.NewRequest("GET", "/get-multiple", nil)
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}

	resp, err = app.Test(req)
	if err != nil {
		t.Fatal(err)
	}

	body, _ := io.ReadAll(resp.Body)
	expected := "value1,value2,value3"
	if string(body) != expected {
		t.Errorf("expected '%s', got '%s'", expected, string(body))
	}
}

func Test_CustomSessionStore(t *testing.T) {
	// Save original manager
	originalManager := SessionManager
	defer func() {
		SessionManager = originalManager
	}()

	// Create custom store with different config
	SessionManager = NewSessionManager(session.NewStore(session.Config{
		CookieHTTPOnly: true,
		CookieSecure:   true,
	}))

	app := fiber.New()
	app.Get("/test", func(c fiber.Ctx) error {
		err := StoreInSession("custom-key", "custom-value", c)
		if err != nil {
			return c.Status(500).SendString(err.Error())
		}
		return c.SendString("ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	// Verify cookie was set
	cookies := resp.Cookies()
	if len(cookies) == 0 {
		t.Error("expected session cookie from custom store")
	}
}

func Test_SessionAfterLogout(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	app.Get("/store", func(c fiber.Ctx) error {
		return StoreInSession("user", "john", c)
	})

	app.Get("/logout", func(c fiber.Ctx) error {
		return Logout(c)
	})

	app.Get("/get", func(c fiber.Ctx) error {
		value, err := GetFromSession("user", c)
		if err != nil {
			return c.Status(404).SendString(err.Error())
		}
		return c.SendString(value)
	})

	// Store a value
	req := httptest.NewRequest("GET", "/store", nil)
	resp, _ := app.Test(req)
	cookies := resp.Cookies()

	// Logout
	req = httptest.NewRequest("GET", "/logout", nil)
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	resp, _ = app.Test(req)

	// Get new cookies after logout (session should be destroyed)
	newCookies := resp.Cookies()

	// Try to get value - should fail
	req = httptest.NewRequest("GET", "/get", nil)
	for _, cookie := range newCookies {
		req.AddCookie(cookie)
	}

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}

	// Should not find the value after logout
	if resp.StatusCode != 404 {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 404 after logout, got %d: %s", resp.StatusCode, string(body))
	}
}

func Test_StateGenerationUniqueness(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/", func(c fiber.Ctx) error {
		state := SetState(c)
		return c.SendString(state)
	})

	// Generate multiple states and ensure they're unique
	states := make(map[string]bool)
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}

		body, _ := io.ReadAll(resp.Body)
		state := string(body)

		if states[state] {
			t.Errorf("duplicate state generated: %s", state)
		}
		states[state] = true

		// State should be base64 encoded (86 chars for 64 bytes)
		if len(state) < 80 {
			t.Errorf("state seems too short: %d chars", len(state))
		}
	}
}

func Test_SpecialCharactersInSessionData(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	// Test with special characters that might break gzip or encoding
	specialData := `{"token":"eyJhbGciOiJSUzI1NiIs...","refresh":"abc123==","user":{"name":"José García","email":"test@例え.jp"}}`

	app.Get("/store", func(c fiber.Ctx) error {
		return StoreInSession("special", specialData, c)
	})

	app.Get("/get", func(c fiber.Ctx) error {
		value, err := GetFromSession("special", c)
		if err != nil {
			return c.Status(404).SendString(err.Error())
		}
		return c.SendString(value)
	})

	req := httptest.NewRequest("GET", "/store", nil)
	resp, _ := app.Test(req)
	cookies := resp.Cookies()

	req = httptest.NewRequest("GET", "/get", nil)
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != specialData {
		t.Errorf("special characters corrupted:\ngot: %s\nwant: %s", string(body), specialData)
	}
}

func Test_EmptyProviderName(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/auth/:provider", BeginAuthHandler)

	// Empty provider in URL param
	req := httptest.NewRequest("GET", "/auth/", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}

	// Should get 404 (route doesn't match) or 400 (bad request)
	if resp.StatusCode == 200 || resp.StatusCode == 307 {
		t.Errorf("expected error for empty provider, got %d", resp.StatusCode)
	}
}
