package main

import _ "github.com/joho/godotenv/autoload"

func main() {
	StartServer(NewDatabase(ConfigDatabaseFromEnv()))
}
