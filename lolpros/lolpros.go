package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"sync"

	// import gorm
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// structs to decode values into
type LadderEntry struct {
	Name string `json:"name"`
}

// Database Specifics
type Player struct {
	gorm.Model
	Name     string    `json:"name"`
	Accounts []Account `json:"accounts"`
}

type Account struct {
	gorm.Model
	PlayerId     int64  `gorm:",primary_key"`
	SummonerName string `json:"summoner_name"`
}

// Riot Specifics
type RiotPlayer struct {
	LeaguePlayer `json:"league_player"`
	Name         string `json:"name"`
}

type LeaguePlayer struct {
	Accounts []Account `json:"accounts"`
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
	httpClient = &http.Client{}

	players = flag.Int("players", 50, "Number of players to pull")

	db *gorm.DB
)

func getLadderUrl(page int) string {
	return fmt.Sprintf(LADDER_URL, page)
}

func getPlayerUrl(player string) string {
	return fmt.Sprintf(PLAYER_URL, player)
}

func makeApiCall(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}
	defer resp.Body.Close()

	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}

	return bytes, nil
}

func makeLadderEntries(bytes []byte) ([]LadderEntry, error) {
	var entry []LadderEntry
	err := json.Unmarshal(bytes, &entry)
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
		log.Fatal(err)
		return
	}

	for _, entry := range entries {
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

		var player Player
		player.Accounts = riotplayer.LeaguePlayer.Accounts
		player.Name = riotplayer.Name

		var local Player
		db.First(&local, player)
		// Only create entry if player is not in db
		if local.ID < 1 {
			db.Model(&player).Save(&player)
		}
	}
}

func main() {
	// setup log for ms
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	flag.Parse()

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Europe/Berlin", DB_HOST, DB_USER, DB_PASS, "lolpros", DB_PORT)
	err := error(nil)
	db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})

	if err != nil {
		log.Fatal(err)
		return
	}

	// create the tables
	db.AutoMigrate(Player{}, Account{})

	// make a go routine for each page index until 5
	count := int(math.Ceil(float64(*players) / float64(50)))

	var wg sync.WaitGroup
	wg.Add(count)

	for i := 1; i <= int(count); i++ {
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
