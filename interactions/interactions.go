package interactions

import (
	"github.com/bwmarrin/discordgo"
)

var Commands []*discordgo.ApplicationCommand
var CommandHandlers = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){}
var MessageCreateHandlers []func(s *discordgo.Session, m *discordgo.MessageCreate)
var ReadyHandlers []func(s *discordgo.Session, r *discordgo.Ready)