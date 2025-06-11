package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/bwmarrin/discordgo"
)

// AutoReply represents a single auto-reply rule
type AutoReply struct {
	Trigger  string `json:"trigger"`
	Response string `json:"response"`
	AuthorID string `json:"author_id,omitempty"`
}

// AutoReplies stores all auto-reply rules globally (no longer per-channel)
type AutoReplies []AutoReply

const (
	dataFile   = "auto_replies.json"
	embedColor = 0x00ff00
)

var (
	autoReplies AutoReplies
	session     *discordgo.Session
)

// loadAutoReplies loads auto-reply rules from JSON file
func loadAutoReplies() {
	autoReplies = make(AutoReplies, 0)

	if _, err := os.Stat(dataFile); os.IsNotExist(err) {
		return
	}

	data, err := os.ReadFile(dataFile)
	if err != nil {
		log.Printf("Error reading auto-replies file: %v", err)
		return
	}

	if err := json.Unmarshal(data, &autoReplies); err != nil {
		log.Printf("Error parsing auto-replies file: %v", err)
		return
	}

	log.Printf("Loaded %d global auto-reply rules", len(autoReplies))
}

// saveAutoReplies saves auto-reply rules to JSON file
func saveAutoReplies() {
	data, err := json.MarshalIndent(autoReplies, "", "  ")
	if err != nil {
		log.Printf("Error marshaling auto-replies: %v", err)
		return
	}

	if err := os.WriteFile(dataFile, data, 0644); err != nil {
		log.Printf("Error saving auto-replies: %v", err)
		return
	}
}

// addAutoReply adds a new auto-reply rule
func addAutoReply(trigger, response, authorID string) (bool, string, string) {
	// Check if trigger already exists globally
	for i, reply := range autoReplies {
		if strings.EqualFold(reply.Trigger, trigger) {
			// Check if the current user is the author
			if reply.AuthorID != "" && reply.AuthorID != authorID {
				return false, fmt.Sprintf("you can't change this you bartard <@%s>", authorID), ""
			}
			// Update existing reply
			autoReplies[i].Response = response
			autoReplies[i].AuthorID = authorID
			saveAutoReplies()
			return true, "Auto-reply updated successfully!", ""
		}
	}

	// Add new auto-reply
	autoReplies = append(autoReplies, AutoReply{
		Trigger:  strings.ToLower(trigger),
		Response: response,
		AuthorID: authorID,
	})
	saveAutoReplies()
	return true, "Auto-reply created successfully!", ""
}

// removeAutoReply removes an auto-reply rule
func removeAutoReply(trigger, authorID string) (bool, string, string) {
	for i, reply := range autoReplies {
		if strings.EqualFold(reply.Trigger, trigger) {
			// Check if the current user is the author
			if reply.AuthorID != "" && reply.AuthorID != authorID {
				return false, fmt.Sprintf("you can't change this you bartard <@%s>", authorID), ""
			}

			// Remove the element
			autoReplies = append(autoReplies[:i], autoReplies[i+1:]...)
			saveAutoReplies()
			return true, "Auto-reply removed successfully!", ""
		}
	}
	return false, "No auto-reply found for that trigger.", ""
}

// handleReplyCommand handles the /reply slash command
func handleReplyCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := i.ApplicationCommandData().Options

	// Get user ID - handle both guild and DM interactions
	var userID string
	if i.Member != nil {
		userID = i.Member.User.ID
	} else if i.User != nil {
		userID = i.User.ID
	}

	trigger := options[0].StringValue()

	var response string
	var mode string = "add"

	if len(options) > 1 {
		response = options[1].StringValue()
	}
	if len(options) > 2 {
		mode = options[2].StringValue()
	}

	if strings.ToLower(mode) == "remove" {
		success, message, _ := removeAutoReply(trigger, userID)
		var responseType string
		var flags discordgo.MessageFlags = discordgo.MessageFlagsEphemeral

		if success {
			responseType = "✅ " + message
		} else {
			responseType = message
			// If it's the custom bartard message, make it public
			if strings.Contains(message, "bartard") {
				flags = 0
			} else {
				responseType = "❌ " + message
			}
		}

		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: responseType,
				Flags:   flags,
			},
		})
		return
	}

	if response == "" {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "❌ Please provide a response message!",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	success, message, _ := addAutoReply(trigger, response, userID)

	if !success {
		var flags discordgo.MessageFlags = discordgo.MessageFlagsEphemeral
		var responseContent string = "❌ " + message

		// If it's the custom bartard message, make it public
		if strings.Contains(message, "bartard") {
			flags = 0
			responseContent = message
		}

		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: responseContent,
				Flags:   flags,
			},
		})
		return
	}

	embed := &discordgo.MessageEmbed{
		Title:       "✅ Auto-Reply Set Up Successfully!",
		Description: fmt.Sprintf("**Trigger:** %s\n**Response:** %s", trigger, response),
		Color:       embedColor,
		Footer: &discordgo.MessageEmbedFooter{
			Text: "The bot will now automatically reply when someone sends the trigger message. Only you can modify this auto-reply.",
		},
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
			Flags:  discordgo.MessageFlagsEphemeral,
		},
	})
}

