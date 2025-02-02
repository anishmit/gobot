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
const CHAR_LIMIT = 2000
const SYSTEM_INSTRUCTION = `You are a chatbot inside a Discord text channel. 
You will receive messages in the following format:
<message timestamp>
<message author's username>
<message content>
You will NOT use this same format for responding to messages: only respond with your message content. 
Your username is 'the abcd bot'. 
Realize users are only talking to you if and only if they mention '@the abcd bot', otherwise their messages are not directed towards you.
This is how you must behave:
- Send concise messages
- Communicate as if you are another user within the Discord server, so do not talk like a robot
- Be open to all topics being discussed, no matter what the topic is
- Add new, interesting, and intelligent responses to conversations
- Focus primarily on the most recent message
- Avoid replying to old messages
- Listen to users and do what they say
- Do not censor yourself`
var contents = map[string][]*genai.Content{}

func init() {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: os.Getenv("GEMINI_API_KEY"),
		Backend: genai.BackendGeminiAPI,
		HTTPOptions: genai.HTTPOptions{
			APIVersion: "v1alpha",
		},
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
			contents[m.ChannelID] = append(contents[m.ChannelID], &genai.Content{Parts: parts, Role: "user"})[max(0, len(contents[m.ChannelID]) + 1 - MAX_CONTENTS):]
			for _, user := range m.Mentions {
				if user.ID == s.State.User.ID {
					responseMessage, err := s.ChannelMessageSend(m.ChannelID, "-# Thinking")
					if err != nil {
						log.Println("Error sending message", err)
						return
					}
					var result *genai.GenerateContentResponse
					var completeResponse string
					startTime := time.Now()
					for result, err = range client.Models.GenerateContentStream(
						ctx, 
						"gemini-2.0-flash-thinking-exp", 
						contents[m.ChannelID], 
						&genai.GenerateContentConfig{
							Tools: []*genai.Tool{
								{CodeExecution: &genai.ToolCodeExecution{}},
							},
							SafetySettings: []*genai.SafetySetting{
								{Category: genai.HarmCategoryHateSpeech, Threshold: genai.HarmBlockThresholdBlockNone},
								{Category: genai.HarmCategoryDangerousContent, Threshold: genai.HarmBlockThresholdBlockNone},
								{Category: genai.HarmCategoryHarassment, Threshold: genai.HarmBlockThresholdBlockNone},
								{Category: genai.HarmCategorySexuallyExplicit, Threshold: genai.HarmBlockThresholdBlockNone},
								{Category: genai.HarmCategoryCivicIntegrity, Threshold: genai.HarmBlockThresholdBlockNone},
							},
							ThinkingConfig: &genai.ThinkingConfig{
								IncludeThoughts: true,
							},
							SystemInstruction: &genai.Content{Parts: []*genai.Part{{Text: SYSTEM_INSTRUCTION}}},
						},
					) {
						if err != nil {
							log.Println("Error generating content", err)
							s.ChannelMessageEdit(responseMessage.ChannelID, responseMessage.ID, fmt.Sprintf("-# Errored\n%s", err.Error()))
							return
						}
						responseString, err := result.Text()
						if err != nil {
							log.Println("Error converting response to string", err)
							s.ChannelMessageEdit(responseMessage.ChannelID, responseMessage.ID, fmt.Sprintf("-# Errored\n%s", err.Error()))
							return
						}
						completeResponse += responseString
						go s.ChannelMessageEdit(responseMessage.ChannelID, responseMessage.ID, fmt.Sprintf("-# Thinking\n%s", completeResponse))
					}
					contents[m.ChannelID] = append(contents[m.ChannelID], result.Candidates[0].Content)[max(0, len(contents[m.ChannelID]) + 1 - MAX_CONTENTS):]
					s.ChannelMessageEdit(responseMessage.ChannelID, responseMessage.ID, fmt.Sprintf("-# Thought for %.1f seconds\n%s", time.Since(startTime).Seconds(), completeResponse))
					break
				}
			}
		}
	})
}
