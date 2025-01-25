package main

import (
	"os"
	"os/signal"
	"github.com/bwmarrin/discordgo"
	"log"
	"github.com/anishmit/gobot/interactions"
	_ "github.com/joho/godotenv/autoload"
)

var s *discordgo.Session

func init() {
	var err error
	s, err = discordgo.New("Bot " + os.Getenv("BOT_TOKEN"))
	if err != nil {
		log.Fatalf("Invalid bot parameters: %v", err)
	}
}

func main() {
	s.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if h, ok := interactions.CommandHandlers[i.ApplicationCommandData().Name]; ok {
			h(s, i)
		}
	})
	s.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		for _, handler := range interactions.MessageCreateHandlers {
			handler(s, m)
		}
	})
	s.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		log.Printf("Logged in as: %v#%v", s.State.User.Username, s.State.User.Discriminator)
		for _, handler := range interactions.ReadyHandlers {
			handler(s, r)
		}
	})

	err := s.Open()
	if err != nil {
		log.Fatalf("Cannot open the session: %v", err)
	}

	log.Println("Adding commands...")
	registeredCommands := make([]*discordgo.ApplicationCommand, len(interactions.Commands))
	for i, v := range interactions.Commands {
		cmd, err := s.ApplicationCommandCreate(s.State.User.ID, "", v)
		if err != nil {
			log.Panicf("Cannot create '%v' command: %v", v.Name, err)
		}
		registeredCommands[i] = cmd
	}

	defer s.Close()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	log.Println("Press Ctrl+C to exit")
	<-stop

	log.Println("Gracefully shutting down.")
}
