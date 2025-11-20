package goth_fiber

import "github.com/gofiber/fiber/v3"

type Params struct {
	ctx fiber.Ctx
}

func (p *Params) Get(key string) string {
	return p.ctx.Query(key)
}
