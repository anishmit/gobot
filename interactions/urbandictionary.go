package interactions

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"github.com/bwmarrin/discordgo"
	"io"
	"encoding/json"
	"sort"
	"strings"
	"time"
)

type Result struct {
	Definition string `json:"definition"`
	Permalink string `json:"permalink"`
	ThumbsUp int `json:"thumbs_up"`
	Author string `json:"author"`
	Word string `json:"word"`
	DefID int64 `json:"defid"`
	CurrentVote string `json:"current_vote"`
	WrittenOn string `json:"written_on"`
	Example string `json:"example"`
	ThumbsDown int64 `json:"thumbs_down"`
}

type UDResponse struct {
	List []Result `json:"list"`
}

var UDResponses = map[string]UDResponse{}

func getUDResponse(term string) (UDResponse, error) {
	resp, err := http.Get(fmt.Sprintf("https://api.urbandictionary.com/v0/define?term=%s", url.QueryEscape(term)))
	if err != nil {
		return UDResponse{}, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return UDResponse{}, err
	}
	var response UDResponse
	err = json.Unmarshal(data, &response)
	if err != nil {
		return UDResponse{}, err
	}
	// Sort list correctly
	sort.Slice(response.List, func(i, j int) bool {
		equalI := strings.EqualFold(response.List[i].Word, term)
		equalJ := strings.EqualFold(response.List[j].Word, term)
		if (equalI != equalJ) {
			return equalI
		}
		return response.List[i].ThumbsUp > response.List[j].ThumbsUp
	})
	return response, nil
}

func getNonEmptyStringWithMaxLen(s string, maxLen int) string {
	if len(s) > 0 {
		return s[:min(len(s), maxLen)]
	} else {
		return "\u200B"
	}
}

func init() {
	Commands = append(Commands, &discordgo.ApplicationCommand{
		Name:        "ud",
		Description: "Search Urban Dictionary",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type: discordgo.ApplicationCommandOptionString,
				Name: "term",
				Description: "Term to search for",
				Required: true,
			},
		},
	})
	CommandHandlers["ud"] = func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		})
		term := i.ApplicationCommandData().Options[0].StringValue()
		response, err := getUDResponse(term)
		if err != nil {
			log.Println("Getting Urban Dictionary response failed", err)
			return
		}
		if len(response.List) == 0 {
			s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
				Embeds: []*discordgo.MessageEmbed{
					{
						Title: getNonEmptyStringWithMaxLen(term, 256),
						Color: 0xff0000,
						Description: "No results were found.",
					},
				},
			})
		} else {
			t, err := time.Parse("2006-01-02T15:04:05.000Z", response.List[0].WrittenOn)
			if err != nil {
				log.Println("Failed at parsing date", err)
				return
			}
			s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
				Embeds: []*discordgo.MessageEmbed{
					{
						Title: getNonEmptyStringWithMaxLen(term, 256),
						Color: 0xff8000,
						Fields: []*discordgo.MessageEmbedField{
							{
								Name: "Definition",
								Value: getNonEmptyStringWithMaxLen(response.List[0].Definition, 1024),
							},
							{
								Name: "Example",
								Value: getNonEmptyStringWithMaxLen(response.List[0].Example, 1024),
							},
							{
								Name: "Author",
								Value: getNonEmptyStringWithMaxLen(response.List[0].Author, 1024),
								Inline: true,
							},
							{
								Name: "Date",
								Value: fmt.Sprintf("<t:%d:d>", t.Unix()),
								Inline: true,
							},
							{
								Name: "\u200B",
								Value: fmt.Sprintf("👍 %d\u00A0\u00A0\u00A0\u00A0👎 %d", response.List[0].ThumbsUp, response.List[0].ThumbsDown),
								Inline: true,
							},
						},
					},
				},
			})
		}
	}
}
