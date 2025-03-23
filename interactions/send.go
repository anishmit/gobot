package interactions

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
	"os"
	"github.com/bwmarrin/discordgo"
)

var (
	REQUEST_BODY = map[string]string{
		"content": "Scheduled message sent.",
	}
	AUTHORIZATION_HEADER = fmt.Sprintf("Bot %s", os.Getenv("BOT_TOKEN"))
	jsonBody []byte
)

func sendScheduledMessage(channelID string) {
	if req, err := http.NewRequest("POST", fmt.Sprintf("https://discord.com/api/channels/%s/messages", channelID), bytes.NewBuffer(jsonBody)); err != nil {
		log.Println("Could not create new HTTP request", err)
	} else {
		req.Header.Add("Authorization", AUTHORIZATION_HEADER)
		req.Header.Add("Content-Type", "application/json")
		http.DefaultClient.Do(req)
	}
}

func init() {
	var err error
	jsonBody, err = json.Marshal(REQUEST_BODY)
	if err != nil {
		log.Fatalln("Request body could not be JSON encoded", err)
	}
	Commands = append(Commands, &discordgo.ApplicationCommand{
		Name:        "send",
		Description: "Schedule sending a message at a Unix epoch time in milliseconds",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type: discordgo.ApplicationCommandOptionInteger,
				Name: "time",
				Description: "Unix epoch time in milliseconds",
				Required: true,
			},
		},
	})
	CommandHandlers["send"] = func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		sendTime := i.ApplicationCommandData().Options[0].IntValue()
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Scheduled message send.",
				Flags: discordgo.MessageFlagsEphemeral,
			},
		})
		time.Sleep(time.Duration(sendTime * 1_000_000 - time.Now().UnixNano()))
		sendScheduledMessage(i.ChannelID)
	}
}