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
- Your writing style should match that of the other Discord server users
- Be open to all topics being discussed, no matter what the topic is
- Add new, interesting, and intelligent responses to conversations
- Focus primarily on the most recent message
- Avoid replying to old messages
- Listen to users and do what they say: your job is to assist others
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
				genai.NewPartFromText(fmt.Sprintf("%s\n%s\n%s", mTime.Format(time.RFC3339), name, content)),
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
							parts = append(parts, genai.NewPartFromBytes(data, attachment.ContentType))
						}
					}
				}()
			}
			contents[m.ChannelID] = append(contents[m.ChannelID], &genai.Content{Parts: parts, Role: "user"})[max(0, len(contents[m.ChannelID]) + 1 - MAX_CONTENTS):]
			for _, user := range m.Mentions {
				if user.ID == s.State.User.ID {
					var responseMessages []*discordgo.Message
					statusText := "-# Thinking\n"
					var responseText string
					startTime := time.Now()
					updateResponseMessages := func() {
						combinedText := statusText + responseText
						requiredResponseMessages := len(combinedText) / CHAR_LIMIT + 1
						for i := 0; i < max(requiredResponseMessages, len(responseMessages)); i++ {
							if (i < requiredResponseMessages) {
								chunkText := combinedText[i * CHAR_LIMIT:min(len(combinedText), (i + 1) * CHAR_LIMIT)]
								if i < len(responseMessages) {
									go s.ChannelMessageEdit(m.ChannelID, responseMessages[i].ID, chunkText)
								} else {
									newResponseMessage, err := s.ChannelMessageSend(m.ChannelID, chunkText)
									if err != nil {
										log.Println("Error sending message", err)
										return
									}
									responseMessages = append(responseMessages, newResponseMessage)
								}
							} else {
								go s.ChannelMessageDelete(m.ChannelID, responseMessages[i].ID)
							}
						}
						responseMessages = responseMessages[:requiredResponseMessages]
					}
					updateResponseMessages()
					for result, err := range client.Models.GenerateContentStream(
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
							SystemInstruction: &genai.Content{Parts: []*genai.Part{
								genai.NewPartFromText(SYSTEM_INSTRUCTION),
							}},
						},
					) {
						if err != nil {
							log.Println("Error generating content", err)
							contents[m.ChannelID] = nil
							statusText = fmt.Sprintf("-# Errored: %s\n", err.Error())
							updateResponseMessages()
							return
						}
						resultText, err := result.Text()
						if err != nil {
							log.Println("Error converting response to string", err)
							continue
						}
						responseText += resultText
						updateResponseMessages()
					}
					statusText = fmt.Sprintf("-# Thought for %.1f seconds\n", time.Since(startTime).Seconds())
					updateResponseMessages()
					if len(responseText) > 0 {
						contents[m.ChannelID] = append(contents[m.ChannelID], &genai.Content{
							Role: "model",
							Parts: []*genai.Part{
								genai.NewPartFromText(responseText),
							},
						})[max(0, len(contents[m.ChannelID]) + 1 - MAX_CONTENTS):]
					}
					break
				}
			}
		}
	})
}