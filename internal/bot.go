package internal

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/lmittmann/tint"
	"github.com/zmb3/spotify/v2"
	spotifyauth "github.com/zmb3/spotify/v2/auth"
	"golang.org/x/oauth2/clientcredentials"
)

var (
	spotifyClientID     = os.Getenv("ISRC_LOOKUP_CLIENT_ID")
	spotifyClientSecret = os.Getenv("ISRC_LOOKUP_CLIENT_SECRET")
)

const (
	spotifyRefreshDelay = 40 * time.Minute
)

type Bot struct {
	*spotify.Client
}

func (b *Bot) InitSpotifyClient() error {
	spotifyConfig := &clientcredentials.Config{
		ClientID:     spotifyClientID,
		ClientSecret: spotifyClientSecret,
		TokenURL:     spotifyauth.TokenURL,
	}
	spotifyToken, err := spotifyConfig.Token(context.Background())
	if err != nil {
		slog.Error("failed to obtain spotify auth token", tint.Err(err))

		time.AfterFunc(time.Minute*1, func() {
			b.InitSpotifyClient()
		})
		return err
	}
	httpClient := spotifyauth.New().Client(context.Background(), spotifyToken)
	b.Client = spotify.New(httpClient)

	time.AfterFunc(spotifyRefreshDelay, func() {
		b.InitSpotifyClient()
	})
	return nil
}
