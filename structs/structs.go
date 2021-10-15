package structs

import (
	"gorm.io/gorm"
)

// Database Specifics
type Player struct {
	gorm.Model
	Name     string    `json:"name"`
	Position string    `json:"position"`
	TeamTag  string    `json:"team"`
	Accounts []Account `json:"accounts"`
	Streamer *Streamer
}

type Account struct {
	gorm.Model
	PlayerId     int64  `gorm:",primary_key"`
	SummonerName string `json:"summoner_name"`
}

type Streamer struct {
	gorm.Model
	Name         string `json:"name"`
	StreamerName string `json:"streamer_name"`
	PlayerId     int64
}

type IngamePlayer struct {
	Name         string
	Champion     string
	Team         bool
	Position     string
	LeaguePoints int
	TeamTag      string
}

type CommandLog struct {
	gorm.Model
	Command   string
	Requester string
	Channel   string
}
