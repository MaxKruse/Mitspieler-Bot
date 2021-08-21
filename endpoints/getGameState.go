package endpoints

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/maxkruse/Mitspieler-Bot/client/globals"
	"github.com/maxkruse/Mitspieler-Bot/client/structs"
	"github.com/yuhanfang/riot/apiclient"
	"github.com/yuhanfang/riot/constants/region"
)

func prettyPrint(str interface{}) {
	strJson, _ := json.MarshalIndent(str, "", "  ")
	log.Printf("%s\n", string(strJson))
}

type GameState struct {
	Game         *apiclient.CurrentGameInfo
	SummonerName string
}

func findActiveAccount(account structs.Player) (GameState, error) {
	for _, acc := range account.Accounts {
		summoner, err := globals.RiotClient.GetBySummonerName(globals.BGContext, region.EUW1, acc.SummonerName)
		if err != nil {
			log.Println("GetBySummonerName:", err)
			continue
		}

		// get current game
		activeGame, err := globals.RiotClient.GetCurrentGameInfoBySummoner(globals.BGContext, region.EUW1, summoner.ID)
		if err != nil {
			log.Println("GetCurrentGameInfoBySummoner:", err)
			continue
		}

		if activeGame.GameID > 0 {
			return GameState{Game: activeGame, SummonerName: acc.SummonerName}, nil
		}
	}

	return GameState{}, errors.New("no active game found")
}

func resolveActiveGame(gameinfo *apiclient.CurrentGameInfo, summonerName string, streamerName string) (string, error) {

	var players []structs.IngamePlayer

	var myTeamId int64

	for _, player := range gameinfo.Participants {
		if player.SummonerName == summonerName {
			myTeamId = player.TeamId
		}
	}

	for _, participant := range gameinfo.Participants {

		var champName string
		for _, champ := range globals.Champions.Data {
			if champ.Key == fmt.Sprint(participant.ChampionId) {
				champName = champ.Name
				break
			}
		}

		celeb := structs.Account{SummonerName: participant.SummonerName}
		res := structs.Account{}

		globals.DBConn.Model(&structs.Account{}).First(&res, celeb)

		// Some account was associated
		if res.PlayerId != 0 {
			temp := structs.Player{}
			globals.DBConn.Model(&structs.Player{}).First(&temp, res.PlayerId)

			encryptedSummonerId := participant.SummonerId
			res, _ := globals.RiotClient.GetAllLeaguePositionsForSummoner(globals.BGContext, region.EUW1, encryptedSummonerId)

			var leaguePos apiclient.LeaguePosition
			for _, pos := range res {
				if pos.QueueType == "RANKED_SOLO_5x5" {
					leaguePos = pos
					break
				}
			}

			if temp.Name != "" {
				players = append(players, structs.IngamePlayer{Name: temp.Name, Champion: champName, Team: myTeamId == participant.TeamId, LeaguePoints: leaguePos.LeaguePoints})
			}
		}
	}

	if len(players) == 0 {
		return "", errors.New("no players found")
	}

	// sort players by champion name
	sort.Slice(players, func(i, j int) bool {
		return players[i].LeaguePoints > players[j].LeaguePoints
	})

	// Turn players into string
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

	res := fmt.Sprintf("%s's Team: "+strings.Join(myTeamPlayers, ", "), streamerName)

	if len(enemyTeamPlayers) > 0 {
		res += " | " + "Gegner: " + strings.Join(enemyTeamPlayers, ", ")
	}

	return res, nil
}

func GetGameState(c *fiber.Ctx) error {

	localDb := globals.DBConn
	streamer := c.Params("streamerName")

	search := structs.Streamer{Name: streamer}

	prettyPrint(search)
	localDb.Model(&structs.Streamer{}).Where("LOWER(streamer_name) = ?", strings.ToLower(search.Name)).First(&search)
	prettyPrint(search)

	if search.Name == "" {
		return c.Status(404).SendString(fmt.Sprintf("%s not in database. Please contact BH_Lithium.", streamer))
	}

	player := structs.Player{}
	localDb.Preload("Accounts").Preload("Streamers").First(&player, search.PlayerId)

	if len(player.Accounts) == 0 {
		return c.SendString("Keine accounts gefunden.")
	}

	gameinfo, err := findActiveAccount(player)
	if err != nil {
		return c.SendString(fmt.Sprintf("%s ist in keinen Game.", streamer))
	}

	globals.DBConn.Create(&structs.CommandLog{Requester: "Nightbot", Command: "!mitspieler", Channel: streamer})

	res, err := resolveActiveGame(gameinfo.Game, gameinfo.SummonerName, streamer)
	if err != nil {
		return c.SendString(fmt.Sprintf("%s ist in keinen Game.", streamer))
	}

	return c.SendString(res)
}