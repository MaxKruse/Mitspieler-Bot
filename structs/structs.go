package structs

import (
	"gorm.io/gorm"
)

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

type IngamePlayer struct {
	Name         string
	Champion     string
	Team         bool
	LeaguePoints int
}
