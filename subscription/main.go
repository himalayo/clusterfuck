package main

import (
	_ "github.com/joho/godotenv/autoload"
)

func main() {
	data := NewDatabase(ConfigDatabaseFromEnv())
	go data.Listen()
	StartServer(data)
}
