package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	// custom imports
	"github.com/gempir/go-twitch-irc/v2"
	"github.com/yuhanfang/riot/apiclient"
	"github.com/yuhanfang/riot/constants/language"
	"github.com/yuhanfang/riot/constants/region"
	"github.com/yuhanfang/riot/ratelimit"
	"github.com/yuhanfang/riot/staticdata"
	uber "go.uber.org/ratelimit"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// structs

type Config struct {
	TWITCH_USERNAME string
	TWITCH_OAUTH    string
	RIOT_API_KEY    string
	TWITCH_CHANNELS []string
	DB_HOST         string
	DB_PORT         string
	DB_USER         string
	DB_PASS         string
}

type IngamePlayer struct {
	Name         string
	Champion     string
	Team         bool
	LeaguePoints int
}

// Database Specifics
type Player struct {
	gorm.Model
	Name     string    `json:"name"`
	Accounts []Account `json:"accounts"`
	Streamer Streamer
}

type Account struct {
	gorm.Model
	PlayerId     int64  `gorm:",primary_key"`
	SummonerName string `json:"summoner_name"`
}

type Streamer struct {
	gorm.Model
	Name     string `json:"name"`
	PlayerId int64
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

	// bg context
	ctx = context.Background()

	// Database
	db *gorm.DB

	// riot api client
	riotClient apiclient.Client

	// twitch client
	twitchClient *twitch.Client

	// list of all champions, used for looking up champion name by id
	champions *staticdata.ChampionList

	// Rate limiter
	ratelimiter uber.Limiter
)

func createDefaults(configPath string) {
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

	os.WriteFile(configPath, configStr, 0644)
}

func loadConfig(configPath string) error {
	log.Println("Reading config:", configPath)

	// read config.json file
	cfgFile, err := os.Open(configPath)
	if err != nil {
		log.Println("Config file not found or readable, creating defaults")
		createDefaults(configPath)

		return &ConfigError{}
	}
	defer cfgFile.Close()

	// parse configContent as json to config
	err = json.NewDecoder(cfgFile).Decode(&config)
	if err != nil {
		log.Println("Error parsing config file:", err)
		createDefaults(configPath)
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
	// ignore messages from self
	if m.User.Name == config.TWITCH_USERNAME {
		return
	}

	if m.Message == "!commands" {
		twitchClient.Say(m.Channel, "!mitspieler [Spieler/Streamer]")
		ratelimiter.Take()
	}
	// if m.Message starts with "!mitspieler"
	if strings.HasPrefix(m.Message, "!mitspieler") {
		prettyPrint(m)
		ratelimiter.Take()
		// split the message
		split := strings.Split(m.Message, " ")

		// If no arg was provided, search for the channel name
		var arguments []string
		if len(split) == 1 {
			arguments = append(arguments, m.Channel)
		} else {
			// get all values after first word
			arguments = split[1:]
		}
		streamerName := strings.Join(arguments, " ")

		summoner, err := riotClient.GetBySummonerName(ctx, region.EUW1, streamerName)
		if err != nil {
			log.Println("GetBySummonerName:", err)
			return
		}

		// get current game
		activeGame, err := riotClient.GetCurrentGameInfoBySummoner(ctx, region.EUW1, summoner.ID)
		if err != nil {
			log.Println("GetCurrentGameInfoBySummoner:", err)
			twitchClient.Say(m.Channel, fmt.Sprintf("%s scheint in keinen Game zu sein.", streamerName))
			return
		}

		// iterate through all champions in the game and print their champion name
		var players []IngamePlayer

		var myTeamId int64

		for _, player := range activeGame.Participants {
			if player.SummonerName == summoner.Name {
				myTeamId = player.TeamId
			}
		}

		for _, participant := range activeGame.Participants {
			//champion, err := riotClient.GetChampionByID(ctx, region.EUW1, champion.Champion(participant.ChampionId))
			if err != nil {
				log.Println("GetChampionByID:", err)
				return
			}

			var champName string

			for _, champ := range champions.Data {
				if champ.Key == fmt.Sprint(participant.ChampionId) {
					champName = champ.Name
					break
				}
			}

			celeb := Account{SummonerName: participant.SummonerName}
			res := Account{}

			db.Model(&Account{}).First(&res, celeb)

			// Some account was associated
			if res.PlayerId != 0 {
				temp := Player{}
				db.Model(&Player{}).First(&temp, res.PlayerId)

				encryptedSummonerId := participant.SummonerId
				res, _ := riotClient.GetAllLeaguePositionsForSummoner(ctx, region.EUW1, encryptedSummonerId)

				var leaguePos apiclient.LeaguePosition
				for _, pos := range res {
					if pos.QueueType == "RANKED_SOLO_5x5" {
						leaguePos = pos
						break
					}
				}

				if temp.Name != "" {
					players = append(players, IngamePlayer{Name: temp.Name, Champion: champName, Team: myTeamId == participant.TeamId, LeaguePoints: leaguePos.LeaguePoints})
				}
			}
		}

		if len(players) == 0 {
			return
		}

		// sort players by champion name
		sort.Slice(players, func(i, j int) bool {
			return players[i].LeaguePoints > players[j].LeaguePoints
		})

		// Turn players into string
		playersStringMyTeam := fmt.Sprintf("%s's Team: ", streamerName)
		playersStringEnemyTeam := "Gegner: "
		var myTeamPlayers []string
		var enemyTeamPlayers []string
		for _, player := range players {
			s := fmt.Sprintf("%s (%s) %d LP", player.Name, player.Champion, player.LeaguePoints)
			if player.Team {
				myTeamPlayers = append(myTeamPlayers, s)
			} else {
				enemyTeamPlayers = append(enemyTeamPlayers, s)
			}
		}

		twitchClient.Say(m.Channel, playersStringMyTeam+strings.Join(myTeamPlayers, ", ")+" | "+playersStringEnemyTeam+strings.Join(enemyTeamPlayers, ", "))
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
	}
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	logfile, err := os.OpenFile("server.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal(err)
	}
	multi := io.MultiWriter(logfile, os.Stdout)
	log.SetOutput(multi)
	log.Println("Starting server...")
	defer logfile.Close()

	ratelimiter = uber.New(5, uber.Per(60*time.Second))

	// flags
	configPath := flag.String("config", "config.json", "Path to config file")
	flag.Parse()

	log.Println("Mitspieler Bot")
	log.Println("Version:", VERSION)

	err = loadConfig(*configPath)

	if err != nil {
		log.Fatal(err.Error())
	}

	log.Println("Using Config:")
	prettyPrint(config)

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Europe/Berlin", config.DB_HOST, config.DB_USER, config.DB_PASS, "lolpros", config.DB_PORT)
	err = error(nil)
	log.Println("Connecting to database:")
	prettyPrint(dsn)
	db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
		return
	}
	log.Println("Connected to database")

	go setupRiot()
	go setupTwitch()

	// wait for CTRL+C
	select {}

}
