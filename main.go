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
	"time"

	// custom imports

	"github.com/gofiber/fiber/v2"
	"github.com/maxkruse/Mitspieler-Bot/client/endpoints"
	"github.com/maxkruse/Mitspieler-Bot/client/globals"
	"github.com/maxkruse/Mitspieler-Bot/client/structs"
	"github.com/yuhanfang/riot/apiclient"
	"github.com/yuhanfang/riot/constants/language"
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
	PORT            int
	TWITCH_CHANNELS []string
	DB_HOST         string
	DB_PORT         string
	DB_USER         string
	DB_PASS         string
}

// Custom errors
type ConfigError struct{}

func (e *ConfigError) Error() string {
	return "Please use the above in your config file"
}

const (
	VERSION = "0.0.1"
)

var (
	// env vars
	config Config

	// bg context
	ctx = context.Background()
)

func createDefaults(configPath string) {
	config = Config{
		TWITCH_USERNAME: "bot_username",
		TWITCH_OAUTH:    "oauth:your_oauth_here",
		RIOT_API_KEY:    "",
		TWITCH_CHANNELS: []string{},
		PORT:            5000,
		DB_HOST:         "localhost",
		DB_PORT:         "5432",
		DB_USER:         "postgres",
		DB_PASS:         "postgres",
	}

	configStr, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(configStr))
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

func setupRiot() {
	// make riot api client
	log.Println("Connecting to Riot API...")
	httpClient := http.DefaultClient
	limiter := ratelimit.NewLimiter()
	globals.RiotClient = apiclient.New(config.RIOT_API_KEY, httpClient, limiter)

	staticdataClient := staticdata.New(http.DefaultClient)
	versions, err := staticdataClient.Versions(ctx)
	if err != nil {
		log.Println("Error getting versions:", err)
		return
	}

	globals.Champions, err = staticdataClient.Champions(ctx, versions[0], language.EnglishUnitedStates)
	if err != nil {
		log.Println("Error getting champions:", err)
		return
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

	globals.Ratelimiter = uber.New(5, uber.Per(60*time.Second))

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
	globals.DBConn, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
		return
	}

	log.Println("Connected to database")
	globals.DBConn.AutoMigrate(&structs.CommandLog{})

	setupRiot()

	// Setup global context
	globals.BGContext = context.Background()

	// Development hot reload
	app := fiber.New()

	app.Get("/streamer/:streamerName", endpoints.GetGameState)
	app.Get("/reload/config", func(c *fiber.Ctx) error {
		err = loadConfig(*configPath)

		if err != nil {
			log.Fatal(err.Error())
		}
		setupRiot()
		return nil
	})

	log.Fatal(app.Listen(fmt.Sprintf(":%d", config.PORT)))
}
