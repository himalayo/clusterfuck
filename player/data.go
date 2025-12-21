package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"
)

type UserData struct {
	Id         int    `redis:"id"`
	Username   string `redis:"username"`
	AuthTicket string `redis:"auth_ticket"`
	Look       string `redis:"look"`
	Motto      string `redis:"motto"`
	HomeRoom   int    `redis:"home_room"`
	Rank       int    `redis:"rank"`
}

type UserRespectData struct {
	RespectsReceived      int
	RespectsGiven         int
	DailyPetRespectPoints int
}

type UserInfoComposerData struct {
	Id                    int
	Username              string
	AuthTicket            string
	Look                  string
	Motto                 string
	RespectsReceived      int
	RespectsGiven         int
	DailyPetRespectPoints int
	AllowNameChange       bool
}

func (u *UserInfoComposerData) String() string {
	return fmt.Sprintf("{id: %d, username: %s, auth_ticket: %s, look: %s, motto: %s, respects_received: %d, respects_given: %d, daily_pet_respect_points: %d, allow_name_change: %v}", u.Id, u.Username, u.AuthTicket, u.Look, u.Motto, u.RespectsReceived, u.RespectsGiven, u.DailyPetRespectPoints, u.AllowNameChange)
}

func (u *UserData) String() string {
	return fmt.Sprintf("{id: %d, username: %s, auth_ticket: %s, look: %s, motto: %s, home_room: %d}", u.Id, u.Username, u.AuthTicket, u.Look, u.Motto, u.HomeRoom)
}

type UserEffect struct {
	UserId              int
	Effect              int
	Duration            int
	ActivationTimestamp int
	Total               int
}

func (e *UserEffect) String() string {
	return fmt.Sprintf("{user_id: %d, effect: %d, duration: %d, activation_timestamp: %d, total: %d}", e.UserId, e.Effect, e.Duration, e.ActivationTimestamp, e.Total)
}

type Database struct {
	cache            *redis.Client
	db               *sql.DB
	auth             chan string
	Auth             chan *UserData
	effects          chan int
	Effects          chan []UserEffect
	achievement      chan int
	AchievementScore chan int
}

type DatabaseConfig struct {
	sql_config   *mysql.Config
	redis_config *redis.Options
}

func ConfigDatabaseFromEnv() *DatabaseConfig {
	sql_cfg := mysql.NewConfig()
	sql_cfg.User = os.Getenv("DB_USER")
	sql_cfg.Passwd = os.Getenv("DB_PASS")
	sql_cfg.Net = "tcp"
	sql_cfg.Addr = os.Getenv("DB_ADDR")
	sql_cfg.DBName = os.Getenv("DB_NAME")

	redis_port, ok := os.LookupEnv("PLAYER_REDIS_PORT")
	if !ok {
		redis_port = "6379"
	}
	redis_addr, ok := os.LookupEnv("PLAYER_REDIS_ADDR")
	if !ok {
		redis_addr = net.JoinHostPort(os.Getenv("PLAYER_REDIS_HOST"), redis_port)
	}
	password := os.Getenv("PLAYER_REDIS")
	redis_config := &redis.Options{
		Addr:     redis_addr,
		Password: password,
		DB:       0,
	}

	return &DatabaseConfig{
		sql_config:   sql_cfg,
		redis_config: redis_config,
	}
}

func NewDatabase(cfg *DatabaseConfig) *Database {
	db, err := sql.Open("mysql", cfg.sql_config.FormatDSN())
	if err != nil {
		log.Fatal(err)
	}

	rdb := redis.NewClient(cfg.redis_config)

	return &Database{
		cache:            rdb,
		db:               db,
		auth:             make(chan string),
		Auth:             make(chan *UserData),
		effects:          make(chan int),
		Effects:          make(chan []UserEffect),
		AchievementScore: make(chan int),
		achievement:      make(chan int),
	}
}

func (data *Database) AuthTicketEvent(sso string) {
	log.Printf("Database.AuthTicketEvent: %s", sso)
	data.auth <- sso
}

func (data *Database) UserEffectsEvent(id int) {
	log.Printf("Database.UserEffectsEvent: %d", id)
	data.effects <- id
}

