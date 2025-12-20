package main

import (
	"cmp"
	"context"
	"database/sql"
	"fmt"
	"log"
	"maps"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/go-sql-driver/mysql"
	pb "github.com/himalayo/clusterfuck/api/achievements/proto"
	"github.com/redis/go-redis/v9"
)

type Database struct {
	rdb *redis.Client
	db  *sql.DB
}

type DatabaseConfig struct {
	sql_config   *mysql.Config
	redis_config *redis.Options
}

func ConfigDatabaseFromEnv() *DatabaseConfig {
	cfg := mysql.NewConfig()
	cfg.User = os.Getenv("DB_USER")
	cfg.Passwd = os.Getenv("DB_PASS")
	cfg.Net = "tcp"
	cfg.Addr = os.Getenv("DB_ADDR")
	cfg.DBName = os.Getenv("DB_NAME")
	redis_config := &redis.Options{
		Addr:     os.Getenv("ACHIEVEMENTS_REDIS_ADDR"),
		Password: os.Getenv("ACHIEVEMENTS_REDIS_PASSWORD"),
		DB:       0,
	}

	return &DatabaseConfig{
		sql_config:   cfg,
		redis_config: redis_config,
	}
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

func NewDatabase(cfg *DatabaseConfig) *Database {
	db, err := sql.Open("mysql", cfg.sql_config.FormatDSN())
	if err != nil {
		log.Fatal(err)
	}

	return &Database{db: db, rdb: redis.NewClient(cfg.redis_config)}
}

func toCategoryEnum(category string) pb.AchievementCategory {
	return pb.AchievementCategory(pb.AchievementCategory_value[strings.ToUpper(category)])
}

func compareAchievements(a, b *Achievement) int {
	return cmp.Compare(a.Id, b.Id)
}

type RedisAchievement struct {
	Id           int    `redis:"id"`
	Name         string `redis:"name"`
	Category     string `redis:"category"`
	Level        int    `redis:"level"`
	RewardAmount int    `redis:"reward_amount"`
	RewardType   int    `redis:"reward_type"`
	Points       int    `redis:"points"`
	Progress     int    `redis:"progress_needed"`
}

type AchievementLevel struct {
	Level        int
	RewardAmount int
	RewardType   int
	Points       int
	Progress     int
}

func (lvl *AchievementLevel) ToProto() *pb.AchievementLevel {
	return &pb.AchievementLevel{
		Level:        int32(lvl.Level),
		RewardAmount: int32(lvl.RewardAmount),
		RewardType:   int32(lvl.RewardType),
		Points:       int32(lvl.Points),
		Progress:     int32(lvl.Progress),
	}
}

type Achievement struct {
	Id       int
	Name     string
	Category string
	Levels   map[int]*AchievementLevel
}

func (ach *Achievement) ToProto() *pb.Achievement {
	lvls := make(map[int32]*pb.AchievementLevel)
	for k, v := range ach.Levels {
		lvls[int32(k)] = v.ToProto()
	}
	return &pb.Achievement{
		Id:       int32(ach.Id),
		Name:     ach.Name,
		Category: toCategoryEnum(ach.Category),
		Levels:   lvls,
	}
}

func (ach *Achievement) GetLevelForProgress(progress int) *AchievementLevel {
	var out *AchievementLevel
	if progress > 0 {
		for _, lvl := range ach.Levels {
			if progress >= lvl.Progress {
				if out != nil {
					if out.Level > lvl.Level {
						continue
					}
				}
				out = lvl
			}
		}
	}
	return out
}

func (ach *Achievement) GetNextLevel(level int) *AchievementLevel {
	for _, lvl := range ach.Levels {
		if lvl.Level == level+1 {
			return lvl
		}
	}
	return nil
}

func (data *Database) getAchievementsFromDB() ([]*Achievement, error) {
	achievements := make(map[string]*Achievement)
	rows, err := data.db.Query("SELECT id, name, category, level, reward_amount, reward_type, points, progress_needed FROM achievements")
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var achievement Achievement
		var level AchievementLevel
		var category string
		if err := rows.Scan(&achievement.Id, &achievement.Name, &category, &level.Level, &level.RewardAmount, &level.RewardType, &level.Points, &level.Progress); err != nil {
			return nil, err
		}

		go func() {
			key := fmt.Sprintf("achievement:%d", achievement.Id)
			data.rdb.HSet(context.Background(), key, RedisAchievement{
				Id:           achievement.Id,
				Name:         achievement.Name,
				Category:     category,
				Level:        level.Level,
				RewardAmount: level.RewardAmount,
				RewardType:   level.RewardType,
				Points:       level.Points,
				Progress:     level.Progress,
			}).Result()
			data.rdb.SAdd(context.Background(), fmt.Sprintf("achievement_keys_set:%s", achievement.Name), key)
		}()

		achievement.Category = category
		_, ok := achievements[achievement.Name]
		if !ok {
			achievement.Levels = make(map[int]*AchievementLevel)
			achievements[achievement.Name] = &achievement
		}
		currentAchievement := achievements[achievement.Name]
		currentAchievement.Levels[level.Level] = &level
		achievements[achievement.Name] = currentAchievement
	}

	return slices.SortedFunc(maps.Values(achievements), compareAchievements), nil
}
func (data *Database) getRedisAchievementInstanceById(ctx context.Context, id int) (*RedisAchievement, error) {
	return data.getRedisAchievementInstanceFromKey(ctx, fmt.Sprintf("achievement:%d", id))
}

