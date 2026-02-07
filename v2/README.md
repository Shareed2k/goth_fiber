# Goth-Fiber v2 (Fiber v3)

This module targets Fiber v3.

## Installation

```text
$ go get github.com/shareed2k/goth_fiber/v2
```

## Example

```go
package main

import (
    "log"
    "os"
    "time"

    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/extractors"
    "github.com/gofiber/fiber/v3/middleware/session"
    "github.com/markbates/goth"
    "github.com/markbates/goth/providers/google"

    "github.com/shareed2k/goth_fiber/v2"
)

func main() {
    app := fiber.New()

    goth.UseProviders(
        google.New(os.Getenv("OAUTH_KEY"), os.Getenv("OAUTH_SECRET"), "http://127.0.0.1:8088/auth/callback/google"),
    )

    sessConfig := session.Config{
        CookieSecure:    os.Getenv("ENVIRONMENT") == "production",
        CookieHTTPOnly:  true,
        CookieSameSite:  "Lax",
        IdleTimeout:     30 * time.Minute,
        AbsoluteTimeout: 24 * time.Hour,
        Extractor:       extractors.FromCookie("__xyz_session"),
    }

    handler, store := session.NewWithStore(sessConfig)
    goth_fiber.SessionManager = goth_fiber.NewSessionManager(store)

    app.Use(handler)

    app.Get("/login/:provider", goth_fiber.BeginAuthHandler)
    app.Get("/auth/callback/:provider", func(ctx fiber.Ctx) error {
        user, err := goth_fiber.CompleteUserAuth(ctx)
        if err != nil {
            log.Printf("auth callback error: %v", err)
            return ctx.Status(fiber.StatusBadRequest).SendString(err.Error())
        }

        return ctx.SendString(user.Email)
    })
    app.Get("/logout", func(ctx fiber.Ctx) error {
        if err := goth_fiber.Logout(ctx); err != nil {
            log.Printf("logout error: %v", err)
            return ctx.Status(fiber.StatusInternalServerError).SendString(err.Error())
        }

        return ctx.SendString("logout")
    })

    if err := app.Listen(":8077"); err != nil {
        log.Fatal(err)
    }
}
```

## Session management

By default, a cookie-based session store is created in `init()`.
You can replace it at startup by creating a new store and wrapping it
with a session manager:

```go
store := session.NewStore(session.Config{
    CookieHTTPOnly: true,
    CookieSecure:   os.Getenv("ENVIRONMENT") == "production",
})

goth_fiber.SessionManager = goth_fiber.NewSessionManager(store)
```
