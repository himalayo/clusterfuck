package main

import "math"

func LoginOKComposer() []byte {
	return compose(2491)
}

func UserNoobStatusComposer(isNoob int) []byte {
	return compose(3738, isNoob)
}

func UserPermissionsComposer(clubLevel int, permissionLevel int, hasAmbassador bool) []byte {
	return compose(411, clubLevel, permissionLevel, hasAmbassador)
}

func AvailabilityStatusComposer(isOpen bool, isShuttingDown bool, isAuthenticHabbo bool) []byte {
	return compose(2033, isOpen, isShuttingDown, isAuthenticHabbo)
}

func EnableNotificationsComposer(enabled bool) []byte {
	return compose(3284, enabled)
}

func AchievementScoreComposer(score int) []byte {
	return compose(1968, score)
}

func MysteryBoxComposer() []byte {
	return compose(2833, "", "")
}

func BuildersClubExpiredComposer() []byte {
	return compose(1452, 0, math.MaxInt32, 100, 0, math.MaxInt32)
}

func FavoriteRoomsCountComposer(maxFavoriteRooms int, favoriteRooms []int) []byte {
	return compose(151, maxFavoriteRooms, len(favoriteRooms), serializeValues(favoriteRooms))
}
