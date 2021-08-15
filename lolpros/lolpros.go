package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"math/rand"
	"net/http"
	"sync"
	"time"

	// import gorm
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// structs to decode values into
type LadderEntry struct {
	Name string `json:"name"`
}

type Player struct {
	gorm.Model
	Name     string    `json:"name"`
	Accounts []Account `json:"accounts"`
}

type Account struct {
	gorm.Model
	PlayerId     int64
	SummonerName string `json:"summoner_name"`
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
		log.Println(err)
		return nil, err
	}
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	defer resp.Body.Close()

	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	return bytes, nil
}

func makeLadderEntry(bytes []byte) (LadderEntry, error) {
	var entry LadderEntry
	err := json.Unmarshal(bytes, &entry)
	return entry, err
}

func makePlayer(bytes []byte) (Player, error) {
	var player Player
	err := json.Unmarshal(bytes, &player)
	return player, err
}

func main() {
	// setup log for ms
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	flag.Parse()

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Europe/Berlin", DB_HOST, DB_USER, DB_PASS, "lolpros", DB_PORT)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})

	if err != nil {
		log.Println(err)
		return
	}

	// create the tables
	db.AutoMigrate(Player{})

	// make a go routine for each page index until 5
	count := int(math.Ceil(float64(*players) / float64(50)))

	var wg sync.WaitGroup
	wg.Add(count)

	for i := 1; i <= int(count); i++ {
		go func(page int, wg *sync.WaitGroup) {
			defer wg.Done()

			// Sleep between 0 and 5 seconds
			time.Sleep(time.Duration(rand.Intn(5)) * time.Second)
			log.Println("Hello World from", page)
		}(i, &wg)
	}

	wg.Wait()
	log.Println("I did it mom!")
}
