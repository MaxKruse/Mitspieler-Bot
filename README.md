[![Go](https://github.com/MaxKruse/Mitspieler-Bot/actions/workflows/go.yml/badge.svg?branch=master)](https://github.com/MaxKruse/Mitspieler-Bot/actions/workflows/go.yml)

# Mitspieler Bot

This project contains a Twitch.tv chatbot, which aims at providing the `!mitspieler` or `!teammates` commands for League of Legends players.

## Getting Started

> go build -o my_cool_bot

Then run it

> ./my_cool_bot --help

## Usage

This bot supports Nightbot.

Add a command with the following syntax:

`$(urlfetch http://<bot-url>/streamer/$(channel))`

## Credits

* <https://github.com/gempir/go-twitch-irc>
* <https://github.com/yuhanfang/riot>
