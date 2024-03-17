package main

import (
	"context"
	"isrc-lookup/handlers"
	"isrc-lookup/internal"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/cache"
	"github.com/disgoorg/disgo/gateway"
	"github.com/lmittmann/tint"
)

func main() {
	b := &internal.Bot{}
	if err := b.InitSpotifyClient(); err != nil {
		panic(err)
	}
	slog.Info("spotify client initialized")

	h := handlers.NewHandler(b)

	logger := tint.NewHandler(os.Stdout, &tint.Options{
		Level: slog.LevelInfo,
	})
	slog.SetDefault(slog.New(logger))

	slog.Info("starting the bot...", slog.String("disgo.version", disgo.Version))

	client, err := disgo.New(os.Getenv("ISRC_LOOKUP_TOKEN"),
		bot.WithGatewayConfigOpts(gateway.WithIntents(gateway.IntentGuildPresences, gateway.IntentGuilds),
			gateway.WithPresenceOpts(gateway.WithListeningActivity("Spotify"))),
		bot.WithCacheConfigOpts(cache.WithCaches(cache.FlagPresences)),
		bot.WithEventListeners(h),
	)
	if err != nil {
		panic(err)
	}

	defer client.Close(context.TODO())

	if err := client.OpenGateway(context.TODO()); err != nil {
		panic(err)
	}

	slog.Info("isrc lookup bot is now running.")
	s := make(chan os.Signal, 1)
	signal.Notify(s, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-s
}
