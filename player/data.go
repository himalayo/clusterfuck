package main

import (
	"database/sql"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"
)

type UserData struct {
	Id         int
	Username   string
	AuthTicket string
	Look       string
	Motto      string
	HomeRoom   int
	Rank       int
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

func (data *Database) loadUserData(sso string) *UserData {
	log.Printf("Database.loadUserData: loading UserData: %s", sso)
	row := data.db.QueryRow("SELECT `id`, `username`, `auth_ticket`, `look`, `motto`, `home_room`, `rank` FROM users WHERE auth_ticket = ?", sso)
	var curr_data UserData
	if err := row.Scan(&curr_data.Id, &curr_data.Username, &curr_data.AuthTicket, &curr_data.Look, &curr_data.Motto, &curr_data.HomeRoom, &curr_data.Rank); err != nil {
		log.Printf("Database.loadUserData: could not load UserData: %s", err)
		return nil
	}
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
