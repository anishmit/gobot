package interactions

import (
	"log"
	"os/exec"
	"github.com/bwmarrin/discordgo"
	"mccoy.space/g/ogg"
)

func init() {
	Commands = append(Commands, &discordgo.ApplicationCommand{
		Name:        "yt",
		Description: "Play YouTube video",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type: discordgo.ApplicationCommandOptionString,
				Name: "term",
				Description: "Search term",
				Required: true,
			},
		},
	})
	CommandHandlers["yt"] = func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		var err error
		voice, err := s.ChannelVoiceJoin("1219548619129225226", "1219548619129225230", false, false)
		if err != nil {
			log.Println("Could not join voice channel", err)
			return
		}
		cmd1 := exec.Command("./yt-dlp", "-o", "-", "https://www.youtube.com/watch?v=k8JflBNovLE&pp=ygUUcG93ZXIga2FueWUgZXhwbGNpaXQ%3D", "-f", "bestaudio")
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
		decoder := ogg.NewDecoder(pipe)
		voice.Speaking(true)
		var lastPacket []byte
		for {
			page, err := decoder.Decode()
			if err != nil {
				log.Println("Could not decode", err)
				break
			}
			continuation := page.Type & ogg.COP != 0
			
			if len(page.Packets) == 1 && continuation {
				lastPacket = append(lastPacket, page.Packets[0]...)
				continue
			} else if continuation {
				voice.OpusSend <- append(lastPacket, page.Packets[0]...)
			} else {
				if lastPacket != nil {
					voice.OpusSend <- lastPacket
				}
				if len(page.Packets) == 1 {
					lastPacket = page.Packets[0]
					continue
				} else {
					voice.OpusSend <- page.Packets[0]
				}
			}
			
			for i := 1; i < len(page.Packets) - 1; i++ {
				voice.OpusSend <- page.Packets[i]
			}

			lastPacket = page.Packets[len(page.Packets) - 1]
		}
		voice.OpusSend <- lastPacket
		voice.Speaking(false)
		cmd2.Wait()
		cmd1.Wait()
	}
}