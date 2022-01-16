package main

import (
	"context"
	"github.com/DisgoOrg/disgo/core/bot"
	"github.com/DisgoOrg/disgo/core/events"
	"github.com/DisgoOrg/disgo/discord"
	"github.com/DisgoOrg/disgo/httpserver"
	"github.com/DisgoOrg/disgo/info"
	"github.com/DisgoOrg/log"
	"github.com/zmb3/spotify/v2"
	spotifyauth "github.com/zmb3/spotify/v2/auth"
	"golang.org/x/oauth2/clientcredentials"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
)

var (
	token      = os.Getenv("isrcLookupToken")
	pub        = os.Getenv("isrcLookupPub")
	trackRegex = regexp.MustCompile("open\\.spotify\\.com/track/(\\w+)")

	ctx           context.Context
	spotifyClient spotify.Client

	commands = []discord.ApplicationCommandCreate{
		discord.SlashCommandCreate{
			Name:        "lookup",
			Description: "Performs a lookup for the track ISRC",
			Options: []discord.ApplicationCommandOption{
				discord.ApplicationCommandOptionSubCommand{
					Name:        "url",
					Description: "performs a lookup based on the spotify url",
					Options: []discord.ApplicationCommandOption{
						discord.ApplicationCommandOptionString{
							Name:        "url",
							Description: "the url to lookup the ISRC for",
							Required:    true,
						},
					},
				},
				discord.ApplicationCommandOptionSubCommand{
					Name:        "search",
					Description: "performs a lookup based on the query",
					Options: []discord.ApplicationCommandOption{
						discord.ApplicationCommandOptionString{
							Name:        "query",
							Description: "the query to lookup the ISRC for",
							Required:    true,
						},
					},
				},
			},
		},
	}
)

func main() {
	log.SetLevel(log.LevelInfo)
	log.Info("starting the bot...")
	log.Info("disgo version: ", info.Version)

	disgo, err := bot.New(token,
		bot.WithHTTPServerOpts(
			httpserver.WithURL("/lookup"),
			httpserver.WithPort(":6665"),
			httpserver.WithPublicKey(pub),
		),
		bot.WithEventListeners(&events.ListenerAdapter{
			OnApplicationCommandInteraction: onSlashCommand,
		}),
	)
	if err != nil {
		log.Fatal("error while building disgo instance: ", err)
		return
	}

	defer disgo.Close(context.TODO())

	if _, err := disgo.SetGuildCommands("919338422173331576", commands); err != nil { // TODO remove this constant and make the commands global
		log.Fatal("error while registering commands: ", err)
	}

	if err = disgo.StartHTTPServer(); err != nil {
		log.Fatal("error while starting http server: ", err)
	}

	initSpotifyClient()

	log.Infof("isrc lookup bot is now running. Press CTRL-C to exit.")
	s := make(chan os.Signal, 1)
	signal.Notify(s, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-s
}

func initSpotifyClient() {
	ctx = context.Background()
	spotifyConfig := &clientcredentials.Config{
		ClientID:     os.Getenv("isrcLookupClientId"),
		ClientSecret: os.Getenv("isrcLookupClientSecret"),
		TokenURL:     spotifyauth.TokenURL,
	}
	spotifyToken, err := spotifyConfig.Token(ctx)
	if err != nil {
		log.Fatal("failed to obtain spotify auth token: ", err)
	}
	httpClient := spotifyauth.New().Client(ctx, spotifyToken)
	spotifyClient = *spotify.New(httpClient)
}

func onSlashCommand(event *events.ApplicationCommandInteractionEvent) {
	data := event.SlashCommandInteractionData()
	options := data.Options
	messageBuilder := discord.NewMessageCreateBuilder()
	if *data.SubCommandName == "url" {
		match := trackRegex.FindStringSubmatch(*options.String("url"))
		if match == nil {
			event.Create(messageBuilder.
				SetContent("Invalid track url.").
				SetEphemeral(true).
				Build())
		} else {
			isrcResponse := lookupISRC(match[1])
			var artists []string
			for _, artist := range isrcResponse.Artist {
				artists = append(artists, "**"+artist.Name+"**")
			}
			joined := strings.Join(artists, ", ")
			event.Create(messageBuilder.
				SetContentf("ISRC for track **%s** by %s is **%s**.", isrcResponse.Name, joined, isrcResponse.ISRC).
				SetEphemeral(true).
				AddActionRow(
					discord.NewDangerButton("Lookup on YouTube", discord.CustomID(isrcResponse.ISRC)).
						WithEmoji(discord.ComponentEmoji{
							Name: "🔎",
						}),
				).
				Build())
		}
	}
}

type ISRCResponse struct {
	ISRC   string
	Artist []spotify.SimpleArtist
	Name   string
}

func lookupISRC(trackId string) *ISRCResponse {
	track, err := spotifyClient.GetTrack(ctx, spotify.ID(trackId))
	if err != nil {
		log.Fatalf("there was an error while looking up track %s: ", trackId, err)
		return nil
	}
	isrc, ok := track.ExternalIDs["isrc"]
	if !ok {
		return nil
	}
	return &ISRCResponse{
		ISRC:   isrc,
		Artist: track.Artists,
		Name:   track.Name,
	}
}
