package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
)

// AutoReply represents a single auto-reply rule
type AutoReply struct {
	Trigger  string `json:"trigger"`
	Response string `json:"response"`
	AuthorID string `json:"author_id,omitempty"`
}

// RSS feed structures
type RSS struct {
	XMLName xml.Name `xml:"rss"`
	Channel Channel  `xml:"channel"`
}

type Channel struct {
	Title       string `xml:"title"`
	Description string `xml:"description"`
	Items       []Item `xml:"item"`
}

type Item struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
}

// ServerAutoReplies stores auto-reply rules per server
type ServerAutoReplies map[string][]AutoReply // map[guildID][]AutoReply

const (
	dataFile   = "auto_replies.json"
	embedColor = 0x00ff00
)

var (
	serverAutoReplies ServerAutoReplies
	session           *discordgo.Session
)

// containsWholeWord checks if the trigger exists as a whole word in the message
func containsWholeWord(message, trigger string) bool {
	words := strings.Fields(message)
	for _, word := range words {
		// Remove common punctuation from the word
		cleanWord := strings.Trim(word, ".,!?;:\"'()[]{}*")
		if cleanWord == trigger {
			return true
		}
	}
	return false
}

// RSS topic mapping based on Investing.com RSS structure
var rssTopics = map[string]string{
	"ringkasan pasar":      "https://id.investing.com/rss/news_25.rss",
	"analisis teknikal":    "https://id.investing.com/rss/news_25.rss",
	"analisis fundamental": "https://id.investing.com/rss/news_25.rss",
	"opini":                "https://id.investing.com/rss/news_25.rss",
	"ide investasi":        "https://id.investing.com/rss/news_25.rss",
	"mata uang kripto":     "https://id.investing.com/rss/news_301.rss",
	"forex":                "https://id.investing.com/rss/news_1.rss",
	"saham":                "https://id.investing.com/rss/news_25.rss",
	"komoditas":            "https://id.investing.com/rss/news_49.rss",
	"berita":               "https://id.investing.com/rss/news.rss",
	"breaking news":        "https://id.investing.com/rss/news.rss",
}

// fetchRSSFeed fetches and parses RSS feed from the given URL
func fetchRSSFeed(url string) (*RSS, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch RSS feed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	var rss RSS
	err = xml.Unmarshal(body, &rss)
	if err != nil {
		return nil, fmt.Errorf("failed to parse XML: %v", err)
	}

	return &rss, nil
}

// loadAutoReplies loads auto-reply rules from JSON file
func loadAutoReplies() {
	serverAutoReplies = make(ServerAutoReplies)

	if _, err := os.Stat(dataFile); os.IsNotExist(err) {
		return
	}

	data, err := os.ReadFile(dataFile)
	if err != nil {
		log.Printf("Error reading auto-replies file: %v", err)
		return
	}

	if err := json.Unmarshal(data, &serverAutoReplies); err != nil {
		log.Printf("Error parsing auto-replies file: %v", err)
		return
	}

	totalRules := 0
	for _, replies := range serverAutoReplies {
		totalRules += len(replies)
	}
	log.Printf("Loaded %d auto-reply rules across %d servers", totalRules, len(serverAutoReplies))
}

// saveAutoReplies saves auto-reply rules to JSON file
func saveAutoReplies() {
	data, err := json.MarshalIndent(serverAutoReplies, "", "  ")
	if err != nil {
		log.Printf("Error marshaling auto-replies: %v", err)
		return
	}

	if err := os.WriteFile(dataFile, data, 0644); err != nil {
		log.Printf("Error saving auto-replies: %v", err)
		return
	}
}

// addAutoReply adds a new auto-reply rule for a specific server
func addAutoReply(trigger, response, authorID, guildID string) (bool, string, string) {
	// Initialize server replies if not exists
	if serverAutoReplies[guildID] == nil {
		serverAutoReplies[guildID] = make([]AutoReply, 0)
	}

	// Check if trigger already exists in this server
	for i, reply := range serverAutoReplies[guildID] {
		if strings.EqualFold(reply.Trigger, trigger) {
			// Check if the current user is the author
			if reply.AuthorID != "" && reply.AuthorID != authorID {
				return false, fmt.Sprintf("you can't change this you bartard <@%s>", authorID), ""
			}
			// Update existing reply
			serverAutoReplies[guildID][i].Response = response
			serverAutoReplies[guildID][i].AuthorID = authorID
			saveAutoReplies()
			return true, "Auto-reply updated successfully!", ""
		}
	}

	// Add new auto-reply
	serverAutoReplies[guildID] = append(serverAutoReplies[guildID], AutoReply{
		Trigger:  strings.ToLower(trigger),
		Response: response,
		AuthorID: authorID,
	})
	saveAutoReplies()
	return true, "Auto-reply created successfully!", ""
}

