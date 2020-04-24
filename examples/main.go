package main

import (
	"log"
	"os"

	"github.com/gofiber/fiber"
	"github.com/markbates/goth"
	"github.com/markbates/goth/providers/google"

	"github.com/shareed2k/goth_fiber"
)

func main() {
	app := fiber.New()

	goth.UseProviders(
		google.New(os.Getenv("OAUTH_KEY"), os.Getenv("OAUTH_SECRET"), "http://127.0.0.1:8088/auth/callback"),
	)

	app.Get("/login", goth_fiber.BeginAuthHandler)
	app.Get("/auth/callback", func(ctx *fiber.Ctx) {
		user, err := goth_fiber.CompleteUserAuth(ctx)
		if err != nil {
			log.Fatal(err)
		}

		ctx.Send(user)

	})
	app.Get("/logout", func(ctx *fiber.Ctx) {
		if err := goth_fiber.Logout(ctx); err != nil {
			log.Fatal(err)
		}

		ctx.SendString("logout")
	})

	if err := app.Listen(8088); err != nil {
		log.Fatal(err)
	}
}
