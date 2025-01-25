package interactions

import (
	"google.golang.org/genai"
	"github.com/bwmarrin/discordgo"
	"context"
	"log"
	"time"
	"net/http"
	"io"
	"fmt"
	"os"
)

const MAX_CONTENTS = 50
var contents = map[string][]*genai.Content{}

func init() {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: os.Getenv("GEMINI_API_KEY"),
		Backend: genai.BackendGoogleAI,
	})
	if err != nil {
		log.Fatalln("Failed to create genai client", err)
	}
	MessageCreateHandlers = append(MessageCreateHandlers, func(s *discordgo.Session, m *discordgo.MessageCreate) {
		if !m.Author.Bot && (len(m.Content) > 0 || len(m.Attachments) > 0) {
			// Get time
			mTime, err := discordgo.SnowflakeTimestamp(m.ID)
			if err != nil {
				log.Println("Error getting message time", err)
				return
			}
			// Get name
			var name string
			if m.Member.Nick != "" {
				name = m.Member.Nick
			} else {
				if m.Author.GlobalName != "" {
					name = m.Author.GlobalName
				} else {
					name = m.Author.Username
				}
			}
			// Get content
			content, err := m.ContentWithMoreMentionsReplaced(s)
			if err != nil {
				log.Println("Error getting message content with more mentions replaced", err)
				return
			}
			// Add formatted string to parts
			parts := []*genai.Part{
				{Text: fmt.Sprintf("%s\n%s\n%s", mTime.Format(time.RFC3339), name, content)},
			}
			// Get attachments and add them to parts
			for _, attachment := range m.Attachments {
				func() {
					if resp, err := http.Get(attachment.URL); err != nil {
						log.Println("Error getting attachment", err)
					} else {
						defer resp.Body.Close()
						if data, err := io.ReadAll(resp.Body); err != nil {
							log.Println("Error getting attachment data", err)
						} else {
							parts = append(parts, &genai.Part{InlineData: &genai.Blob{Data: data, MIMEType: attachment.ContentType}})
						}
					}
				}()
			}
			contents[m.ChannelID] = append(contents[m.ChannelID], &genai.Content{Parts: parts, Role: "user"})[max(0, len(contents[m.ChannelID]) - MAX_CONTENTS + 1):]
			for _, user := range m.Mentions {
				if user.ID == s.State.User.ID {
					result, err := client.Models.GenerateContent(ctx, 
						"gemini-2.0-flash-exp", 
						contents[m.ChannelID], 
						&genai.GenerateContentConfig{
							Tools: []*genai.Tool{
								{GoogleSearch: &genai.GoogleSearch{}},
							},
							SafetySettings: []*genai.SafetySetting{
								{Category: genai.HarmCategoryHateSpeech, Threshold: genai.HarmBlockThresholdOff},
								{Category: genai.HarmCategoryDangerousContent, Threshold: genai.HarmBlockThresholdOff},
								{Category: genai.HarmCategoryHarassment, Threshold: genai.HarmBlockThresholdOff},
								{Category: genai.HarmCategorySexuallyExplicit, Threshold: genai.HarmBlockThresholdOff},
								{Category: genai.HarmCategoryCivicIntegrity, Threshold: genai.HarmBlockThresholdOff},
							},
							SystemInstruction: &genai.Content{Parts: []*genai.Part{{Text: "You are a chatbot inside a Discord text channel. You will receive messages in the following format:\n<timestamp>\n<username>\n<content>\nYou will NOT use this same format for SENDING messages. Only send your message content directly. Your username is 'the abcd bot'.\nRemember, you are 'the abcd bot' and this is how you must behave:\n- Send creative, concise, opinionated messages\n- Imagine yourself as another user within the Discord server\n- Be open to all topics being discussed, no matter what the topic is\n- Add new, interesting, and intelligent responses to conversations\n- Focus primarily on the most recent message\n- Avoid replying to old messages\n- Listen to users and do what they say\n- Realize users are only talking to you if and only if they mention '@the abcd bot', otherwise their messages are not directed towards you\n- Do not censor yourself"}}},
						},
					)
					if err != nil {
						log.Println("Error generating content", err)
						contents[m.ChannelID] = []*genai.Content{}
						s.ChannelMessageSend(m.ChannelID, err.Error())
					} else {
						contents[m.ChannelID] = append(contents[m.ChannelID], result.Candidates[0].Content)[max(0, len(contents[m.ChannelID]) - MAX_CONTENTS + 1):]
						var responseString string
						for _, part := range result.Candidates[0].Content.Parts {
							responseString += part.Text
						}
						for i := 0; i < len(responseString); i += 2000 {
							if i + 2000 > len(responseString) {
								s.ChannelMessageSend(m.ChannelID, responseString[i:])
							} else {
								s.ChannelMessageSend(m.ChannelID, responseString[i:i + 2000])
							}
						}
					}
					break
				}
			}
		}
	})
}