// removeAutoReply removes an auto-reply rule from a specific server
func removeAutoReply(trigger, authorID, guildID string) (bool, string, string) {
	if serverAutoReplies[guildID] == nil {
		return false, "No auto-reply found for that trigger.", ""
	}

	for i, reply := range serverAutoReplies[guildID] {
		if strings.EqualFold(reply.Trigger, trigger) {
			// Check if the current user is the author
			if reply.AuthorID != "" && reply.AuthorID != authorID {
				return false, fmt.Sprintf("you can't change this you bartard <@%s>", authorID), ""
			}

			// Remove the element
			serverAutoReplies[guildID] = append(serverAutoReplies[guildID][:i], serverAutoReplies[guildID][i+1:]...)

			// Clean up empty server entries
			if len(serverAutoReplies[guildID]) == 0 {
				delete(serverAutoReplies, guildID)
			}

			saveAutoReplies()
			return true, "Auto-reply removed successfully!", ""
		}
	}
	return false, "No auto-reply found for that trigger.", ""
}

// handleReplyCommand handles the /reply slash command
func handleReplyCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := i.ApplicationCommandData().Options

	// Get guild ID - only work in servers, not DMs
	guildID := i.GuildID
	if guildID == "" {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "❌ Auto-reply commands only work in servers, not in DMs!",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

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
		success, message, _ := removeAutoReply(trigger, userID, guildID)
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

	success, message, _ := addAutoReply(trigger, response, userID, guildID)

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
	// Get guild ID - only work in servers, not DMs
	guildID := i.GuildID
	if guildID == "" {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "❌ Auto-reply commands only work in servers, not in DMs!",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	// Check if this server has any auto-replies
	serverReplies := serverAutoReplies[guildID]
	if len(serverReplies) == 0 {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "📝 No auto-reply rules set up for this server.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	embed := &discordgo.MessageEmbed{
		Title:       "📋 Server Auto-Reply Rules",
		Description: "Active rules for this server",
		Color:       0x3498db,
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("Total rules: %d", len(serverReplies)),
		},
	}

	for _, reply := range serverReplies {
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
		Description: "Smart auto-reply system for Discord servers",
		Color:       0x9b59b6,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "📝 `/reply [trigger] [response]`",
				Value:  "Set up a new auto-reply rule for this server. When someone sends a message containing the trigger word, the bot will automatically respond.",
				Inline: false,
			},
			{
				Name:   "🗑️ `/reply [trigger] [response] remove`",
				Value:  "Remove an existing auto-reply rule for the specified trigger in this server.",
				Inline: false,
			},
			{
				Name:   "📋 `/list_replies`",
				Value:  "Show all active auto-reply rules for this server.",
				Inline: false,
			},
			{
				Name:   "ℹ️ How it works:",
				Value:  "• Triggers are case-insensitive and match whole words only\n• Bot only works in servers where auto-replies have been set up\n• Anyone can create new rules\n• Only the original author can modify/delete their rules\n• Rules are server-specific",
				Inline: false,
			},
			{
				Name:   "⚠️ Note:",
				Value:  "• Commands only work in servers, not in DMs\n• The bot needs 'Send Messages' permission in channels where you want auto-replies to work\n• Auto-replies only work in servers that have at least one rule set up",
				Inline: false,
			},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Use /reply to set up smart auto-replies for this server! Only you can modify rules you create.",
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

// handleCommandsCommand handles the /commands slash command
func handleCommandsCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	embed := &discordgo.MessageEmbed{
		Title:       "🎛️ Bot Commands",
		Description: "All available commands for this bot",
		Color:       0x3498db,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "🤖 **Auto-Reply Commands**",
				Value:  "`/reply` - Set up auto-reply rules\n`/list_replies` - Show server's auto-reply rules\n`/help_reply` - Help for auto-reply system",
				Inline: false,
			},
			{
				Name:   "📰 **News & Analysis Commands**",
				Value:  "`/analisis` - Get latest financial news from Investing.com",
				Inline: false,
			},
			{
				Name:   "ℹ️ **Information Commands**",
				Value:  "`/commands` - Show this list of all commands",
				Inline: false,
			},
			{
				Name:   "📖 **Quick Usage Examples:**",
				Value:  "• `/reply kerja working hard!` - Create auto-reply\n• `/analisis ringkasan pasar` - Get market news\n• `/list_replies` - See all server replies\n• `/help_reply` - Detailed auto-reply help",
				Inline: false,
			},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: "💡 Tip: Use /help_reply for detailed auto-reply instructions",
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
			Flags:  discordgo.MessageFlagsEphemeral,
		},
	})
}