func (data *Database) loadUserInfoComposerData(sso string) *UserInfoComposerData {
	var curr_data UserInfoComposerData
	var wg sync.WaitGroup
	wg.Go(func() {
		userData := data.loadUserData(sso)
		curr_data.Id = userData.Id
		curr_data.Username = userData.Username
		curr_data.AuthTicket = userData.AuthTicket
		curr_data.Look = userData.Look
		curr_data.Motto = userData.Motto
	})
	wg.Go(func() {
		respectData := data.loadUserRespectData(sso)
		curr_data.RespectsGiven = respectData.RespectsGiven
		curr_data.RespectsReceived = respectData.RespectsReceived
		curr_data.DailyPetRespectPoints = respectData.DailyPetRespectPoints
	})
	wg.Go(func() {
		canChangeUsername := data.loadUserCanChangeName(sso)
		curr_data.AllowNameChange = canChangeUsername
	})
	wg.Wait()

	return &curr_data
}

func (data *Database) loadUserCanChangeName(_ string) bool {
	return true
}

func (data *Database) loadUserRespectData(sso string) *UserRespectData {
	var out UserRespectData
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	vals, err := data.cache.Exists(ctx, fmt.Sprintf("respect:%s", sso)).Result()
	if err != nil {
		if vals != 0 {
			err := data.cache.HGetAll(ctx, fmt.Sprintf("respect:%s", sso)).Scan(&out)
			if err == nil {
				return &out
			}
		}
	}

	row := data.db.QueryRow("SELECT `respects_received`, `respects_given`, `daily_pet_respect_points` FROM users_settings INNER JOIN users ON users_settings.user_id = users.id WHERE users.auth_ticket = ?", sso)
	if err := row.Scan(&out.RespectsReceived, &out.RespectsGiven, &out.DailyPetRespectPoints); err != nil {
		log.Printf("Database.loadUserRespectData: could not load UserRespectData: %s", err)
		return nil
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		data.cache.HSet(ctx, fmt.Sprintf("respect:%s", sso), out)
	}()

	return &out
}

func (data *Database) getCachedUserCurrencies(ctx context.Context, sso string) (map[int]int, error) {
	m, err := data.cache.HGetAll(ctx, fmt.Sprintf("user_currency:%s", sso)).Result()
	if err != nil {
		return nil, err
	}
	result := make(map[int]int)
	for k, v := range m {
		kInt, err := strconv.ParseInt(k, 10, 32)
		if err != nil {
			continue
		}
		vInt, err := strconv.ParseInt(v, 10, 32)
		if err != nil {
			continue
		}
		result[int(kInt)] = int(vInt)
	}

	return result, nil
}

func (data *Database) GetUserCurrencies(ctx context.Context, sso string) (map[int]int, error) {
	exists, err := data.cache.Exists(ctx, fmt.Sprintf("user_currency:%s", sso)).Result()
	if err != nil {
		return data.loadUserCurrencies(ctx, sso)
	}
	if exists != 0 {
		return data.getCachedUserCurrencies(ctx, sso)
	}
	return data.loadUserCurrencies(ctx, sso)
}

func (data *Database) loadUserCurrencies(ctx context.Context, sso string) (map[int]int, error) {
	rows, err := data.db.QueryContext(ctx, "SELECT `type`, `amount` FROM users_currency INNER JOIN users ON users.id = users_currency.user_id WHERE users.auth_ticket = ?", sso)
	if err != nil {
		log.Printf("data.loadUserCurrenciesById(%s): Got error: %v", sso, err)
		return nil, err
	}

	currencies := make(map[int]int)

	for rows.Next() {
		var currencyType int
		var amount int
		if err := rows.Scan(&currencyType, &amount); err != nil {
			return nil, err
		}
		currencies[currencyType] = amount
		go func() {
			data.cache.HSet(ctx, fmt.Sprintf("user_currency:%s", sso), currencyType, amount)
		}()
	}

	return currencies, nil
}

func (data *Database) loadUserCredits(ctx context.Context, sso string) (int, error) {
	var credits int
	row := data.db.QueryRowContext(ctx, "SELECT `credits` from users where auth_ticket = ?", sso)
	if err := row.Scan(&credits); err != nil {
		return 0, nil
	}
	go func() {
		data.cache.Set(ctx, fmt.Sprintf("user_credits:%s", sso), credits, 0)
	}()
	return credits, nil
}

