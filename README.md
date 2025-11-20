# Goth-Fiber: Multi-Provider Authentication for Go [![GoDoc](https://godoc.org/github.com/wakatara/goth_fiber?status.svg)](https://godoc.org/github.com/wakatara/goth_fiber)

A wrapper for [goth library](https://github.com/markbates/goth) to use with [Fiber Framework v3](https://github.com/gofiber/fiber), providing a simple, clean, and idiomatic way to write authentication packages for Go web applications.

Unlike other similar packages, Goth lets you write OAuth, OAuth2, or any other protocol providers, as long as they implement the `Provider` and `Session` interfaces.

## Requirements

- Go 1.25 or higher
- Fiber v3.0.0-beta.4 or higher

## Installation

```bash
go get github.com/wakatara/goth_fiber
```

## Supported Providers

- Amazon
- Apple
- Auth0
- Azure AD
- Battle.net
- Bitbucket
- Box
- Cloud Foundry
- Dailymotion
- Deezer
- Digital Ocean
- Discord
- Dropbox
- Eve Online
- Facebook
- Fitbit
- Gitea
- GitHub
- Gitlab
- Google
- Heroku
- InfluxCloud
- Instagram
- Intercom
- Kakao
- Lastfm
- Linkedin
- LINE
- Mailru
- Meetup
- MicrosoftOnline
- Naver
- Nextcloud
- OneDrive
- OpenID Connect (auto discovery)
- Paypal
- SalesForce
- Shopify
- Slack
- Soundcloud
- Spotify
- Steam
- Strava
- Stripe
- Tumblr
- Twitch
- Twitter
- Typetalk
- Uber
- VK
- Wepay
- Xero
- Yahoo
- Yammer
- Yandex

## Quick Start

```go
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

    goth.UseProviders(
        google.New(os.Getenv("OAUTH_KEY"), os.Getenv("OAUTH_SECRET"), "http://127.0.0.1:8088/auth/callback/google"),
    )

    app.Get("/login/:provider", goth_fiber.BeginAuthHandler)
    app.Get("/auth/callback/:provider", func(ctx fiber.Ctx) error {
        user, err := goth_fiber.CompleteUserAuth(ctx)
        if err != nil {
            return ctx.Status(fiber.StatusInternalServerError).SendString(err.Error())
        }
        return ctx.SendString(user.Email)
    })
    app.Get("/logout", func(ctx fiber.Ctx) error {
        if err := goth_fiber.Logout(ctx); err != nil {
            return ctx.Status(fiber.StatusInternalServerError).SendString(err.Error())
        }
        return ctx.SendString("logged out")
    })

    log.Fatal(app.Listen(":8088"))
}
```

## Examples

See the [examples](examples) folder for a working application that lets users authenticate through Google and other providers.

To run the example:

```bash
git clone https://github.com/wakatara/goth_fiber.git
cd goth_fiber/examples
export OAUTH_KEY=your-google-client-id
export OAUTH_SECRET=your-google-client-secret
go run main.go
```

Now open up your browser and go to [http://localhost:8088/login/google](http://localhost:8088/login/google) to see the example.

## Security Notes

By default, goth_fiber uses a `Store` from the `gofiber/fiber/v3/middleware/session` package to store session data.

As configured, goth_fiber will generate cookies with the following `session.Config`:

```go
session.Config{
    Extractor:      extractors.FromCookie("_gothic_session"),
    CookieHTTPOnly: true,
}
```

To tailor these fields for your application, you can override the `goth_fiber.SessionStore` variable at startup.

The following snippet shows one way to do this:

```go
import (
    "time"
    "github.com/gofiber/fiber/v3/extractors"
    "github.com/gofiber/fiber/v3/middleware/session"
    "github.com/gofiber/storage/sqlite3"
)

// Custom session configuration
goth_fiber.SessionStore = session.NewStore(session.Config{
    Extractor:       extractors.FromCookie("my_session"),
    Storage:         sqlite3.New(), // From github.com/gofiber/storage/sqlite3
    IdleTimeout:     30 * time.Minute,
    AbsoluteTimeout: 24 * time.Hour,
    CookieDomain:    "example.com",
    CookiePath:      "/",
    CookieSecure:    true, // Enable for HTTPS
    CookieHTTPOnly:  true, // Should always be enabled
    CookieSameSite:  "Lax",
})
```

## Migration from Fiber v2

If you're migrating from the original goth_fiber (Fiber v2), note these key changes:

1. **Handler signatures**: `*fiber.Ctx` is now `fiber.Ctx` (interface, not pointer)
2. **Session configuration**: `KeyLookup` is replaced with `Extractor` functions
3. **Import path**: Use `github.com/wakatara/goth_fiber` for v3 support

## Issues

Issues always stand a significantly better chance of getting fixed if they are accompanied by a pull request.

## Contributing

Would I love to see more providers? Certainly! Would you love to contribute one? Hopefully, yes!

1. Fork it
2. Create your feature branch (`git checkout -b my-new-feature`)
3. Write Tests!
4. Commit your changes (`git commit -am 'Add some feature'`)
5. Push to the branch (`git push origin my-new-feature`)
6. Create new Pull Request

## License

MIT License - see [LICENSE](LICENSE) for details.

## Acknowledgments

- Original [goth_fiber](https://github.com/Shareed2k/goth_fiber) by Shareed2k
- [Goth](https://github.com/markbates/goth) by Mark Bates
- [Fiber](https://github.com/gofiber/fiber) by the Fiber team
