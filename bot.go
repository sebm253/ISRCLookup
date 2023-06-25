package main

import (
	"context"
	"fmt"
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
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"
)

const (
	youtubeSearchTemplate = `https://www.youtube.com/results?search_query="%s"`
	spotifyActivityID     = "spotify:1"
)

var (
	spotifyClientID     = os.Getenv("ISRC_LOOKUP_CLIENT_ID")
	spotifyClientSecret = os.Getenv("ISRC_LOOKUP_CLIENT_SECRET")
	trackRegex          = regexp.MustCompile(`open\.spotify\.com/(intl-[a-z]{2}/)?track/(\w+)`)

	spotifyClient spotify.Client
)

func main() {
	initSpotifyClient(false)

	log.SetLevel(log.LevelInfo)
	log.Info("starting the bot...")
	log.Info("disgo version: ", disgo.Version)

	client, err := disgo.New(os.Getenv("ISRC_LOOKUP_TOKEN"),
		bot.WithGatewayConfigOpts(gateway.WithIntents(gateway.IntentGuildPresences, gateway.IntentGuilds),
			gateway.WithPresenceOpts(gateway.WithListeningActivity("Spotify"))),
		bot.WithCacheConfigOpts(cache.WithCaches(cache.FlagPresences)),
		bot.WithEventListeners(&events.ListenerAdapter{
			OnApplicationCommandInteraction: onCommand,
		}),
	)
	if err != nil {
		log.Fatal("error while building disgo instance: ", err)
	}

	defer client.Close(context.TODO())

	if err := client.OpenGateway(context.TODO()); err != nil {
		log.Fatal("error while connecting to the gateway: ", err)
	}

	log.Info("isrc lookup bot is now running.")
	s := make(chan os.Signal, 1)
	signal.Notify(s, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-s
}

func initSpotifyClient(retry bool) {
	spotifyConfig := &clientcredentials.Config{
		ClientID:     spotifyClientID,
		ClientSecret: spotifyClientSecret,
		TokenURL:     spotifyauth.TokenURL,
	}
	ctx := context.Background()
	spotifyToken, err := spotifyConfig.Token(ctx)
	if err != nil {
		if retry {
			log.Error("failed to obtain spotify auth token: ", err)
			time.AfterFunc(time.Minute*1, func() {
				initSpotifyClient(true)
			})
			return
		} else {
			log.Fatal("failed to obtain spotify auth token: ", err)
		}
	}
	httpClient := spotifyauth.New().Client(ctx, spotifyToken)
	spotifyClient = *spotify.New(httpClient)
	if !retry {
		log.Info("spotify client initialized.")
	}
	time.AfterFunc(time.Minute*40, func() {
		initSpotifyClient(true)
	})
}

func onCommand(event *events.ApplicationCommandInteractionCreate) {
	data := event.Data
	switch data := data.(type) {
	case discord.SlashCommandInteractionData:
		match := trackRegex.FindStringSubmatch(data.String("url"))
		if match == nil {
			createMessage("Invalid track URL.", event)
		} else {
			sendISRCDetails(match[2], event)
		}
	case discord.UserCommandInteractionData:
		caches := event.Client().Caches()
		presence, ok := caches.Presence(*event.GuildID(), data.TargetID())
		if !ok {
			createMessage("The user has no presence.", event)
			return
		}
		for _, activity := range presence.Activities {
			if activity.ID != spotifyActivityID {
				continue
			}
			trackID := activity.SyncID
			if trackID == nil {
				createMessage("The user is listening to a local track.", event)
				return
			}
			sendISRCDetails(*trackID, event)
		}
		createMessage("The user isn't listening to Spotify.", event)
	}
}

func sendISRCDetails(trackID string, event *events.ApplicationCommandInteractionCreate) {
	track, err := spotifyClient.GetTrack(context.Background(), spotify.ID(trackID))
	if err != nil {
		createMessage(fmt.Sprintf("there was an error while looking up track %s: %s", trackID, err), event)
		return
	}
	var artists []string
	for _, artist := range track.Artists {
		artists = append(artists, "**"+artist.Name+"**")
	}
	isrc := track.ExternalIDs["isrc"]
	_ = event.CreateMessage(discord.NewMessageCreateBuilder().
		SetContentf("ISRC for track **%s** by %s is **%s**.", track.Name, strings.Join(artists, ", "), isrc).
		SetEphemeral(true).
		AddActionRow(discord.NewLinkButton("🔎 Lookup on YouTube", fmt.Sprintf(youtubeSearchTemplate, isrc))).
		Build())
}

func createMessage(content string, event *events.ApplicationCommandInteractionCreate) {
	_ = event.CreateMessage(discord.NewMessageCreateBuilder().
		SetContent(content).
		SetEphemeral(true).
		Build())
}
