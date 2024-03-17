package handlers

import (
	"isrc-lookup"

	"github.com/disgoorg/disgo/handler"
)

func NewHandler(bot *main.Bot) *Handler {
	h := &Handler{
		Bot:    bot,
		Router: handler.New(),
	}
	h.SlashCommand("/lookup", h.HandleISRCSlash)
	h.UserCommand("/Lookup ISRC from activities", h.HandleISRCContext)
	return h
}

type Handler struct {
	Bot *main.Bot
	handler.Router
}