// handleAnalisisCommand handles the /analisis slash command for RSS feeds
func handleAnalisisCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := i.ApplicationCommandData().Options

	if len(options) == 0 {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "❌ Please provide a topic! Example: `/analisis ringkasan pasar`",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	topic := strings.ToLower(options[0].StringValue())

	// Find matching RSS URL
	var rssURL string
	var foundTopic string
	for key, url := range rssTopics {
		if strings.Contains(topic, key) || key == topic {
			rssURL = url
			foundTopic = key
			break
		}
	}

	if rssURL == "" {
		// Show available topics
		availableTopics := make([]string, 0, len(rssTopics))
		for topic := range rssTopics {
			availableTopics = append(availableTopics, topic)
		}

		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("❌ Topic not found! Available topics:\n• %s", strings.Join(availableTopics, "\n• ")),
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	// Defer the response since fetching RSS might take time
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	// Fetch RSS feed
	rss, err := fetchRSSFeed(rssURL)
	if err != nil {
		s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: fmt.Sprintf("❌ Failed to fetch RSS feed: %v", err),
			Flags:   discordgo.MessageFlagsEphemeral,
		})
		return
	}

	if len(rss.Channel.Items) == 0 {
		s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "📰 No news articles found for this topic.",
			Flags:   discordgo.MessageFlagsEphemeral,
		})
		return
	}

	// Create embed with latest news (limit to 5 articles)
	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("📰 %s - %s", strings.ToUpper(string(foundTopic[0]))+foundTopic[1:], rss.Channel.Title),
		Description: "Latest news from Investing.com",
		Color:       0x1f8b4c,
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Source: Investing.com",
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	maxItems := 5
	if len(rss.Channel.Items) < maxItems {
		maxItems = len(rss.Channel.Items)
	}

	for i := 0; i < maxItems; i++ {
		item := rss.Channel.Items[i]

		// Clean up description (remove HTML tags and limit length)
		description := strings.ReplaceAll(item.Description, "<![CDATA[", "")
		description = strings.ReplaceAll(description, "]]>", "")
		description = strings.ReplaceAll(description, "<p>", "")
		description = strings.ReplaceAll(description, "</p>", "")
		description = strings.ReplaceAll(description, "<br>", "\n")
		description = strings.ReplaceAll(description, "<br/>", "\n")

		if len(description) > 200 {
			description = description[:200] + "..."
		}

		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   item.Title,
			Value:  fmt.Sprintf("%s\n\n[Read More](%s)", description, item.Link),
			Inline: false,
		})
	}

	s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Embeds: []*discordgo.MessageEmbed{embed},
	})
}

// messageCreate handles incoming messages for auto-replies
func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore bot messages
	if m.Author.Bot {
		return
	}

	// Only work in servers, not DMs
	if m.GuildID == "" {
		return
	}

	// Check if this server has any auto-replies set up
	serverReplies := serverAutoReplies[m.GuildID]
	if len(serverReplies) == 0 {
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

	// Check for matching triggers - search for whole word matches only
	for _, reply := range serverReplies {
		if containsWholeWord(messageContent, reply.Trigger) {
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
	case "analisis":
		handleAnalisisCommand(s, i)
	case "commands":
		handleCommandsCommand(s, i)
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
		{
			Name:        "analisis",
			Description: "Fetch latest news and analysis from Investing.com",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "topic",
					Description: "Topic to get news for",
					Required:    true,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{Name: "Ringkasan Pasar", Value: "ringkasan pasar"},
						{Name: "Analisis Teknikal", Value: "analisis teknikal"},
						{Name: "Analisis Fundamental", Value: "analisis fundamental"},
						{Name: "Opini", Value: "opini"},
						{Name: "Ide Investasi", Value: "ide investasi"},
						{Name: "Mata Uang Kripto", Value: "mata uang kripto"},
						{Name: "Forex", Value: "forex"},
						{Name: "Saham", Value: "saham"},
						{Name: "Komoditas", Value: "komoditas"},
						{Name: "Berita", Value: "berita"},
						{Name: "Breaking News", Value: "breaking news"},
					},
				},
			},
		},
		{
			Name:        "commands",
			Description: "Show all available bot commands",
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
