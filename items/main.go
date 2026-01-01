package main

import (
	"context"

	_ "github.com/joho/godotenv/autoload"
)

var (
	data = NewDatabase(ConfigDatabaseFromEnv())
)

func main() {
	go data.loadBaseItemsFromDB(context.Background())
	StartServer(data)
}
