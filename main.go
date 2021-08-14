package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"

	// custom imports
	"github.com/gempir/go-twitch-irc/v2"
)

// structs

type Config struct {
	TWITCH_USERNAME string
	TWITCH_OAUTH    string
	RIOT_API_KEY    string
	TWITCH_CHANNELS []string
}

// Custom errors
type ConfigError struct{}

func (e *ConfigError) Error() string {
	return "Default Config Created, please update it."
}

const (
	VERSION = "0.0.1"
)

var (
	// env vars
	config Config

	// flags
	configPath = flag.String("config", "config.json", "Path to config file")
)

func createDefaults() {
	config = Config{
		TWITCH_USERNAME: "bot_username",
		TWITCH_OAUTH:    "oauth:your_oauth_here",
		RIOT_API_KEY:    "",
		TWITCH_CHANNELS: []string{},
	}

	configStr, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		log.Fatal(err)
	}

	os.WriteFile(*configPath, configStr, 0644)
}

func loadConfig() error {
	log.Println("Reading config:", *configPath)

	// read config.json file
	cfgFile, err := os.Open(*configPath)
	if err != nil {
		log.Println("Config file not found or readable, creating defaults")
		createDefaults()

		return &ConfigError{}
	}
	defer cfgFile.Close()

	// parse configContent as json to config
	err = json.NewDecoder(cfgFile).Decode(&config)
	if err != nil {
		log.Println("Error parsing config file:", err)
		createDefaults()
		return &ConfigError{}
	}
	log.Println("Config loaded")
	return nil
}

func prettyPrint(str interface{}) {
	strJson, _ := json.MarshalIndent(str, "", "  ")
	log.Printf("%s\n", string(strJson))
}

// onConnect is called when the client connects to the server
func onConnect() {
	log.Println("Connected to twitch")
}

func onMessage(m twitch.PrivateMessage) {
	prettyPrint(m)
}

func main() {
	flag.Parse()
	log.Println("Mitspieler Bot")
	log.Println("Version:", VERSION)

	err := loadConfig()

	if err != nil {
		log.Fatal(err.Error())
		os.Exit(1)
	}

	log.Println("Using Config:")
	prettyPrint(config)

	// create a new client
	client := twitch.NewClient(config.TWITCH_USERNAME, config.TWITCH_OAUTH)

	// Set "On" event handlers
	client.OnConnect(onConnect)
	client.OnPrivateMessage(onMessage)

	// Connect to twitch
	err = client.Connect()
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	// Join channels
	client.Join(config.TWITCH_CHANNELS...)

}