func (data *Database) getRedisAchievementInstanceFromKey(ctx context.Context, key string) (out *RedisAchievement, err error) {
	err = data.rdb.HGetAll(ctx, key).Scan(out)
	return
}

func (data *Database) getCachedAchievementByName(ctx context.Context, name string) (*Achievement, error) {
	keys, err := data.rdb.SMembers(ctx, fmt.Sprintf("achievement_keys_set:%s", name)).Result()
	if err != nil {
		return nil, err
	}
	var out Achievement
	for _, key := range keys {
		ach, err := data.getRedisAchievementInstanceFromKey(ctx, key)
		if err != nil {
			continue
		}

		if ach.Name != out.Name {
			continue
		}

		level := AchievementLevel{
			Level:        ach.Level,
			RewardAmount: ach.RewardAmount,
			RewardType:   ach.RewardType,
			Points:       ach.Points,
			Progress:     ach.Progress,
		}

		if ach.Id < out.Id {
			out.Id = ach.Id
		}
		out.Levels[level.Level] = &level
	}

	return &out, nil
}

func (data *Database) getCachedAchievementById(ctx context.Context, id int) (*Achievement, error) {
	ins, err := data.getRedisAchievementInstanceById(ctx, id)
	if err != nil {
		return nil, err
	}
	return data.getCachedAchievementByName(ctx, ins.Name)
}

func (data *Database) getAchievementsFromCache() ([]*Achievement, error) {
	achievements := make(map[string]*Achievement)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	var cursor uint64
	var err error
	var wg sync.WaitGroup
	for {
		ch := make(chan *RedisAchievement)
		var keysFromScan []string
		keysFromScan, cursor, err = data.rdb.ScanType(ctx, cursor, "achievement:*", 10, "hash").Result()
		if err != nil {
			break
		}
		for i := range keysFromScan {
			wg.Go(func() {
				res, err := data.getRedisAchievementInstanceFromKey(ctx, keysFromScan[i])
				if err == nil {
					log.Printf("getAchievementsFromCache(): %d %s", res.Id, res.Name)
					ch <- res
				}
			})
		}

		go func() {
			wg.Wait()
			close(ch)
		}()

		for ach := range ch {
			_, ok := achievements[ach.Name]
			if !ok {
				achievements[ach.Name] = &Achievement{
					Id:       ach.Id,
					Name:     ach.Name,
					Category: ach.Category,
					Levels:   make(map[int]*AchievementLevel),
				}
			}

			if ach.Id < achievements[ach.Name].Id {
				achievements[ach.Name].Id = ach.Id
			}

			level := &AchievementLevel{
				Level:        ach.Level,
				RewardAmount: ach.RewardAmount,
				RewardType:   ach.RewardType,
				Points:       ach.Points,
				Progress:     ach.Progress,
			}
			achievements[ach.Name].Levels[ach.Level] = level
		}

		if cursor == 0 {
			break
		}
	}

	return slices.SortedFunc(maps.Values(achievements), compareAchievements), nil
}

type UserAchievement struct {
	Id              int    `redis:"id"`
	UserId          int    `redis:"user_id"`
	AchievementName string `redis:"achievement_name"`
	Progress        int    `redis:"progress"`
}

