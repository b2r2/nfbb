package bot

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"net"
	"net/http"

	"golang.org/x/net/proxy"
	"gopkg.in/telegram-bot-api.v4"
)

func Run(debug, updatesMode bool) {
	config := Configuration{}
	if err := config.Load("config.json"); err != nil {
		log.Fatal(err)
	}
	tb := &TelegramBot{}
	if err := tb.init(config, debug); err != nil {
		log.Fatal(err)
	}
	log.Printf("Authorized on account %s\n", tb.BotAPI.Self.UserName)
	log.Printf("Debug mode: %v\n", tb.BotAPI.Debug)
	log.Printf("Update mode: %v\n", updatesMode)
	if updatesMode {
		if err := tb.setWebhook(config.Webhook.Host, config.Webhook.Port, config.Webhook.Cert, config.Webhook.Priv); err != nil {
			log.Fatal(err)
		}
	} else {
		if err := tb.getUpdates(); err != nil {
			log.Fatal(err)
		}
	}
	tb.start(config)
}

type Configuration struct {
	Description struct {
		Start    string `json:"start"`
		About    string `json:"about"`
		Feedback string `json:"feedback"`
		Ad       string `json:"ad"`
		Suggest  string `json:"suggest"`
	} `json:"description"`
	Webhook struct {
		Host   string `json:"host"`
		Port   string `json:"port"`
		Listen string `json:"listen"`
		Cert   string `json:"certificate"`
		Priv   string `json:"private_ssl_key"`
	} `json:"webhook"`
	Links struct {
		Ad      string `json:"ads"`
		WebSite string `json:"web"`
		Medium  string `json:"medium"`
		VK      string `json:"vk"`
	} `json:links`
	Proxy struct {
		URL      string `json:"url"`
		Login    string `json:"login"`
		Port     string `json:"port"`
		Password string `json:"password"`
	} `json:"proxy"`
	Token   string `json:"token"`
	GroupID int64  `json:"group_id"`
}

func (c *Configuration) Load(file string) error {
	configFile, _ := ioutil.ReadFile(file)
	err := json.Unmarshal(configFile, c)
	return err
}

type TelegramBot struct {
	BotAPI  *tgbotapi.BotAPI
	Updates tgbotapi.UpdatesChannel
}

func (tb TelegramBot) setProxy(config Configuration) (*http.Client, error) {
	dialer, err := proxy.SOCKS5("tcp", config.Proxy.URL+":"+config.Proxy.Port,
		&proxy.Auth{
			User:     config.Proxy.Login,
			Password: config.Proxy.Password,
		},
		proxy.Direct)
	if err != nil {
		return nil, err
	}
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.Dial(network, addr)
			},
		},
	}
	return client, nil

}

func (tb *TelegramBot) init(config Configuration, debug bool) error {
	//bot, err := tgbotapi.NewBotAPI(token)
	client, err := tb.setProxy(config)
	if err != nil {
		log.Panic(err)
	}
	bot, err := tgbotapi.NewBotAPIWithClient(config.Token, client)
	if err != nil {
		return err
	}
	tb.BotAPI = bot
	tb.BotAPI.Debug = debug
	return nil
}

func (tb *TelegramBot) getUpdates() error {
	if _, err := tb.BotAPI.RemoveWebhook(); err != nil {
		return err
	}
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := tb.BotAPI.GetUpdatesChan(u)
	if err != nil {
		return err
	}
	tb.Updates = updates
	log.Println("Webhook was deactivated")
	return nil
}

func (tb *TelegramBot) setWebhook(host, port, cert, priv string) error {
	//doesn't work
	url := host + ":" + port + "/" + tb.BotAPI.Token
	_, err := tb.BotAPI.SetWebhook(tgbotapi.NewWebhookWithCert(url, cert))
	if err != nil {
		return err
	}
	info, err := tb.BotAPI.GetWebhookInfo()
	if err != nil {
		return err
	}
	if info.LastErrorDate != 0 {
		log.Print(info)
		return errors.New("Telegram callback failed " + info.LastErrorMessage)
	}
	updates := tb.BotAPI.ListenForWebhook("/" + tb.BotAPI.Token)
	go http.ListenAndServeTLS("0.0.0.0:"+port, cert, priv, nil)
	tb.Updates = updates
	log.Println("Webhook was activated")
	log.Printf("%+v\n", info)
	return nil
}

