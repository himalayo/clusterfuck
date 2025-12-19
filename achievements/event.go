package main

import (
	"context"

	pb "github.com/himalayo/clusterfuck/api/player/proto"
)

func sendInventoryAchievements(ctx context.Context, evt *pb.LoginEvent) {
	achievements, err := data.GetInventoryAchievements()
	if err != nil {
		return
	}
	Net.Send(evt.UserData.Username, InventoryAchievementsComposer(achievements))
}

func RegisterLoginHandlers() {
	Player.RegisterLoginHandler(sendInventoryAchievements)
}
