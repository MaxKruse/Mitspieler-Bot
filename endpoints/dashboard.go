package endpoints

import (
	"github.com/gofiber/fiber/v2"
	"github.com/maxkruse/Mitspieler-Bot/client/globals"
	"github.com/maxkruse/Mitspieler-Bot/client/structs"
)

func Dashboard(c *fiber.Ctx) error {

	// get the last 50 CommandLogs
	commandLogs := []structs.CommandLog{}

	err := globals.DBConn.Find(&commandLogs).Limit(50).Order("created_at desc").Error
	if err != nil {
		return c.Status(500).JSON(err)
	}

	content := "<table>"
	content += "<tr><th>User</th><th>Command</th><th>Channel</th><th>Time</th></tr>"

	for _, l := range commandLogs {
		content += "<tr>"

		content += "<td>" + l.Requester + "</td>"
		content += "<td>" + l.Command + "</td>"
		content += "<td>" + l.Channel + "</td>"
		content += "<td>" + l.CreatedAt.Format("2006-01-02 15:04:05") + "</td>"

		content += "</tr>"
	}

	content += "</table>"

	return c.Render("dashboard", fiber.Map{
		"content": content,
	})
}
