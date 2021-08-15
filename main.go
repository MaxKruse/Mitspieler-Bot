package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	// custom imports
	"github.com/gempir/go-twitch-irc/v2"
	"github.com/yuhanfang/riot/apiclient"
	"github.com/yuhanfang/riot/constants/language"
	"github.com/yuhanfang/riot/constants/region"
	"github.com/yuhanfang/riot/ratelimit"
	"github.com/yuhanfang/riot/staticdata"
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

	// bg context
	ctx = context.Background()

	// riot api client
	riotClient apiclient.Client

	// twitch client
	twitchClient *twitch.Client

	// list of all champions, used for looking up champion name by id
	champions *staticdata.ChampionList
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
	if m.Message == "!help" {
		twitchClient.Say(m.Channel, "!help - show this message")
	}
	if m.Message == "!version" {
		twitchClient.Say(m.Channel, VERSION)
	}
	// if m.Message contains "!test"
	if strings.Contains(m.Message, "!mitspieler") {
		// split the message
		split := strings.Split(m.Message, " ")

		// If no arg was provided, search for the channel name
		var arguments []string
		if len(split) < 1 {
			arguments[0] = m.Channel
		}
		// get all values after first word
		arguments = split[1:]

		summoner, err := riotClient.GetBySummonerName(ctx, region.EUW1, strings.Join(arguments, " "))
		if err != nil {
			log.Println("GetBySummonerName:", err)
			return
		}
		prettyPrint(summoner)

		// get current game
		activeGame, err := riotClient.GetCurrentGameInfoBySummoner(ctx, region.EUW1, summoner.ID)
		if err != nil {
			log.Println("GetCurrentGameInfoBySummoner:", err)
			return
		}
		prettyPrint(activeGame)

		// iterate through all champions in the game and print their champion name
		var playerChamps []string

		for _, participant := range activeGame.Participants {
			//champion, err := riotClient.GetChampionByID(ctx, region.EUW1, champion.Champion(participant.ChampionId))
			if err != nil {
				log.Println("GetChampionByID:", err)
				return
			}
			//prettyPrint(champion)
			prettyPrint(participant.SummonerName)

			var champName string

			for _, champ := range champions.Data {
				if champ.Key == fmt.Sprint(participant.ChampionId) {
					champName = champ.Name
					break
				}
			}

			// find champion name from list of all champions
			playerChamps = append(playerChamps, fmt.Sprintf("%s (%s)", champName, participant.SummonerName))
		}
		prettyPrint(playerChamps)
		twitchClient.Say(m.Channel, "Mitspieler: "+strings.Join(playerChamps, ", "))

	}
}

func setupRiot() {
	// make riot api client
	log.Println("Connecting to Riot API...")
	httpClient := http.DefaultClient
	limiter := ratelimit.NewLimiter()
	riotClient = apiclient.New(config.RIOT_API_KEY, httpClient, limiter)

	staticdataClient := staticdata.New(http.DefaultClient)
	versions, err := staticdataClient.Versions(ctx)
	if err != nil {
		log.Println("Error getting versions:", err)
		return
	}

	champions, err = staticdataClient.Champions(ctx, versions[0], language.EnglishUnitedStates)
	if err != nil {
		log.Println("Error getting champions:", err)
		return
	}
}

func setupTwitch() {
	// create a new client
	twitchClient = twitch.NewClient(config.TWITCH_USERNAME, config.TWITCH_OAUTH)

	// Set "On" event handlers
	twitchClient.OnConnect(onConnect)
	twitchClient.OnPrivateMessage(onMessage)

	// Join channels
	log.Println("Joining channels:", config.TWITCH_CHANNELS)
	twitchClient.Join(config.TWITCH_CHANNELS...)
	defer twitchClient.Disconnect()

	// Connect to twitch
	log.Println("Connecting to twitch...")
	err := twitchClient.Connect()
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
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

	go setupRiot()
	go setupTwitch()

	// wait for CTRL+C
	select {}
}
