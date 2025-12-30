package main

import (
	_ "github.com/joho/godotenv/autoload"
)

var (
	data = NewDatabase(ConfigDatabaseFromEnv())
)

func main() {
	StartServer(data)
}