func (tb TelegramBot) pushDescription(desc string, chatID int64, config Configuration) {
	preparing := tgbotapi.NewMessage(chatID, desc)
	preparing.DisableWebPagePreview = true
	preparing.ParseMode = "markdown"
	switch desc {
	case "/ad", "Реклама", "Условия рекламы":
		preparing.ReplyMarkup = getInlineMarkup(
			[]string{"Перейти"},
			[]string{config.Links.Ad},
		)
	case "/about", "О проекте", "О канале":
		preparing.ReplyMarkup = getInlineMarkup(
			[]string{
				"Перейти на сайт",
				"Подписаться medium",
				"Подписаться VK",
			},
			[]string{
				config.Links.WebSite,
				config.Links.Medium,
				config.Links.VK,
			},
		)
	}
	tb.BotAPI.Send(preparing)
}

func (tb TelegramBot) pushForward(groupID int64, fromID, messageID int) {
	preparing := tgbotapi.NewForward(groupID, int64(fromID), messageID)
	tb.BotAPI.Send(preparing)
}

func (tb TelegramBot) handleUpdates(update tgbotapi.Update, config Configuration) {
	chat := update.Message.Chat
	msg := update.Message
	if desc := getDescription(msg.Text, config); len(desc) > 0 {
		tb.pushDescription(desc, chat.ID, config)
		return
	}
	if chat.IsPrivate() {
		tb.pushForward(config.GroupID, msg.From.ID, msg.MessageID)
		return
	}
	bot := tb.BotAPI
	switch {
	case msg.ReplyToMessage == nil:
		preparing := tgbotapi.NewMessage(chat.ID, "Select reply message")
		bot.Send(preparing)
		return
	case msg.ReplyToMessage.ForwardFrom == nil:
		preparing := tgbotapi.NewMessage(chat.ID, "Choice reply collocutor")
		bot.Send(preparing)
		return
	default:
		forwardID := int64(msg.ReplyToMessage.ForwardFrom.ID)
		if msg.Photo != nil {
			for _, image := range *msg.Photo {
				preparing := tgbotapi.NewPhotoShare(forwardID, image.FileID)
				bot.Send(preparing)
				return
			}
		}
		if msg.Document != nil {
			preparing := tgbotapi.NewDocumentShare(forwardID, msg.Document.FileID)
			bot.Send(preparing)
			return
		}
		if msg.Sticker != nil {
			preparing := tgbotapi.NewStickerShare(forwardID, msg.Sticker.FileID)
			bot.Send(preparing)
			return
		}
		if len(msg.Text) == 0 {
			text := "I understand next format: text, photo, sticker, any document"
			preparing := tgbotapi.NewMessage(chat.ID, text)
			bot.Send(preparing)
			return
		}
		preparing := tgbotapi.NewMessage(forwardID, msg.Text)
		bot.Send(preparing)
		return
	}
}

func (tb *TelegramBot) start(config Configuration) {
	for update := range tb.Updates {
		if update.Message != nil {
			tb.handleUpdates(update, config)
		}
	}
}

func getDescription(message string, c Configuration) (desc string) {
	switch message {
	case "/start":
		desc = c.Description.Start
	case "/about", "О проекте", "О канале":
		desc = c.Description.About
	case "/feedback", "Оставить отзыв":
		desc = c.Description.Feedback
	case "/ad", "Реклама", "Условия рекламы":
		desc = c.Description.Ad
	case "/suggest", "Предложить статью", "Предложить новость":
		desc = c.Description.Suggest
	}
	return
}

func getInlineMarkup(names, links []string) (keyboard tgbotapi.InlineKeyboardMarkup) {
	for i := 0; i < len(names); i++ {
		var row []tgbotapi.InlineKeyboardButton
		button := tgbotapi.NewInlineKeyboardButtonURL(names[i], links[i])
		row = append(row, button)
		keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, row)
	}
	return
}
