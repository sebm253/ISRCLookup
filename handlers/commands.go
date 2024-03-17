package handlers

import (
	"isrc-lookup/internal"

	"github.com/disgoorg/disgo/handler"
)

func NewHandler(bot *internal.Bot) *Handler {
	h := &Handler{
		Bot:    bot,
		Router: handler.New(),
	}
	h.SlashCommand("/lookup", h.HandleISRCSlash)
	h.UserCommand("/Lookup ISRC from activities", h.HandleISRCContext)
	return h
}

type Handler struct {
	Bot *internal.Bot
	handler.Router
}
