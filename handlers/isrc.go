package handlers

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/zmb3/spotify/v2"
)

var (
	trackRegex = regexp.MustCompile(`open\.spotify\.com/(intl-[a-z]{2}/)?track/(\w+)`)
)

const (
	spotifyActivityID = "spotify:1"

	lookupButtonLabel     = "🔎 Lookup on YouTube"
	youtubeSearchTemplate = `https://www.youtube.com/results?search_query="%s"`
)

func (h *Handler) HandleISRCSlash(data discord.SlashCommandInteractionData, event *handler.CommandEvent) error {
	messageBuilder := discord.NewMessageCreateBuilder().SetEphemeral(true)

	matches := trackRegex.FindStringSubmatch(data.String("url"))
	if matches == nil {
		return event.CreateMessage(messageBuilder.
			SetContent("Invalid track URL.").
			Build())
	}
	return h.handleISRCLookup(matches[2], event) // TODO possibly get rid of this matches bullshit some time
}

func (h *Handler) HandleISRCContext(data discord.UserCommandInteractionData, event *handler.CommandEvent) error {
	messageBuilder := discord.NewMessageCreateBuilder().SetAllowedMentions(&discord.AllowedMentions{}).SetEphemeral(true)

	caches := event.Client().Caches()
	presence, ok := caches.Presence(*event.GuildID(), data.TargetID())
	if !ok {
		return event.CreateMessage(messageBuilder.
			SetContentf("<@%d> has no presence.", data.TargetID()).
			Build())
	}
	for _, activity := range presence.Activities {
		if activity.ID != spotifyActivityID {
			continue
		}
		trackID := activity.SyncID
		if trackID == nil {
			return event.CreateMessage(messageBuilder.
				SetContentf("<@%d> is listening to a local track.", data.TargetID()).
				Build())
		}
		return h.handleISRCLookup(*trackID, event)
	}
	return event.CreateMessage(messageBuilder.
		SetContentf("<@%d> isn't listening to Spotify.", data.TargetID()).
		Build())
}

func (h *Handler) handleISRCLookup(trackID string, event *handler.CommandEvent) error {
	track, err := h.Bot.GetTrack(context.Background(), spotify.ID(trackID))
	if err != nil {
		return err
	}
	var artists []string
	for _, artist := range track.Artists {
		artists = append(artists, "**"+artist.Name+"**")
	}
	isrc := track.ExternalIDs["isrc"]
	return event.CreateMessage(discord.NewMessageCreateBuilder().
		SetContentf("ISRC for track **%s** by %s is **%s**.", track.Name, strings.Join(artists, ", "), isrc).
		SetEphemeral(true).
		AddActionRow(discord.NewLinkButton(lookupButtonLabel, fmt.Sprintf(youtubeSearchTemplate, isrc))).
		Build())
}
