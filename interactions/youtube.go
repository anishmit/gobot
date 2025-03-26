package interactions

import (
	"os/exec"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"encoding/json"
	"github.com/bwmarrin/discordgo"
	"strings"
	"github.com/jonas747/ogg"
)

const MAX_RESULTS = "25"

type SearchResult struct {
	Title string
	ID string
	Duration string
}	

func inVoiceChannel(s *discordgo.Session, guildID, userID string) (bool, string) {
	// An error means that the user isn't in a VC
	if voiceState, err := s.State.VoiceState(guildID, userID); err != nil {
		return false, ""
	} else {
		return true, voiceState.ChannelID
	}
}

func search(query string) ([]SearchResult, error) {
	URL1, err := url.Parse("https://www.googleapis.com/youtube/v3/search")
	if err != nil {
		return nil, err
	}
	parameters1 := url.Values{}
	parameters1.Add("key", os.Getenv("YOUTUBE_API_KEY"))
	parameters1.Add("part", "snippet")
	parameters1.Add("type", "video")
	parameters1.Add("maxResults", MAX_RESULTS)
	parameters1.Add("q", query)
	URL1.RawQuery = parameters1.Encode()
	res1, err := http.Get(URL1.String())

	if err != nil {
		return nil, err
	}
	var data1 map[string]any
	json.NewDecoder(res1.Body).Decode(&data1)
	items1 := data1["items"].([]any)
	
	URL2, err := url.Parse("https://www.googleapis.com/youtube/v3/videos")
	if err != nil {
		return nil, err
	}
	parameters2 := url.Values{}
	parameters2.Add("key", os.Getenv("YOUTUBE_API_KEY"))
	parameters2.Add("part", "contentDetails")
	parameters2.Add("maxResults", MAX_RESULTS)
	commaSeparatedIDs := ""
	for _, item := range items1 {
		commaSeparatedIDs += item.(map[string]any)["id"].(map[string]any)["videoId"].(string) + ","
	}
	parameters2.Add("id", commaSeparatedIDs)
	URL2.RawQuery = parameters2.Encode()
	res2, err := http.Get(URL2.String())

	if err != nil {
		return nil, err
	}
	var data2 map[string]any
	json.NewDecoder(res2.Body).Decode(&data2)
	items2 := data2["items"].([]any)
	
	var results []SearchResult
	for i, item1 := range items1 {
		results = append(results, SearchResult{
			Title: item1.(map[string]any)["snippet"].(map[string]any)["title"].(string),
			ID: item1.(map[string]any)["id"].(map[string]any)["videoId"].(string),
			Duration: strings.ToLower(strings.Replace(items2[i].(map[string]any)["contentDetails"].(map[string]any)["duration"].(string)[1:], "T", "", 1)),
		})
	}
	return results, nil
}


func init() {
	Commands = append(Commands, &discordgo.ApplicationCommand{
		Name:        "yt",
		Description: "Play YouTube video",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type: discordgo.ApplicationCommandOptionString,
				Name: "query",
				Description: "Search query",
				Required: true,
			},
		},
	})
	CommandHandlers["yt"] = func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		})
		// Check to make sure user is connected to a voice channel
		if i.Member == nil {
			s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
				Content: "You must be in a guild voice channel to use this command.",
			})
			return
		}
		inVC, _ := inVoiceChannel(s, i.GuildID, i.Member.User.ID)
		if !inVC {
			s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
				Content: "You must be in a voice channel to use this command.",
			})
			return
		}
		searchQuery := i.ApplicationCommandData().Options[0].StringValue()
		searchResults, err := search(searchQuery)
		if err != nil {
			s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
				Content: "Search request failed.",
			})
			return
		} else if len(searchResults) == 0 {
			s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
				Content: fmt.Sprintf("No results found for %s.", searchQuery[:min(1978, len(searchQuery))]),
			})
			return
		}
		var selectMenuOptions []discordgo.SelectMenuOption 
		for _, searchResult := range searchResults {
			selectMenuOptions = append(selectMenuOptions, discordgo.SelectMenuOption{
				Label: searchResult.Title[:min(100, len(searchResult.Title))],
				Value: searchResult.ID,
				Description: searchResult.Duration,
			})
		}
		placeholderText := fmt.Sprintf("Results for %s", searchQuery)
		s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.SelectMenu{
							MenuType: discordgo.StringSelectMenu,
							CustomID: "ytSelect",
							Placeholder: placeholderText[:min(150, len(placeholderText))],
							MaxValues: len(selectMenuOptions),
							Options: selectMenuOptions,
						},
					},
				},
			},
		})
	}
	ComponentHandlers["ytSelect"] = func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredMessageUpdate,
		})
		log.Println(i.Message.ReferencedMessage)
		videoID := i.MessageComponentData().Values[0]
		inVC, channelID := inVoiceChannel(s, i.GuildID, i.Member.User.ID)
		if !inVC {
			content := "You must be in a voice channel to use this command."
			s.FollowupMessageEdit(i.Interaction, i.Message.ID, &discordgo.WebhookEdit{
				Content: &content,
			})
			return
		}
		content := "Playing"
		s.FollowupMessageEdit(i.Interaction, i.Message.ID, &discordgo.WebhookEdit{
			Content: &content,
		})
		// UNFINISHED //
		voice, err := s.ChannelVoiceJoin(i.GuildID, channelID, false, false)
		if err != nil {
			log.Println("Could not join voice channel", err)
			return
		}
		cmd1 := exec.Command("yt-dlp", "-f", "ba", "-o", "-", fmt.Sprintf("https://youtube.com/watch?v=%s", videoID))
		cmd2 := exec.Command("ffmpeg", "-i", "-", "-c:a", "libopus", "-b:a", "96K", "-ar", "48000", "-ac", "2", "-f", "opus", "-")
		cmd2.Stdin, err = cmd1.StdoutPipe()
		if err != nil {
			log.Println("Could not get command 1 standard output pipe", err)
			return
		}
		pipe, err := cmd2.StdoutPipe()
		if err != nil {
			log.Println("Could not get command 2 standard output pipe", err)
			return
		}
		if err = cmd1.Start(); err != nil {
			log.Println("Could not start command 1", err)
		}
		if err = cmd2.Start(); err != nil {
			log.Println("Could not start command 2", err)
		}
		decoder := ogg.NewPacketDecoder(ogg.NewDecoder(pipe))
		voice.Speaking(true)
		for {
			packet, _, err := decoder.Decode()
			if err != nil {
				log.Println("Could not decode", err)
				break
			}
			voice.OpusSend <- packet
		}
		log.Println("Finished sending packets")
		voice.Speaking(false)
		cmd2.Wait()
		cmd1.Wait()

	}
}