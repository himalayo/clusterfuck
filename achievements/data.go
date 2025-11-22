
package main

import (
	"database/sql"
	"os"
	"log"
	"strings"
	"cmp"
	"slices"
	"maps"

	"github.com/go-sql-driver/mysql"
	pb "github.com/himalayo/clusterfuck/achievements/proto"
)

type Database struct {
	db *sql.DB
}

func ConfigDatabaseFromEnv() *mysql.Config {
	cfg := mysql.NewConfig()
	cfg.User = os.Getenv("DB_USER")
	cfg.Passwd = os.Getenv("DB_PASS")
	cfg.Net = "tcp"
	cfg.Addr = os.Getenv("DB_ADDR")
	cfg.DBName = os.Getenv("DB_NAME")

	return cfg
}

func ConfigDatabase(user string, password string, address string, name string) *mysql.Config {
	cfg := mysql.NewConfig()
	cfg.User = user
	cfg.Passwd = password
	cfg.Net = "tcp"
	cfg.Addr = address
	cfg.DBName = name

	return cfg
}

func NewDatabase(cfg *mysql.Config) *Database {
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		log.Fatal(err)
	}

	return &Database{db: db}
}

func toCategoryEnum(category string) pb.AchievementCategory {
	return pb.AchievementCategory(pb.AchievementCategory_value[strings.ToUpper(category)])
}

func compareAchievements(a, b pb.Achievement) int {
	return cmp.Compare(a.Id, b.Id)
}

func (data *Database) GetAchievements() ([]pb.Achievement, error) {
	achievements := make(map[string]pb.Achievement)
	rows, err := data.db.Query("SELECT id, name, category, level, reward_amount, reward_type, points, progress_needed FROM achievements")
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var achievement pb.Achievement
		var level pb.AchievementLevel
		var category string
		if err := rows.Scan(&achievement.Id, &achievement.Name, &category, &level.Level, &level.RewardAmount, &level.RewardAmount, &level.Points, &level.Progress); err != nil {
			return nil, err
		}
		achievement.Category = toCategoryEnum(category)
		_, ok := achievements[achievement.Name]
		if !ok {
			achievement.Levels = make(map[int32]*pb.AchievementLevel)
			achievements[achievement.Name] = achievement
		}
		currentAchievement := achievements[achievement.Name]
		currentAchievement.Levels[level.Level] = &level
		achievements[achievement.Name] = currentAchievement
	}
	return slices.SortedFunc(maps.Values(achievements), compareAchievements), nil
}

func (data *Database) GetAchievementByName(name string) (*pb.Achievement, error) {
	var achievement pb.Achievement
	achievement.Levels = make(map[int32]*pb.AchievementLevel)
	rows, err := data.db.Query("SELECT id, name, category, level, reward_amount, reward_type, points, progress_needed FROM achievements WHERE name = ? ORDER BY id ASC", name)
	if err != nil {
		return nil, err
	}
	if rows.Next() {
		var level pb.AchievementLevel
		var category string
		if err := rows.Scan(&achievement.Id, &achievement.Name, &category, &level.Level, &level.RewardAmount, &level.RewardAmount, &level.Points, &level.Progress); err != nil {
			return nil, err
		}
		achievement.Category = toCategoryEnum(category)
		achievement.Levels[level.Level] = &level
	}
	for rows.Next() {
		var tmp pb.Achievement
		var level pb.AchievementLevel
		var category string
		if err := rows.Scan(&tmp.Id, &tmp.Name, &category, &level.Level, &level.RewardAmount, &level.RewardAmount, &level.Points, &level.Progress); err != nil {
			return nil, err
		}
		achievement.Category = toCategoryEnum(category)
		achievement.Levels[level.Level] = &level
	}
	return &achievement, nil
}

func (data *Database) GetAchievement(id int) (*pb.Achievement, error) {
	var name string
	row := data.db.QueryRow("SELECT name FROM achievements WHERE id = ?", id)
	if err := row.Scan(&name); err != nil {
		return nil, err
	}
	return data.GetAchievementByName(name)
}

func compareInventoryAchievements(a, b InventoryAchievement) int {
	return cmp.Compare(a.Id, b.Id)
}

func compareInventoryAchievementsLevels(a, b InventoryAchievementLevel) int {
	return cmp.Compare(a.Level, b.Level)
}

func (data *Database) GetInventoryAchievements() ([]InventoryAchievement, error) {
	achievements := make(map[string]InventoryAchievement)
	rows, err := data.db.Query("SELECT id, name, level, progress_needed FROM achievements")
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var achievement InventoryAchievement
		var level InventoryAchievementLevel
		if err := rows.Scan(&achievement.Id, &achievement.Name, &level.Level, &level.Progress); err != nil {
			return nil, err
		}
		_, ok := achievements[achievement.Name]
		if !ok {
			achievement.Levels = make(map[int]InventoryAchievementLevel)
			achievements[achievement.Name] = achievement
		}
		currentAchievement := achievements[achievement.Name]
		currentAchievement.Levels[level.Level] = level
		achievements[achievement.Name] = currentAchievement
	}
	return slices.SortedFunc(maps.Values(achievements), compareInventoryAchievements), nil
}
