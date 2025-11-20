package main

import (
	"log"
	"os"

	"github.com/gofiber/fiber/v3"
	"github.com/markbates/goth"
	"github.com/markbates/goth/providers/google"

	"github.com/wakatara/goth_fiber"
)

func main() {
	app := fiber.New()

	// Optionally, you can override the session store here:
	// goth_fiber.SessionStore = session.NewStore(session.Config{
	// 	CookieName:     "dinosaurus",
	// 	CookieHTTPOnly: true,
	// 	Storage:        sqlite3.New(),
	// })

	goth.UseProviders(
		google.New(os.Getenv("OAUTH_KEY"), os.Getenv("OAUTH_SECRET"), "http://127.0.0.1:8088/auth/callback/google"),
	)

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

	if err := app.Listen(":8088"); err != nil {
		log.Fatal(err)
	}
}
