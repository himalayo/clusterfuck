package main

import (
	"context"
	"log"

	pb "github.com/himalayo/clusterfuck/api/player/proto"
)

func sendCfhTopics(ctx context.Context, evt *pb.LoginEvent) {
	categories, err := data.GetPartialCategories()
	if err != nil {
		return
	}
	packet := CfhTopicsComposer(categories)
	Net.Send(evt.UserData.AuthTicket, packet)
	log.Printf("Responded Login: %v", evt)
}

func RegisterLoginHandlers() {
	Player.RegisterLoginHandler(sendCfhTopics)
}
