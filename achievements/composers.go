package main

import (
	"maps"
	"slices"
)

type Serializeable interface {
	Serialize() []byte
}

func SerializeAll[S Serializeable](values []S) []byte {
	data := integerToBytes(len(values))
	for _, value := range values {
		data = append(data, value.Serialize()...)
	}
	return data
}

type InventoryAchievementLevel struct {
	Level int
	Progress int
}

func (level InventoryAchievementLevel) Serialize() []byte {
	return serializeValues(level.Level, level.Progress)
}

type InventoryAchievement struct {
	Id int
	Name string
	Levels map[int]InventoryAchievementLevel
}



func (achievement InventoryAchievement) Serialize() []byte {
	return serializeValues(achievement.Name, SerializeAll(slices.SortedFunc(maps.Values(achievement.Levels), compareInventoryAchievementsLevels)))
}

func InventoryAchievementsComposer(achievements []InventoryAchievement) []byte {
	return compose(2501, SerializeAll(achievements))
}
