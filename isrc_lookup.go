package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/cache"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/gateway"
	"github.com/disgoorg/log"
	"github.com/zmb3/spotify/v2"
	spotifyauth "github.com/zmb3/spotify/v2/auth"
	"golang.org/x/oauth2/clientcredentials"
)

var (
	token                 = os.Getenv("ISRC_LOOKUP_TOKEN")
	spotifyClientId       = os.Getenv("ISRC_LOOKUP_CLIENT_ID")
	spotifyClientSecret   = os.Getenv("ISRC_LOOKUP_CLIENT_SECRET")
	trackRegex            = regexp.MustCompile(`open\.spotify\.com/track/(\w+)`)
	youtubeSearchTemplate = `https://www.youtube.com/results?search_query="%s"`

	spotifyClient spotify.Client
)

func main() {
	log.SetLevel(log.LevelInfo)
	log.Info("starting the bot...")
	log.Info("disgo version: ", disgo.Version)

	client, err := disgo.New(token,
		bot.WithGatewayConfigOpts(gateway.WithIntents(gateway.IntentsNone),
			gateway.WithPresenceOpts(gateway.WithListeningActivity("Spotify"))),
		bot.WithCacheConfigOpts(cache.WithCaches(cache.FlagsNone)),
		bot.WithEventListeners(&events.ListenerAdapter{
			OnApplicationCommandInteraction: onSlashCommand,
		}),
	)
	if err != nil {
		log.Fatal("error while building disgo instance: ", err)
	}

	defer client.Close(context.TODO())

	if err := client.OpenGateway(context.TODO()); err != nil {
		log.Fatal("error while connecting to the gateway: ", err)
	}

	initSpotifyClient(false)

	log.Info("isrc lookup bot is now running.")
	s := make(chan os.Signal, 1)
	signal.Notify(s, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-s
}

func initSpotifyClient(retry bool) {
	spotifyConfig := &clientcredentials.Config{
		ClientID:     spotifyClientId,
		ClientSecret: spotifyClientSecret,
		TokenURL:     spotifyauth.TokenURL,
	}
	ctx := context.Background()
	spotifyToken, err := spotifyConfig.Token(ctx)
	if err != nil {
		if retry {
			log.Error("failed to obtain spotify auth token: ", err)
			go func() {
				time.Sleep(time.Minute * 1)
				initSpotifyClient(true)
			}()
			return
		} else {
			log.Fatal("failed to obtain spotify auth token: ", err)
		}
	}
	httpClient := spotifyauth.New().Client(ctx, spotifyToken)
	spotifyClient = *spotify.New(httpClient)
	log.Info("spotify client initialized.")
	go func() { // troll face
		time.Sleep(time.Minute * 40)
		initSpotifyClient(true)
	}()
}

func onSlashCommand(event *events.ApplicationCommandInteractionCreate) {
	data := event.SlashCommandInteractionData()
	messageBuilder := discord.NewMessageCreateBuilder()
	match := trackRegex.FindStringSubmatch(data.String("url"))
	if match == nil {
		_ = event.CreateMessage(messageBuilder.
			SetContent("Invalid track url.").
			SetEphemeral(true).
			Build())
	} else {
		isrcResponse, err := lookupISRC(match[1])
		if err != nil {
			_ = event.CreateMessage(messageBuilder.
				SetContentf("there was an error while looking up the track ISRC: %s", err).
				SetEphemeral(true).
				Build())
			return
		}
		var artists []string
		for _, artist := range isrcResponse.Artists {
			artists = append(artists, "**"+artist.Name+"**")
		}
		joined := strings.Join(artists, ", ")
		_ = event.CreateMessage(messageBuilder.
			SetContentf("ISRC for track **%s** by %s is **%s**.", isrcResponse.Name, joined, isrcResponse.ISRC).
			SetEphemeral(true).
			AddActionRow(discord.NewLinkButton("🔎 Lookup on YouTube", fmt.Sprintf(youtubeSearchTemplate, isrcResponse.ISRC))).
			Build())
	}
}

type ISRCResponse struct {
	ISRC    string
	Artists []spotify.SimpleArtist
	Name    string
}

func lookupISRC(trackId string) (*ISRCResponse, error) {
	track, err := spotifyClient.GetTrack(context.Background(), spotify.ID(trackId))
	if err != nil {
		log.Errorf("there was an error while looking up track %s: ", trackId, err)
		return nil, err
	}
	isrc, ok := track.ExternalIDs["isrc"]
	if !ok {
		return nil, err
	}
	return &ISRCResponse{
		ISRC:    isrc,
		Artists: track.Artists,
		Name:    track.Name,
	}, nil
}
