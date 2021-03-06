package endpoints

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/maxkruse/Mitspieler-Bot/client/globals"
	"github.com/maxkruse/Mitspieler-Bot/client/structs"
	"github.com/yuhanfang/riot/apiclient"
	"github.com/yuhanfang/riot/constants/region"
	"github.com/yuhanfang/riot/constants/tier"
)

type GameState struct {
	Game         *apiclient.CurrentGameInfo
	SummonerName string
}

func findActiveAccount(account structs.Player) (GameState, error) {
	// 3 second timeout
	timeoutCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	for _, acc := range account.Accounts {
		summoner, err := globals.RiotClient.GetBySummonerName(timeoutCtx, region.EUW1, acc.SummonerName)
		if err != nil {
			log.Println("GetBySummonerName:", err)
			continue
		}

		// get current game
		activeGame, err := globals.RiotClient.GetCurrentGameInfoBySummoner(timeoutCtx, region.EUW1, summoner.ID)
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

	// 2 second timeout
	timeoutCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var players []structs.IngamePlayer

	var myTeamId int64

	for _, player := range gameinfo.Participants {
		if player.SummonerName == summonerName {
			myTeamId = player.TeamId
		}
	}

	runningTotalLP := 0
	totalPlayers := 0

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

		globals.DBConn.Debug().First(&res, celeb)

		encryptedSummonerId := participant.SummonerId
		playerPositions, _ := globals.RiotClient.GetAllLeaguePositionsForSummoner(timeoutCtx, region.EUW1, encryptedSummonerId)

		for _, pos := range playerPositions {
			if pos.QueueType == "RANKED_SOLO_5x5" {

				if pos.Tier == tier.Challenger || pos.Tier == tier.Grandmaster || pos.Tier == tier.Master {
					runningTotalLP += pos.LeaguePoints
					totalPlayers++
					break
				}
			}
		}

		// Some account was associated
		if res.PlayerId != 0 {
			temp := structs.Player{}
			globals.DBConn.Debug().First(&temp, res.PlayerId)

			var leaguePos apiclient.LeaguePosition
			for _, pos := range playerPositions {
				if pos.QueueType == "RANKED_SOLO_5x5" {
					leaguePos = pos
					break
				}
			}

			if temp.Name != "" {
				players = append(players, structs.IngamePlayer{Name: temp.Name, Champion: champName, Team: myTeamId == participant.TeamId, LeaguePoints: leaguePos.LeaguePoints, Position: temp.Position, TeamTag: temp.TeamTag})
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
		var name string
		if player.TeamTag != "" {
			name = fmt.Sprintf("%s %s", player.TeamTag, player.Name)
		} else {
			name = player.Name
		}

		s := fmt.Sprintf("%s (%s) %d LP", name, player.Champion, player.LeaguePoints)
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

	avgLp := runningTotalLP / totalPlayers

	res += " | Average " + fmt.Sprintf("%d", avgLp) + " LP"

	return res, nil
}

func GetGameState(c *fiber.Ctx) error {

	localDb := globals.DBConn
	streamer := c.Params("streamerName")

	search := structs.Streamer{Name: streamer}

	localDb.Debug().Where("LOWER(streamer_name) = ?", strings.ToLower(search.Name)).First(&search)

	if search.Name == "" {
		return c.Status(404).SendString(fmt.Sprintf("%s not in database. Please contact BH_Lithium.", streamer))
	}

	requester := c.Query("requester", streamer)
	requester = strings.ReplaceAll(requester, "@", "")

	player := structs.Player{}
	localDb.Debug().Preload("Accounts").Preload("Streamer").First(&player, search.PlayerId)

	if len(player.Accounts) == 0 {
		return c.SendString("Keine accounts gefunden.")
	}

	gameinfo, err := findActiveAccount(player)
	if err != nil {
		return c.SendString(fmt.Sprintf("%s ist in keinem Game.", streamer))
	}

	globals.DBConn.Debug().Create(&structs.CommandLog{Requester: requester, Command: "!mitspieler", Channel: streamer})

	res, err := resolveActiveGame(gameinfo.Game, gameinfo.SummonerName, streamer)
	if err != nil {
		return c.SendString(fmt.Sprintf("%s ist in keinem Game.", streamer))
	}

	return c.SendString(res)
}
