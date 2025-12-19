package main

import (
	"context"
	"log"

	pb "github.com/himalayo/clusterfuck/api/player/proto"
)

func sendInventoryAchievements(ctx context.Context, evt *pb.LoginEvent) {
	achievements, err := data.GetInventoryAchievements()
	if err != nil {
		return
	}
	log.Printf("Responding to Login Event: %v", evt)
	Net.Send(evt.UserData.AuthTicket, InventoryAchievementsComposer(achievements))
}

func RegisterLoginHandlers() {
	Player.RegisterLoginHandler(sendInventoryAchievements)
}
