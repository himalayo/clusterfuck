package main

import
(
	_ "github.com/joho/godotenv/autoload"
)

func main() {
	data := NewDatabase(ConfigDatabaseFromEnv())
	StartServer(data)
}
