# Mitspieler Bot

This project contains a Twitch.tv chatbot, which aims at providing the `!mitspieler` or `!teammates` commands for League of Legends players.

## Getting Started

> go build -o my_cool_bot

Then run it

> ./my_cool_bot --help

## Todo

1. Build Database of lolpro players (endpoint: <https://api.lolpros.gg/es/ladder?page=1&sort=rank&order=desc>)
2. Associate Twitch Channel name with Player Aliases (endpoint: <https://api.lolpros.gg/es/players/player_name>)
3. Only print out relevant player names

## Credits

* <https://github.com/gempir/go-twitch-irc>
* <https://github.com/yuhanfang/riot>
