package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	// import gorm
	"github.com/maxkruse/Mitspieler-Bot/client/structs"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	// Ratelimit http
	"go.uber.org/ratelimit"
)

// structs to decode values into
type LadderEntry struct {
	Name string `json:"name"`
}

// Riot Specifics
type RiotPlayer struct {
	LeaguePlayer `json:"league_player"`
	Name         string `json:"name"`
}

type LeaguePlayer struct {
	Position string            `json:"position"`
	Accounts []structs.Account `json:"accounts"`
}

const (
	DB_HOST = "localhost"
	DB_PORT = "5432"
	DB_USER = "postgres"
	DB_PASS = "mitspieler"

	// Use fmt.Sprintf on this to change the page
	LADDER_URL = "https://api.lolpros.gg/es/ladder?page=%d&sort=rank&order=desc"
	PLAYER_URL = "https://api.lolpros.gg/es/players/%s"
)

var (
	players       = flag.Int("players", 50, "Number of players to pull")
	saveStreamers = flag.Bool("save-streamers", false, "Save streamers to db")

	db *gorm.DB

	Streamers []structs.Streamer

	ratelimiter = ratelimit.New(2)
)

func getLadderUrl(page int) string {
	return fmt.Sprintf(LADDER_URL, page)
}

func getPlayerUrl(player string) string {
	return fmt.Sprintf(PLAYER_URL, url.QueryEscape(strings.ReplaceAll(player, " ", "-")))
}

func makeApiCall(url string) ([]byte, error) {
	ratelimiter.Take()
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")

	var httpClient http.Client
	resp, err := httpClient.Do(req)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}
	defer resp.Body.Close()
	time.Sleep(1 * time.Second)

	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		prettyPrint(bytes)
		log.Fatal(err)
		return nil, err
	}
	return bytes, nil
}

func makeLadderEntries(bytes []byte) ([]LadderEntry, error) {
	var entry []LadderEntry
	err := json.Unmarshal(bytes, &entry)
	if err != nil {
		prettyPrint(string(bytes))
	}
	return entry, err
}

func makePlayer(bytes []byte) (RiotPlayer, error) {
	var player RiotPlayer
	err := json.Unmarshal(bytes, &player)
	return player, err
}

func prettyPrint(str interface{}) {
	strJson, _ := json.MarshalIndent(str, "", "  ")
	log.Printf("%s\n", string(strJson))
}

func savePlayer(wg *sync.WaitGroup, entry LadderEntry) {
	defer wg.Done()

	playerUrl := getPlayerUrl(entry.Name)
	bytes, err := makeApiCall(playerUrl)
	if err != nil {
		log.Println(err)
		return
	}

	riotplayer, err := makePlayer(bytes)
	if err != nil {
		log.Println(err)
		return
	}

	var player structs.Player
	player.Accounts = riotplayer.LeaguePlayer.Accounts

	// cut first 3 characters from position
	if len(riotplayer.LeaguePlayer.Position) > 3 {
		player.Position = riotplayer.LeaguePlayer.Position[3:]
		// uppercase first letter
		player.Position = strings.ToUpper(player.Position[:1]) + player.Position[1:]
	}

	player.Name = riotplayer.Name

	// If player.Name is in Streamers, save
	for _, streamer := range Streamers {
		if streamer.Name == player.Name {
			player.Streamer = streamer
			prettyPrint(player)
			break
		}
	}

	var local structs.Player
	db.First(&local, player)
	// Only create entry if player is not in db
	if local.ID < 1 {
		db.Model(&player).Save(&player)
	}
}

func populatePage(wg *sync.WaitGroup, page int) {
	defer wg.Done()

	// Do actual work with the page
	url := getLadderUrl(page)
	bytes, err := makeApiCall(url)
	if err != nil {
		log.Fatal(err)
		return
	}

	entries, err := makeLadderEntries(bytes)
	if err != nil {
		log.Println(entries)
		log.Fatal(err)
		return
	}

	wg.Add(len(entries))
	for _, entry := range entries {
		go savePlayer(wg, entry)
	}
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	logfile, err := os.OpenFile("lolpros.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal(err)
	}
	multi := io.MultiWriter(logfile, os.Stdout)
	log.SetOutput(multi)
	log.Println("Starting server...")
	defer logfile.Close()

	flag.Parse()

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Europe/Berlin", DB_HOST, DB_USER, DB_PASS, "lolpros", DB_PORT)
	err = error(nil)
	db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})

	if err != nil {
		log.Fatal(err)
		return
	}

	// create the tables
	db.AutoMigrate(structs.Player{}, structs.Account{}, structs.Streamer{})

	// Load streamers from json
	file, err := os.Open("streamers.json")
	if err != nil {
		log.Fatal(err)
		return
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&Streamers)
	if err != nil {
		log.Fatal(err)
		return
	}

	if !*saveStreamers {

		// make a go routine for each page index until 5
		count := int(math.Ceil(float64(*players) / float64(50)))

		var wg sync.WaitGroup
		wg.Add(count)

		for i := 1; i <= int(count); i++ {
			time.Sleep(250 * time.Millisecond)
			go populatePage(&wg, i)

			// Sleep between 0 and 5 seconds

			// Example code
			// p := &Player{
			// 	Name: "Agurin",
			// 	Accounts: []Account{
			// 		{SummonerName: "Agurin"},
			// 		{SummonerName: "Charlie Heaton"},
			// 	},
			// }
			// db.Model(Player{}).FirstOrCreate(p)

		}

		wg.Wait()

	} else {
		// Check for all streamers if the player is in the db
		for _, streamer := range Streamers {
			var player structs.Player
			db.Model(&structs.Player{}).Where("name = ?", streamer.Name).First(&player)
			if err != nil {
				log.Fatal(err)
				return
			}
			if player.ID > 0 {
				log.Printf("%s is in the db", streamer.Name)
				var temp structs.Streamer
				db.First(&temp, streamer)
				streamer.PlayerId = int64(player.ID)
				db.Save(&streamer)
			}
		}
	}
}
