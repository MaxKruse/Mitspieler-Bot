package main

import (
	"encoding/json"
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

	"github.com/maxkruse/Mitspieler-Bot/client/structs"

	// import gorm
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	// Ratelimit http
	"go.uber.org/ratelimit"
	// Cron
)

// structs to decode values into
type LadderEntry struct {
	Name string `json:"slug"`
}

// Riot Specifics
type RiotPlayer struct {
	LeaguePlayer `json:"league_player"`
	Name         string `json:"name"`
	Teams        []Team `json:"teams"`
}

type Team struct {
	Name string `json:"tag"`
}
type LeaguePlayer struct {
	Position string            `json:"position"`
	Accounts []structs.Account `json:"accounts"`
}

const (
	DB_HOST = "db"
	DB_PORT = "5432"
	DB_USER = "postgres"
	DB_PASS = "mitspieler"

	// Use fmt.Sprintf on this to change the page
	LADDER_URL = "https://api.lolpros.gg/es/ladder?page=%d&sort=rank&order=desc"
	PLAYER_URL = "https://api.lolpros.gg/es/players/%s"

	PLAYERS = 2500.0 // lolpros limitation
)

var (
	db *gorm.DB

	Streamers []structs.Streamer

	ratelimiter = ratelimit.New(1)
)

func getLadderUrl(page int) string {
	return fmt.Sprintf(LADDER_URL, page)
}

func getPlayerUrl(player string) string {
	return fmt.Sprintf(PLAYER_URL, url.QueryEscape(player))
}

func makeApiCall(url string) ([]byte, error) {
	time.Sleep(time.Second * 1)
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

	log.Println("Requested:", url)

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

	for _, t := range riotplayer.LeaguePlayer.Accounts {
		player.Accounts = append(player.Accounts, &t)
	}

	// cut first 3 characters from position
	if len(riotplayer.LeaguePlayer.Position) > 3 {
		player.Position = riotplayer.LeaguePlayer.Position[3:]
		// uppercase first letter
		player.Position = strings.ToUpper(player.Position[:1]) + player.Position[1:]
	}

	player.Name = riotplayer.Name

	db.Preload("Accounts").Preload("Streamer").First(&player, "name = ?", player.Name)

	if len(riotplayer.Teams) > 0 {
		player.TeamTag = riotplayer.Teams[0].Name
	} else {
		player.TeamTag = ""
	}

	// If player.Name is in Streamers, save
	for _, streamer := range Streamers {
		if streamer.Name == player.Name && player.Streamer == nil {
			player.Streamer = new(structs.Streamer)
			player.Streamer.Name = streamer.Name
			player.Streamer.StreamerName = streamer.StreamerName
			log.Println("Streamer Found:", player.Name)
			prettyPrint(player)
			break
		}
	}

	// Only create entry if player is not in db
	if player.ID < 1 {
		db.Save(&player)
		log.Println("Saved", player.Name)
	} else {
		accs := player.Accounts
		for _, account := range accs {
			found := false
			new := structs.Account{}
			for _, newAccount := range player.Accounts {
				if strings.EqualFold(newAccount.SummonerName, account.SummonerName) {
					found = true
					new = *newAccount
					break
				}
			}
			if !found {
				player.Accounts = append(player.Accounts, &new)
				log.Println("Added", new)
			}
		}

		db.Save(&player)
	}
}

func populatePage(wg *sync.WaitGroup, page int) {
	defer wg.Done()

	// Do actual work with the page
	url := getLadderUrl(page)
	bytes, err := makeApiCall(url)
	if err != nil || len(bytes) < 50 {
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

func FetchLolpros() {
	// Load streamers from json
	file, err := os.Open("/app/streamers.json")
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

	// make a go routine for each page index until 5
	count := int(math.Ceil(PLAYERS / float64(50)))

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
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	logfile, err := os.OpenFile("lolpros.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal(err)
	}
	multi := io.MultiWriter(logfile, os.Stdout)
	log.SetOutput(multi)
	log.Println("Starting Lolpros Fetcher...")
	defer logfile.Close()

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Europe/Berlin", DB_HOST, DB_USER, DB_PASS, "lolpros", DB_PORT)
	err = error(nil)
	db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})

	if err != nil {
		log.Fatal(err)
		return
	}
	log.Println("Connected to database")

	// create the tables
	db.AutoMigrate(structs.Player{}, structs.Account{}, structs.Streamer{})

	FetchLolpros()
}
