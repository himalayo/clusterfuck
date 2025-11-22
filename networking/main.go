package main

import (
	"flag"
	"net/http"
	"log"
	player "github.com/himalayo/clusterfuck/player/api"
)

var (
	addr = flag.String("addr", "localhost:2096", "http service address")
	playerAddr = flag.String("player_addr", "localhost:50053", "player service address")
	Man = newManager()
	Player = player.NewClient()
	Auth = newAuth()
)

func main() {
	flag.Parse()
	log.SetFlags(0)
	go Man.run()
	go Player.Listen(*playerAddr)
	go Auth.run(Player, Man)
	RegisterHandlers()
	go StartRpc()
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		serveWs(Man, w, r)
	})
	log.Fatal(http.ListenAndServe(*addr, nil))
}
