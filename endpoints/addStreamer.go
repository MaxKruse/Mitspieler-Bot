package endpoints

import (
	"github.com/gofiber/fiber/v2"
	"github.com/maxkruse/Mitspieler-Bot/client/globals"
	"github.com/maxkruse/Mitspieler-Bot/client/structs"
)

func AddStreamer(c *fiber.Ctx) error {
	streamer := structs.Streamer{}

	if err := c.BodyParser(&streamer); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"Message": err.Error(),
		})
	}

	player := structs.Player{}
	localDB := globals.DBConn
	localDB.Model(&player).Preload("Streamer").Where("Name = ?", streamer.Name).First(&player)

	if player.ID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Player not found",
		})
	}

	if player.Streamer != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Player already has a streamer",
		})
	}

	player.Streamer = new(structs.Streamer)
	player.Streamer.Name = streamer.Name
	player.Streamer.StreamerName = streamer.StreamerName

	localDB.Save(&player)

	return c.Status(fiber.StatusCreated).JSON(player)
}
