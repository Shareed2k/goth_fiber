package goth_fiber

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

// newTestApp wires up the three routes the session round-trip tests need.
func newTestApp(t *testing.T) *fiber.App {
	t.Helper()

	app := fiber.New()

	app.Get("/set", func(ctx *fiber.Ctx) error {
		if err := StoreInSession("faux", "hello", ctx); err != nil {
			return ctx.Status(fiber.StatusInternalServerError).SendString(err.Error())
		}
		return ctx.SendString("ok")
	})

	// Stores a non-string under the provider key, which is what happens when an
	// application shares SessionStore with its own session data.
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
		v, err := GetFromSession("faux", ctx)
		if err != nil {
			return ctx.Status(fiber.StatusNotFound).SendString(err.Error())
		}
		return ctx.SendString(v)
	})

	return app
}

func do(t *testing.T, app *fiber.App, path string, cookies []*http.Cookie) (int, string, []*http.Cookie) {
	t.Helper()

	req := httptest.NewRequest("GET", path, nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("%s: reading body: %v", path, err)
	}

	return resp.StatusCode, string(body), resp.Cookies()
}

func Test_StoreAndGetFromSession(t *testing.T) {
	app := newTestApp(t)

	status, body, cookies := do(t, app, "/set", nil)
	if status != fiber.StatusOK {
		t.Fatalf("set: expected 200, got %d: %s", status, body)
	}
	if len(cookies) == 0 {
		t.Fatal("set: expected a session cookie")
	}

	status, body, _ = do(t, app, "/get", cookies)
	if status != fiber.StatusOK {
		t.Fatalf("get: expected 200, got %d: %s", status, body)
	}
	if body != "hello" {
		t.Fatalf("get: expected %q, got %q", "hello", body)
	}
}

// The "not found" error used to swallow every underlying failure. It should
// still carry the historical prefix, but now also say which key was missing.
func Test_GetFromSession_MissingKeyNamesTheKey(t *testing.T) {
	app := newTestApp(t)

	status, body, _ := do(t, app, "/get", nil)
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
	app := newTestApp(t)

	status, body, cookies := do(t, app, "/set-bad", nil)
	if status != fiber.StatusOK {
		t.Fatalf("set-bad: expected 200, got %d: %s", status, body)
	}

	status, body, _ = do(t, app, "/get", cookies)
	if status != fiber.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", status, body)
	}
	if !strings.Contains(body, "not a string") {
		t.Fatalf("expected a type mismatch error, got %q", body)
	}
}
