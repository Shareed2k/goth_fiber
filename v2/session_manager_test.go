package goth_fiber

import (
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func Test_SessionManager_SetGetDel(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	app.Get("/set", func(c fiber.Ctx) error {
		if err := SessionManager.setValue(c, "sm-key", "sm-value"); err != nil {
			return c.Status(500).SendString(err.Error())
		}
		return c.SendString("ok")
	})

	app.Get("/get", func(c fiber.Ctx) error {
		v, err := SessionManager.getValue(c, "sm-key")
		if err != nil {
			return c.Status(404).SendString(err.Error())
		}
		return c.SendString(v)
	})

	app.Get("/del", func(c fiber.Ctx) error {
		if err := SessionManager.delSession(c); err != nil {
			return c.Status(500).SendString(err.Error())
		}
		return c.SendString("deleted")
	})

	// Set the value
	req := httptest.NewRequest("GET", "/set", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200 on set, got %d", resp.StatusCode)
	}

	cookies := resp.Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected session cookie to be set on set route")
	}

	// Get the value using same cookies
	req = httptest.NewRequest("GET", "/get", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err = app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200 on get, got %d: %s", resp.StatusCode, string(body))
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "sm-value" {
		t.Fatalf("expected 'sm-value', got '%s'", string(body))
	}

	// Delete the session
	req = httptest.NewRequest("GET", "/del", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err = app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200 on del, got %d", resp.StatusCode)
	}

	// Attempt to get again - should fail
	req = httptest.NewRequest("GET", "/get", nil)
	// use cookies returned from delete (if any)
	for _, c := range resp.Cookies() {
		req.AddCookie(c)
	}
	resp, err = app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode == 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected non-200 after delete, got 200 with body: %s", string(body))
	}
}
