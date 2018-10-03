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
	Token   string `json:"token"`
	AdLink  string `json:"ad_link"`
	WebSite string `json:"web_side"`
	Medium  string `json:"medium_channel"`
	GroupID int64  `json:"group_id"`
	Proxy   struct {
		URL      string `json:"url"`
		Login    string `json:"login"`
		Port     string `json:"port"`
		Password string `json:"password"`
	} `json:"proxy"`
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

//func (tb *TelegramBot) init(token string, debug bool) error {
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

func (tb *TelegramBot) start() error {
	for update := range tb.Updates {
		log.Printf("%+v\n", update.Message)
	}
	return nil
}

func Run(debug, updatesMode bool) {
	config := Configuration{}
	if err := config.Load("config.json"); err != nil {
		log.Fatal(err)
	}
	tb := &TelegramBot{}
	if err := tb.init(config, debug); err != nil {
		log.Fatal(err)
	}
	log.Printf("Authorized on account %s\nDebug mode: %v\n", tb.BotAPI.Self.UserName, tb.BotAPI.Debug)
	log.Println("update mode:", updatesMode)
	if updatesMode {
		if err := tb.setWebhook(config.Webhook.Host, config.Webhook.Port, config.Webhook.Cert, config.Webhook.Priv); err != nil {
			log.Fatal(err)
		}
	} else {
		if err := tb.getUpdates(); err != nil {
			log.Fatal(err)
		}
	}
	if err := tb.start(); err != nil {
		log.Fatal(err)
	}
}
