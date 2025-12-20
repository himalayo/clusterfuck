package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"

	netpb "github.com/himalayo/clusterfuck/api/networking/proto"
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

func setUserId(ctx context.Context, evt *pb.LoginEvent) {
	go func() {
		data.rdb.Set(ctx, fmt.Sprintf("user_id:%s", evt.UserData.AuthTicket), evt.UserData.Id, 0)
	}()
}

func loadUserAchievements(ctx context.Context, evt *pb.LoginEvent) {
	go func() {
		data.getUserAchievementsFromDB(ctx, int(evt.UserData.Id))
	}()
}

func sendAchievementsListComposer(ctx context.Context, evt *netpb.PacketEvent) {
	go func() {
		log.Printf("sendAchievementsListComposer: %v", evt)
		user_id_str, err := data.rdb.Get(ctx, fmt.Sprintf("user_id:%s", evt.Packet.ClientId)).Result()
		if err != nil {
			return
		}
		user_id, err := strconv.ParseInt(user_id_str, 10, 32)
		if err != nil {
			return
		}

		user_achievements, _ := data.GetUserAchievements(ctx, int(user_id))
		name_to_user_achievment := make(map[string]*UserAchievement)
		for _, ach := range user_achievements {
			name_to_user_achievment[ach.AchievementName] = ach
		}

		achievements, err := data.GetAchievements(ctx)
		if err != nil {
			return
		}

		packet := appendInt(shortToBytes(305), len(achievements))
		for _, ach := range achievements {
			if ach == nil {
				continue
			}
			user_achievement, ok := name_to_user_achievment[ach.Name]
			var level *AchievementLevel
			if ok {
				level = ach.GetLevelForProgress(user_achievement.Progress)
			}
			var nextLevel *AchievementLevel
			if level != nil {
				nextLevel = ach.GetNextLevel(level.Level)
			} else {
				nextLevel = ach.GetNextLevel(0)
			}
			packet = appendInt(packet, ach.Id)
			lvl := 0
			if nextLevel != nil {
				lvl = nextLevel.Level
			} else {
				if level != nil {
					lvl = level.Level
				}
			}
			packet = appendInt(packet, lvl)
			packet = appendString(packet, fmt.Sprintf("ACH_%s%d", ach.Name, lvl))
			if level != nil {
				packet = appendInt(packet, level.Progress)
			} else {
				packet = appendInt(packet, 0)
			}
			if nextLevel != nil {
				packet = appendInt(packet, nextLevel.Progress)
				packet = appendInt(packet, nextLevel.RewardAmount)
				packet = appendInt(packet, nextLevel.RewardType)
			} else {
				packet = appendInt(packet, -1)
				packet = appendInt(packet, -1)
				packet = appendInt(packet, -1)
			}
			prog := 0
			if user_achievement != nil {
				prog = user_achievement.Progress
			}
			if prog < 0 {
				prog = 0
			}
			packet = appendInt(packet, prog)
			var hasAchieved bool
			if level == nil || user_achievement == nil {
				hasAchieved = false
			} else {
				hasAchieved = nextLevel == nil && user_achievement.Progress >= level.Progress
			}
			packet = appendBool(packet, hasAchieved)
			packet = appendString(packet, strings.ToLower(ach.Category))
			packet = appendString(packet, "")
			packet = appendInt(packet, len(ach.Levels))
			if hasAchieved {
				packet = appendInt(packet, 1)
			} else {
				packet = appendInt(packet, 0)
			}
		}
		packet = appendString(packet, "")
		packet = addSize(packet)

		Net.Send(evt.Packet.ClientId, packet)
	}()
}

func RegisterPacketHandlers() {
	Net.RegisterRedisHandler(219, sendAchievementsListComposer)
}

func RegisterLoginHandlers() {
	Player.RegisterLoginHandler(setUserId)
	Player.RegisterLoginHandler(loadUserAchievements)
	Player.RegisterLoginHandler(sendInventoryAchievements)
}
