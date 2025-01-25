package interactions

import (
	"context"
	"log"
	"firebase.google.com/go/v4/db"
	"github.com/bwmarrin/discordgo"
	"time"
	"sort"
	"fmt"
	"github.com/anishmit/gobot/firebase"
)

type TimePeriod struct {
	Name string
	Days int
}	
type FirstMessage struct {
	Content string `json:"content"`
	Date int64 `json:"date"`
	MsgID string `json:"msgId"`
	UserID string `json:"userId"`
}
type FirstMessageWithTime struct {
	Time int64
	Date string
	MsgID string
	UserId string
}

const SERVER_ID = "407302806241017866"
const CHANNEL_ID = "407302806241017868"
var TIME_PERIODS = [5]TimePeriod{
	{Name: "Today", Days: 1},
	{Name: "Past Week", Days: 7},
	{Name: "Past Month", Days: 30},
	{Name: "Past Year", Days: 365},
	{Name: "All Time", Days: 1e9},
}
var channelCreatedTime time.Time;

func init() {
	location, err := time.LoadLocation("America/Detroit")
	if err != nil {
		log.Fatalln("Error loading location", err)
	}

	ctx := context.Background()
	client, err := firebase.App.Database(ctx)
	if err != nil {
		log.Fatalln("Error initializing database client", err)
	}
	dbRef := client.NewRef("firstMessages")

	Commands = append(Commands, &discordgo.ApplicationCommand{
		Name:        "first",
		Description: "Data about first messages",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Name: "count",
				Description: "Leaderboard for number of first messages",
				Type: discordgo.ApplicationCommandOptionSubCommand,
			},
			{
				Name: "time",
				Description: "Leaderboard for fastest first messages",
				Type: discordgo.ApplicationCommandOptionSubCommand,
			},
		},
	})

	CommandHandlers["first"] = func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		})

		var data map[string]FirstMessage
		if err := dbRef.Get(ctx, &data); err != nil {
			log.Println("Error reading from database", err)
			return
		}

		options := i.ApplicationCommandData().Options

		switch options[0].Name {
		case "count":
			curTime, err := discordgo.SnowflakeTimestamp(i.Interaction.ID)
			if err != nil {
				log.Println("Error getting interaction time", err)
				return
			}
			curTime = curTime.In(location)

			daysSubtracted := 0
			var timePeriodsData [len(TIME_PERIODS)]map[string]int
			for i := range timePeriodsData {
				timePeriodsData[i] = make(map[string]int)
			}
			for curTime.Year() != channelCreatedTime.Year() || curTime.YearDay() != channelCreatedTime.YearDay() {
				if value, ok := data[curTime.Format(time.DateOnly)]; ok {
					for i, timePeriod := range TIME_PERIODS {
						if timePeriod.Days > daysSubtracted {
							timePeriodsData[i][value.UserID]++
						}
					}
				}
				curTime = curTime.AddDate(0, 0, -1)
				daysSubtracted++;
			}

			fields := make([]*discordgo.MessageEmbedField, 0, len(timePeriodsData))
			for i, timePeriodData := range timePeriodsData {
				userIds := make([]string, 0, len(timePeriodData))
				for userId := range timePeriodData {
					userIds = append(userIds, userId)
				}
				sort.Slice(userIds, func(i, j int) bool { return timePeriodData[userIds[i]] > timePeriodData[userIds[j]] })
				var fieldValue string
				for _, userId := range userIds {
					fieldValue += fmt.Sprintf("<@%s>: %d\n", userId, timePeriodData[userId])
				}
				fields = append(fields, &discordgo.MessageEmbedField{
					Name: TIME_PERIODS[i].Name,
					Value: fieldValue,
				})
			}
			
			s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
				Embeds: []*discordgo.MessageEmbed{
					{
						Title: "First Leaderboard (Count)",
						Color: 0xff4d01,
						Fields: fields,
					},
				},
			})
		case "time":
			firstMessages := make([]FirstMessageWithTime, 0, len(data))
			for dateStr, firstMessage := range data {
				if t, err := time.ParseInLocation(time.DateOnly, dateStr, location); err != nil {
					log.Println("Error parsing location", err)
				} else {
					firstMessages = append(firstMessages, FirstMessageWithTime{
						Time: firstMessage.Date - t.UnixMilli(),
						Date: dateStr,
						MsgID: firstMessage.MsgID,
						UserId: firstMessage.UserID,
					})
				}
			}
			sort.Slice(firstMessages, func(i, j int) bool { return firstMessages[i].Time < firstMessages[j].Time })
			var description string
			for i := 0; i < min(15, len(firstMessages)); i++ {
				description += fmt.Sprintf(
					"%d. <@%s>: **%d** ms on [%s](https://discord.com/channels/%s/%s/%s)\n", 
					i + 1, 
					firstMessages[i].UserId, 
					firstMessages[i].Time, 
					firstMessages[i].Date,
					SERVER_ID,
					CHANNEL_ID,
					firstMessages[i].MsgID,
				)
			}
			s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
				Embeds: []*discordgo.MessageEmbed{
					{
						Title: "First Leaderboard (Time)",
						Color: 0xff4d01,
						Description: description,
					},
				},
			})
		}
		
		
	}

	MessageCreateHandlers = append(MessageCreateHandlers, func(s *discordgo.Session, m *discordgo.MessageCreate) {
		if m.ChannelID == CHANNEL_ID {
			curTime, err := discordgo.SnowflakeTimestamp(m.ID)
			if err != nil {
				log.Println("Error getting message time", err)
				return
			}
			curTime = curTime.In(location)
			dbRef.Child(curTime.Format(time.DateOnly)).Transaction(ctx, func(value db.TransactionNode) (interface{}, error) {
				var firstMessage FirstMessage
				value.Unmarshal(&firstMessage)
				if firstMessage.MsgID == "" || firstMessage.Date > curTime.UnixMilli() {
					return FirstMessage{
						Content: m.Content,
						Date: curTime.UnixMilli(),
						MsgID: m.ID,
						UserID: m.Author.ID,
					}, nil
				} else {
					return firstMessage, nil
				}
			})
		}
	})

	ReadyHandlers = append(ReadyHandlers, func(s *discordgo.Session, r *discordgo.Ready) {
		channel, err := s.Channel(CHANNEL_ID); 
		if (err != nil) {
			log.Fatalln("Error getting channel", err)
		}
		channelCreatedTime, err = discordgo.SnowflakeTimestamp(channel.ID)
		if err != nil {
			log.Fatalln("Error getting channel created time", err)
		}
		channelCreatedTime = channelCreatedTime.In(location)
	})
}