func (data *Database) getUserAchievementFromRedis(ctx context.Context, key string) (*UserAchievement, error) {
	var out UserAchievement
	err := data.rdb.HGetAll(ctx, key).Scan(&out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (data *Database) getUserAchievementsFromDB(ctx context.Context, userId int) ([]*UserAchievement, error) {
	achievements := make([]*UserAchievement, 0)

	rows, err := data.db.Query("SELECT user_id, achievement_name, progress FROM users_achievements WHERE user_id = ?", userId)
	if err != nil {
		return nil, err
	}

	count := 0
	for rows.Next() {
		var achievement UserAchievement
		if err := rows.Scan(&achievement.UserId, &achievement.AchievementName, &achievement.Progress); err != nil {
			return nil, err
		}
		achievement.Id = count
		go func() {
			data.rdb.HSet(ctx, fmt.Sprintf("user_achievements:%d:%d", achievement.UserId, achievement.Id), achievement).Result()
		}()
		achievements = append(achievements, &achievement)
		count++
	}
	return achievements, nil
}

func (data *Database) getUserAchievementsFromCache(ctx context.Context, userId int) ([]*UserAchievement, error) {
	out := make([]*UserAchievement, 0)
	var cursor uint64
	var err error
	var wg sync.WaitGroup
	for {
		ch := make(chan *UserAchievement)
		var keysFromScan []string
		keysFromScan, cursor, err = data.rdb.ScanType(ctx, cursor, fmt.Sprintf("user_achievements:%d:*", userId), 10, "hash").Result()
		if err != nil {
			break
		}

		for i := range keysFromScan {
			wg.Go(func() {
				ach, err := data.getUserAchievementFromRedis(ctx, keysFromScan[i])
				if err == nil {
					ch <- ach
				}
			})
		}

		go func() {
			wg.Wait()
			close(ch)
		}()

		for ach := range ch {
			out = append(out, ach)
		}

		if cursor == 0 {
			break
		}
	}

	return out, nil
}

func (data *Database) GetUserAchievements(ctx context.Context, userId int) ([]*UserAchievement, error) {
	exists, err := data.rdb.Exists(ctx, fmt.Sprintf("user_achievements:%d:0", userId)).Result()
	if err == nil {
		if exists != 0 {
			ach, err := data.getUserAchievementsFromCache(ctx, userId)
			if err != nil {
				return data.getUserAchievementsFromDB(ctx, userId)
			}
			return ach, nil
		}
	}
	return data.getUserAchievementsFromDB(ctx, userId)
}

func (data *Database) GetAchievements(ctx context.Context) ([]*Achievement, error) {
	out, err := data.getAchievementsFromCache()
	if err != nil {
		return data.getAchievementsFromDB()
	}
	if len(out) == 0 {
		return data.getAchievementsFromDB()
	}
	return out, nil
}

func (data *Database) GetAchievementByName(name string) (*pb.Achievement, error) {
	exists, err := data.rdb.Exists(context.Background(), fmt.Sprintf("achievement_key_set:%s", name)).Result()
	if err == nil {
		if exists != 0 {
			ach, err := data.getCachedAchievementByName(context.Background(), name)
			if err == nil {
				return ach.ToProto(), nil
			}
		}
	}

	var achievement pb.Achievement
	achievement.Levels = make(map[int32]*pb.AchievementLevel)
	rows, err := data.db.Query("SELECT id, name, category, level, reward_amount, reward_type, points, progress_needed FROM achievements WHERE name = ? ORDER BY id ASC", name)
	if err != nil {
		return nil, err
	}
	if rows.Next() {
		var level pb.AchievementLevel
		var category string
		if err := rows.Scan(&achievement.Id, &achievement.Name, &category, &level.Level, &level.RewardAmount, &level.RewardType, &level.Points, &level.Progress); err != nil {
			return nil, err
		}
		go func() {
			key := fmt.Sprintf("achievement:%d", achievement.Id)
			data.rdb.HSet(context.Background(), key, RedisAchievement{
				Id:           int(achievement.Id),
				Name:         achievement.Name,
				Category:     category,
				Level:        int(level.Level),
				RewardAmount: int(level.RewardAmount),
				RewardType:   int(level.RewardType),
				Points:       int(level.Points),
				Progress:     int(level.Progress),
			}).Result()
			data.rdb.SAdd(context.Background(), fmt.Sprintf("achievement_keys_set:%s", achievement.Name), key)
		}()
		achievement.Category = toCategoryEnum(category)
		achievement.Levels[level.Level] = &level
	}
	for rows.Next() {
		var tmp pb.Achievement
		var level pb.AchievementLevel
		var category string
		if err := rows.Scan(&tmp.Id, &tmp.Name, &category, &level.Level, &level.RewardAmount, &level.RewardType, &level.Points, &level.Progress); err != nil {
			return nil, err
		}

		go func() {
			key := fmt.Sprintf("achievement:%d", tmp.Id)
			data.rdb.HSet(context.Background(), key, RedisAchievement{
				Id:           int(tmp.Id),
				Name:         achievement.Name,
				Category:     category,
				Level:        int(level.Level),
				RewardAmount: int(level.RewardAmount),
				RewardType:   int(level.RewardType),
				Points:       int(level.Points),
				Progress:     int(level.Progress),
			}).Result()
			data.rdb.SAdd(context.Background(), fmt.Sprintf("achievement_keys_set:%s", achievement.Name), key)
		}()

		achievement.Category = toCategoryEnum(category)
		achievement.Levels[level.Level] = &level
	}
	return &achievement, nil
}

func (data *Database) GetAchievement(id int) (*pb.Achievement, error) {
	exists, err := data.rdb.Exists(context.Background(), fmt.Sprintf("achievement:%d", id)).Result()
	if err == nil {
		if exists != 0 {
			ach, err := data.getCachedAchievementById(context.Background(), id)
			if err == nil {
				return ach.ToProto(), nil
			}
		}
	}
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