func (data *Database) loadUserCreditsFromCache(ctx context.Context, sso string) (int, error) {
	credits_str, err := data.cache.Get(ctx, fmt.Sprintf("user_credits:%s", sso)).Result()
	if err != nil {
		return 0, err
	}

	credits, err := strconv.ParseInt(credits_str, 10, 32)
	if err != nil {
		return 0, err
	}

	return int(credits), nil
}

func (data *Database) GetUserCredits(ctx context.Context, sso string) (int, error) {
	exists, err := data.cache.Exists(ctx, fmt.Sprintf("user_credits:%s", sso)).Result()
	if err == nil && exists != 0 {
		return data.loadUserCreditsFromCache(ctx, sso)
	}
	return data.loadUserCredits(ctx, sso)
}

func (data *Database) loadUserData(sso string) *UserData {
	log.Printf("Database.loadUserData: loading UserData: %s", sso)
	var curr_data UserData
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	vals, err := data.cache.Exists(ctx, fmt.Sprintf("user:%s", sso)).Result()
	if err != nil {
		if vals != 0 {
			err := data.cache.HGetAll(ctx, fmt.Sprintf("user:%s", sso)).Scan(&curr_data)
			if err == nil {
				log.Printf("%s", curr_data.String())
				return &curr_data
			}
		}
	}

	row := data.db.QueryRow("SELECT `id`, `username`, `auth_ticket`, `look`, `motto`, `home_room`, `rank` FROM users WHERE auth_ticket = ?", sso)
	if err := row.Scan(&curr_data.Id, &curr_data.Username, &curr_data.AuthTicket, &curr_data.Look, &curr_data.Motto, &curr_data.HomeRoom, &curr_data.Rank); err != nil {
		log.Printf("Database.loadUserData: could not load UserData: %s", err)
		return nil
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		data.cache.HSet(ctx, fmt.Sprintf("user:%s", sso), curr_data)
	}()

	log.Printf("Loaded UserData: %s", curr_data.String())
	return &curr_data
}

func (data *Database) loadUserEffects(id int) []UserEffect {
	log.Printf("Database.loadUserEffects: loading UserEffects for user: %d", id)
	var effects []UserEffect

	rows, err := data.db.Query("SELECT * FROM users_effects WHERE user_id = ?", id)
	if err != nil {
		log.Printf("Database.loadUserEffects: could not load UserEffects: %s", err)
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		var effect UserEffect
		if err := rows.Scan(&effect.UserId, &effect.Effect, &effect.Duration, &effect.ActivationTimestamp, &effect.Total); err != nil {
			log.Printf("Database.loadUserEffects: could not load UserEffect: %s", err)
			return nil
		}
		effects = append(effects, effect)
	}
	if err := rows.Err(); err != nil {
		log.Printf("Database.loadUserEffects: %s", err)
		return nil
	}
	return effects
}

func (data *Database) getAchievementScore(userId int) int {
	var out int
	row := data.db.QueryRow("SELECT achievement_score FROM users_settings WHERE user_id = ? LIMIT 1", userId)
	if err := row.Scan(&out); err != nil {
		return -1
	}
	return out
}

func (data *Database) getFavoriteRooms(userId int) []int {
	favoriteRooms := []int{}
	rows, err := data.db.Query("SELECT room_id FROM users_favorite_rooms WHERE user_id = ?", userId)
	if err != nil {
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		var roomId int
		if err := rows.Scan(&roomId); err != nil {
			log.Printf("Database.getFavoriteRooms: %v", err)
			return nil
		}
		favoriteRooms = append(favoriteRooms, roomId)
	}
	return favoriteRooms
}

func (data *Database) Listen() {
	for {
		select {
		case sso := <-data.auth:
			go func(data *Database) {
				data.Auth <- data.loadUserData(sso)
			}(data)
		case id := <-data.effects:
			go func(data *Database) {
				data.Effects <- data.loadUserEffects(id)
			}(data)
		case id := <-data.achievement:
			go func(data *Database) {
				data.AchievementScore <- data.getAchievementScore(id)
			}(data)
		}
	}
}
