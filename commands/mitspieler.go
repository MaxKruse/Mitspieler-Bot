package commands

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/gempir/go-twitch-irc/v2"
	"github.com/maxkruse/Mitspieler-Bot/client/structs"
	"github.com/yuhanfang/riot/apiclient"
	"github.com/yuhanfang/riot/constants/region"
	"github.com/yuhanfang/riot/staticdata"
	uber "go.uber.org/ratelimit"
	"gorm.io/gorm"
)

type MitspielerCommand struct {
	ctx          context.Context
	twitchClient *twitch.Client
	riotClient   apiclient.Client
	champions    *staticdata.ChampionList
	message      twitch.PrivateMessage
	db           *gorm.DB
	limiter      uber.Limiter
}

func NewMitspielerCommand(twitchClient *twitch.Client, riotClient *apiclient.Client, champions *staticdata.ChampionList, message *twitch.PrivateMessage, db *gorm.DB, rateLimiter *uber.Limiter) MitspielerCommand {
	return MitspielerCommand{
		ctx:          context.Background(),
		riotClient:   *riotClient,
		champions:    champions,
		twitchClient: twitchClient,
		message:      *message,
		db:           db,
		limiter:      *rateLimiter,
	}
}

func (m *MitspielerCommand) Run() {
	// split the message
	split := strings.Split(m.message.Message, " ")

	// If no arg was provided, search for the channel name
	var arguments []string
	if len(split) == 1 {
		arguments = append(arguments, m.message.Channel)
	} else {
		// get all values after first word
		arguments = split[1:]
	}
	streamerName := strings.Join(arguments, " ")

	summoner, err := m.riotClient.GetBySummonerName(m.ctx, region.EUW1, streamerName)
	if err != nil {
		log.Println("GetBySummonerName:", err)
		return
	}

	// get current game
	activeGame, err := m.riotClient.GetCurrentGameInfoBySummoner(m.ctx, region.EUW1, summoner.ID)
	if err != nil {
		log.Println("GetCurrentGameInfoBySummoner:", err)
		m.twitchClient.Say(m.message.Channel, fmt.Sprintf("%s scheint in keinen Game zu sein.", streamerName))
		return
	}

	m.limiter.Take()
	// Save command to db
	m.db.Create(&structs.CommandLog{Requester: m.message.User.DisplayName, Command: m.message.Message, Channel: m.message.Channel})

	// iterate through all champions in the game and print their champion name
	var players []structs.IngamePlayer

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

		for _, champ := range m.champions.Data {
			if champ.Key == fmt.Sprint(participant.ChampionId) {
				champName = champ.Name
				break
			}
		}

		celeb := structs.Account{SummonerName: participant.SummonerName}
		res := structs.Account{}

		m.db.Model(&structs.Account{}).First(&res, celeb)

		// Some account was associated
		if res.PlayerId != 0 {
			temp := structs.Player{}
			m.db.Model(&structs.Player{}).First(&temp, res.PlayerId)

			encryptedSummonerId := participant.SummonerId
			res, _ := m.riotClient.GetAllLeaguePositionsForSummoner(m.ctx, region.EUW1, encryptedSummonerId)

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

	m.twitchClient.Say(m.message.Channel, playersStringMyTeam+strings.Join(myTeamPlayers, ", ")+" | "+playersStringEnemyTeam+strings.Join(enemyTeamPlayers, ", "))
}