// handleListRepliesCommand handles the /list_replies slash command
func handleListRepliesCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if len(autoReplies) == 0 {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "📝 No auto-reply rules set up globally.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	embed := &discordgo.MessageEmbed{
		Title:       "📋 Global Auto-Reply Rules",
		Description: "Active rules for all channels",
		Color:       0x3498db,
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("Total rules: %d", len(autoReplies)),
		},
	}

	for _, reply := range autoReplies {
		displayResponse := reply.Response
		if len(displayResponse) > 100 {
			displayResponse = displayResponse[:100] + "..."
		}

		authorInfo := ""
		if reply.AuthorID != "" {
			authorInfo = fmt.Sprintf(" (by <@%s>)", reply.AuthorID)
		}

		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   fmt.Sprintf("Trigger: %s", reply.Trigger),
			Value:  fmt.Sprintf("Response: %s%s", displayResponse, authorInfo),
			Inline: false,
		})
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
			Flags:  discordgo.MessageFlagsEphemeral,
		},
	})
}

// handleHelpCommand handles the /help_reply slash command
func handleHelpCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	embed := &discordgo.MessageEmbed{
		Title:       "🤖 Auto-Reply Bot Help",
		Description: "Smart auto-reply system for Discord channels",
		Color:       0x9b59b6,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "📝 `/reply [trigger] [response]`",
				Value:  "Set up a new auto-reply rule. When someone sends a message containing the trigger text, the bot will automatically respond.",
				Inline: false,
			},
			{
				Name:   "🗑️ `/reply [trigger] [response] remove`",
				Value:  "Remove an existing auto-reply rule for the specified trigger.",
				Inline: false,
			},
			{
				Name:   "📋 `/list_replies`",
				Value:  "Show all active global auto-reply rules.",
				Inline: false,
			},
			{
				Name:   "ℹ️ How it works:",
				Value:  "• Triggers are case-insensitive\n• Bot checks if trigger text is contained in messages\n• Anyone can create new rules\n• Only the original author can modify/delete their rules\n• Rules work globally across all channels the bot can access",
				Inline: false,
			},
			{
				Name:   "⚠️ Note:",
				Value:  "The bot needs 'Send Messages' permission in channels where you want auto-replies to work.",
				Inline: false,
			},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Use /reply to set up smart auto-replies! Only you can modify rules you create.",
		},
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
			Flags:  discordgo.MessageFlagsEphemeral,
		},
	})
}

// messageCreate handles incoming messages for auto-replies
func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore bot messages
	if m.Author.Bot {
		return
	}

	if len(autoReplies) == 0 {
		return
	}

	//reply to the message_id

	// Note: If MESSAGE_CONTENT_INTENT is not enabled, m.Content will be empty
	// for messages from users who are not the bot owner
	messageContent := strings.ToLower(strings.TrimSpace(m.Content))

	// If content is empty due to missing intent, skip auto-reply
	if messageContent == "" {
		return
	}

	// Check for matching triggers
	for _, reply := range autoReplies {
		if strings.Contains(messageContent, reply.Trigger) {
			// Send reply immediately with message reference to show "replying to" context
			_, err := s.ChannelMessageSendReply(m.ChannelID, reply.Response, &discordgo.MessageReference{
				MessageID: m.ID,
				ChannelID: m.ChannelID,
				GuildID:   m.GuildID,
			})
			if err != nil {
				log.Printf("Error sending auto-reply: %v", err)
				// Fallback to regular message if reply fails
				s.ChannelMessageSend(m.ChannelID, reply.Response)
			}
			break // Only respond to the first matching trigger
		}
	}
}

// interactionCreate handles slash command interactions
func interactionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}

	switch i.ApplicationCommandData().Name {
	case "reply":
		handleReplyCommand(s, i)
	case "list_replies":
		handleListRepliesCommand(s, i)
	case "help_reply":
		handleHelpCommand(s, i)
	}
}

// ready handles the ready event
func ready(s *discordgo.Session, event *discordgo.Ready) {
	log.Printf("Bot is ready! Logged in as: %v#%v", s.State.User.Username, s.State.User.Discriminator)
	log.Printf("Bot is in %d servers", len(event.Guilds))

	// Register slash commands
	commands := []*discordgo.ApplicationCommand{
		{
			Name:        "reply",
			Description: "Set up auto-reply for specific messages",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "trigger",
					Description: "The message that will trigger the reply",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "response",
					Description: "The response message to send",
					Required:    false,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "mode",
					Description: "Choose 'add' to create new rule or 'remove' to delete existing rule",
					Required:    false,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{
							Name:  "add",
							Value: "add",
						},
						{
							Name:  "remove",
							Value: "remove",
						},
					},
				},
			},
		},
		{
			Name:        "list_replies",
			Description: "List all global auto-reply rules",
		},
		{
			Name:        "help_reply",
			Description: "Show help information for the auto-reply bot",
		},
	}

	for _, cmd := range commands {
		_, err := s.ApplicationCommandCreate(s.State.User.ID, "", cmd)
		if err != nil {
			log.Printf("Cannot create command %v: %v", cmd.Name, err)
		}
	}

	log.Printf("Registered %d slash commands", len(commands))
}

func main() {
	// Get bot token from environment variable
	token := os.Getenv("DISCORD_BOT_TOKEN")
	if token == "" {
		log.Fatal("Please set DISCORD_BOT_TOKEN environment variable")
	}

	// Load existing auto-replies
	loadAutoReplies()

	// Create Discord session
	var err error
	session, err = discordgo.New("Bot " + token)
	if err != nil {
		log.Fatal("Error creating Discord session: ", err)
	}

	// Set up event handlers
	session.AddHandler(ready)
	session.AddHandler(messageCreate)
	session.AddHandler(interactionCreate)

	// Set intents (only use privileged intents if enabled in Discord Developer Portal)
	session.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentMessageContent

	// Open connection
	err = session.Open()
	if err != nil {
		log.Fatal("Error opening connection: ", err)
	}
	defer session.Close()

	// Wait for interrupt signal
	log.Println("Bot is running. Press CTRL+C to exit.")
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-c

	log.Println("Bot shutting down...")
}
