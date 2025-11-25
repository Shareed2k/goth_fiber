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

	// Optionally, you can override the session manager here:
	// store := session.NewStore(session.Config{
	// 	KeyLookup:			"cookie:dinosaurus",
	// 	CookieHTTPOnly:	true,
	// 	Storage:				sqlite3.New(),
	// })
	//
	// goth_fiber.SessionManager = goth_fiber.NewSessionManager(store)

	goth.UseProviders(
		google.New(os.Getenv("OAUTH_KEY"), os.Getenv("OAUTH_SECRET"), "http://127.0.0.1:8088/auth/callback/google"),
	)

	sessConfig := session.Config{
		CookieSecure:    true,             // HTTPS only
		CookieHTTPOnly:  true,             // Prevent XSS
		CookieSameSite:  "Lax",            // CSRF protection
		IdleTimeout:     30 * time.Minute, // Session timeout
		AbsoluteTimeout: 24 * time.Hour,   // Maximum session life
		Extractor:       extractors.FromCookie("__xyz_session"),
	}

	handler, store := session.NewWithStore(sessConfig)
	goth_fiber.SessionManager = goth_fiber.NewSessionManager(store)

	app.Use(handler)

	app.Get("/login/:provider", goth_fiber.BeginAuthHandler)
	app.Get("/auth/callback/:provider", func(ctx fiber.Ctx) error {
		user, err := goth_fiber.CompleteUserAuth(ctx)
		if err != nil {
			log.Fatal(err)
		}

		return ctx.SendString(user.Email)
	})
	app.Get("/logout", func(ctx fiber.Ctx) error {
		if err := goth_fiber.Logout(ctx); err != nil {
			log.Fatal(err)
		}

		return ctx.SendString("logout")
	})

	if err := app.Listen(":8077"); err != nil {
		log.Fatal(err)
	}
}
