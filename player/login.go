package main

import (
	"log"
	"math"
	"time"
)

func RegisterLoginHandlers(e *EventListener) {
	e.RegisterLoginHandler(sendLoginOK)
	e.RegisterLoginHandler(sendUserEffects)
	e.RegisterLoginHandler(sendUserNoobStatus)
	e.RegisterLoginHandler(sendUserPermissions)
	e.RegisterLoginHandler(sendAvailabilityStatus)
	e.RegisterLoginHandler(sendEnableNotifications)
	e.RegisterLoginHandler(sendAchievementScore)
	e.RegisterLoginHandler(sendMysteryBox)
	e.RegisterLoginHandler(sendBuildersClubExpired)
	e.RegisterLoginHandler(sendCfhTopics)
	e.RegisterLoginHandler(sendFavoriteRooms)
	e.RegisterLoginHandler(sendInventoryAchievements)
}

func sendLoginOK(event *LoginEvent) []byte {
	log.Printf("sendLoginOK: %s", event.UserData.AuthTicket)
	return LoginOKComposer()
}

func sendUserNoobStatus(event *LoginEvent) []byte {
	log.Printf("sendUserNoobStatus: %s", event.UserData.AuthTicket)
	return UserNoobStatusComposer(1)
}

func sendUserEffects(event *LoginEvent) []byte {
	log.Printf("sendUserEffects: %s", event.UserData.AuthTicket)
	var packet = shortToBytes(340)
	event.Data.UserEffectsEvent(event.UserData.Id)
	effects := <-event.Data.Effects
	if effects == nil {
		log.Printf("sendUserEffects: sending null")
		return addSize(appendInt(packet, 0))
	}
	packet = appendInt(packet, len(effects))
	for _, userEffect := range effects {
		packet = serializeUserEffect(packet, &userEffect)
	}
	packet = addSize(packet)
	return packet
}

func sendUserPermissions(event *LoginEvent) []byte {
	log.Printf("sendUserPermissions: %s", event.UserData.AuthTicket)
	clubLevel := 0
	if event.Subscription.UserHasSubscription(event.UserData.Id, "HABBO_CLUB") {
		clubLevel = 2
	}
	permissionLevel := int(event.Permission.GetRankLevel(event.UserData.Rank).Level)
	hasAmbassador := event.Permission.GetPermission(event.UserData.Rank, "acc_ambassador").Setting == 1
	log.Printf("sendUserPermissions: %d", permissionLevel)
	return UserPermissionsComposer(clubLevel, permissionLevel, hasAmbassador)
}

func sendAvailabilityStatus(event *LoginEvent) []byte {
	return AvailabilityStatusComposer(true, false, true)
}

func sendEnableNotifications(event *LoginEvent) []byte {
	return EnableNotificationsComposer(true)
}

func sendAchievementScore(event *LoginEvent) []byte {
	return AchievementScoreComposer(event.Data.getAchievementScore(event.UserData.Id))
}

func sendMysteryBox(event *LoginEvent) []byte {
	return MysteryBoxComposer()
}

func sendBuildersClubExpired(event *LoginEvent) []byte {
	return BuildersClubExpiredComposer()
}

func sendCfhTopics(event *LoginEvent) []byte {
	return event.Modtool.CfhTopicsMessageComposer()
}

func sendFavoriteRooms(event *LoginEvent) []byte {
	maxFavoriteRooms, err := event.Configuration.GetInt("hotel.rooms.max.favorite")
	if err != nil {
		log.Printf("sendFavoriteRoomsComposer: %v", err)
		return nil
	}
	favoriteRooms := event.Data.getFavoriteRooms(event.UserData.Id)
	if favoriteRooms == nil {
		favoriteRooms = []int{}
	}
	log.Printf("sendFavoriteRoomsComposer: %d %v", maxFavoriteRooms, favoriteRooms)

	return FavoriteRoomsCountComposer(maxFavoriteRooms, favoriteRooms)
}

func sendInventoryAchievements(event *LoginEvent) []byte {
	packet, err := event.Achievements.InventoryAchievementsComposer()
	if err != nil {
		return nil
	}
	return packet
}

func serializeUserEffect(packet []byte, effect *UserEffect) []byte {
	var out = appendInt(packet, effect.Effect)
	out = appendInt(out, 0)
	if effect.Duration > 0 {
		out = appendInt(out, effect.Duration)
		if effect.ActivationTimestamp >= 0 {
			out = appendInt(out, effect.Total-1)
		} else {
			out = appendInt(out, effect.Total)
		}
	} else {
		out = appendInt(out, math.MaxUint32)
		out = appendInt(out, 0)
	}

	if !(effect.ActivationTimestamp >= 0) && effect.Duration > 0 {
		out = appendInt(out, 0)
	} else {
		if effect.Duration > 0 {
			out = appendInt(out, int(time.Now().Unix())-effect.ActivationTimestamp+effect.Duration)
		} else {
			out = appendInt(out, 0)
		}
	}
	out = appendBool(out, true)
	return out
}
