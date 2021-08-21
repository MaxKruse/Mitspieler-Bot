package globals

import (
	"context"

	"github.com/yuhanfang/riot/apiclient"
	"github.com/yuhanfang/riot/staticdata"
	uber "go.uber.org/ratelimit"
	_ "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var (
	DBConn *gorm.DB

	RiotClient apiclient.Client
	Champions  *staticdata.ChampionList

	BGContext context.Context

	// Rate limiter
	Ratelimiter uber.Limiter
)
