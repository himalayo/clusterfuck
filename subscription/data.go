package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"

	pb "github.com/himalayo/clusterfuck/api/subscription/proto"
)

type Database struct {
	db              *sql.DB
	rdb             *redis.Client
	userId          chan int
	Subscriptions   chan []SubscriptionData
	checkClub       chan *pb.SubscriptionRequest
	setActive       chan *pb.ActivationRequest
	addDuration     chan *pb.DurationRequest
	HasSubscription chan bool
	Active          chan *SubscriptionData
	DurationAdded   chan *SubscriptionData
}

type DatabaseConfig struct {
	mysql_conf *mysql.Config
	redis_conf *redis.Options
}

func ConfigDatabaseFromEnv() *DatabaseConfig {
	cfg := mysql.NewConfig()
	cfg.User = os.Getenv("DB_USER")
	cfg.Passwd = os.Getenv("DB_PASS")
	cfg.Net = "tcp"
	cfg.Addr = os.Getenv("DB_ADDR")
	cfg.DBName = os.Getenv("DB_NAME")

	return &DatabaseConfig{
		mysql_conf: cfg,
		redis_conf: &redis.Options{
			Addr:     os.Getenv("SUBSCRIPTION_REDIS_ADDR"),
			Password: os.Getenv("SUBSCRIPTION_REDIS_PASSWORD"),
			DB:       0,
		},
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
	db, err := sql.Open("mysql", cfg.mysql_conf.FormatDSN())
	if err != nil {
		log.Fatal(err)
	}
	rdb := redis.NewClient(cfg.redis_conf)

	return &Database{
		db:              db,
		rdb:             rdb,
		userId:          make(chan int),
		Subscriptions:   make(chan []SubscriptionData),
		checkClub:       make(chan *pb.SubscriptionRequest),
		HasSubscription: make(chan bool),
		setActive:       make(chan *pb.ActivationRequest),
		Active:          make(chan *SubscriptionData),
		addDuration:     make(chan *pb.DurationRequest),
		DurationAdded:   make(chan *SubscriptionData),
	}
}

type SubscriptionData struct {
	Id             int    `redis:"id"`
	UserId         int    `redis:"user_id"`
	Type           string `redis:"type"`
	TimestampStart int    `redis:"timestamp_start"`
	Duration       int    `redis:"duration"`
	Active         bool   `redis:"active"`
}

func NewSubscription(id int, userId int, subscriptionType string, duration int, active bool) *SubscriptionData {
	return &SubscriptionData{Id: id, UserId: userId, Type: subscriptionType, Duration: duration, Active: active}
}

func (sub *SubscriptionData) String() string {
	return fmt.Sprintf("{id: %d, user_id: %d, type: %s, timestamp_start: %d, duration: %d, active: %v}", sub.Id, sub.UserId, sub.Type, sub.TimestampStart, sub.Duration, sub.Active)
}

func (sub *SubscriptionData) LastModified(ctx context.Context) int {
	if sub == nil {
		return int(time.Now().Unix())
	}
	return data.GetSubscriptionLastModified(ctx, sub.Id)
}

func (sub *SubscriptionData) HasLastModified(ctx context.Context) bool {
	if sub == nil {
		return false
	}
	return data.GetSubscriptionHasLastModified(ctx, sub.Id)
}

var ErrNilSubscription = errors.New("subscription is nil")

func (sub *SubscriptionData) SetLastModified(ctx context.Context, modified int) error {
	if sub == nil {
		return ErrNilSubscription
	}
	return data.SetSubscriptionLastModified(ctx, sub.Id, modified)
}

func (data *Database) GetSubscriptionHasLastModified(ctx context.Context, subscriptionId int) bool {
	exists, err := data.rdb.Exists(ctx, fmt.Sprintf("subscription_last_modified:%d", subscriptionId)).Result()
	if err != nil {
		return false
	}
	return exists != 0
}

type SubscriptionModified struct {
	SubscriptionId int `redis:"subscription_id"`
	LastModified   int `redis:"last_modified"`
}

func (data *Database) GetSubscriptionLastModified(ctx context.Context, subscriptionId int) int {
	var lastModified SubscriptionModified
	lastModified.LastModified = int(time.Now().Unix())
	exists, err := data.rdb.Exists(ctx, fmt.Sprintf("subscription_last_modified:%d", subscriptionId)).Result()
	if err != nil {
		return int(time.Now().Unix())
	}
	if exists != 0 {
		err := data.rdb.HGetAll(ctx, fmt.Sprintf("subscription_last_modified:%d", subscriptionId)).Scan(&lastModified)
		if err != nil {
			lastModified.LastModified = int(time.Now().Unix())
			data.SetSubscriptionLastModified(ctx, subscriptionId, int(time.Now().Unix()))
			return lastModified.LastModified
		}
	} else {
		lastModified.LastModified = int(time.Now().Unix())
		data.SetSubscriptionLastModified(ctx, subscriptionId, int(time.Now().Unix()))
		return lastModified.LastModified
	}
	return lastModified.LastModified
}

func (data *Database) SetSubscriptionLastModified(ctx context.Context, subscriptionId int, modified int) error {
	_, err := data.rdb.HSet(ctx, fmt.Sprintf("subscription_last_modified:%d", subscriptionId), SubscriptionModified{
		SubscriptionId: subscriptionId,
		LastModified:   modified,
	}).Result()
	return err
}

func (sub *SubscriptionData) toSubscriptionInstance() *pb.SubscriptionInstance {
	return &pb.SubscriptionInstance{
		Id:               int32(sub.Id),
		UserId:           int32(sub.UserId),
		SubscriptionType: sub.Type,
		TimestampStart:   int32(sub.TimestampStart),
		Duration:         int32(sub.Duration),
		Active:           sub.Active,
	}
}

func (data *Database) getUserSubscriptionsFromCache(ctx context.Context, userId int) ([]SubscriptionData, error) {
	out := make([]SubscriptionData, 0)
	var cursor uint64
	var err error
	for {
		var keysFromScan []string
		keysFromScan, cursor, err = data.rdb.ScanType(ctx, cursor, fmt.Sprintf("user_subscription:%d:*", userId), 100, "hash").Result()
		if err != nil {
			log.Printf("Database.getUserSubsriptionsFromCache: Got error: %v", err)
			break
		}

		pipe := data.rdb.Pipeline()

		for i := range keysFromScan {
			pipe.HGetAll(ctx, keysFromScan[i])
		}
		cmds, err := pipe.Exec(ctx)
		if err != nil {
			log.Printf("Database.getUserSubsriptionsFromCache: Got error: %v", err)
			continue
		}

		for _, cmd := range cmds {
			var sub SubscriptionData
			cmd.(*redis.MapStringStringCmd).Scan(&sub)
			out = append(out, sub)
		}

		if cursor == 0 {
			break
		}
	}

	return out, nil
}

func (data *Database) loadUserSubscriptionsFromDB(ctx context.Context, userId int) ([]SubscriptionData, error) {
	log.Printf("Database.getSubscriptionsForUser: loading subscriptions for user: %d", userId)

	var subscriptions []SubscriptionData
	rows, err := data.db.QueryContext(ctx, "SELECT * FROM users_subscriptions WHERE user_id = ?", userId)
	if err != nil {
		log.Printf("Database.getSubscriptionsForUser: could not load subscriptions for user %d, reason: %v", userId, err)
		return nil, err
	}
	defer rows.Close()
	count := 0
	typeCount := make(map[string]int)
	var typeMu sync.Mutex
	pipe := data.rdb.Pipeline()
	for rows.Next() {
		var subscription SubscriptionData
		if err := rows.Scan(&subscription.Id, &subscription.UserId, &subscription.Type, &subscription.TimestampStart, &subscription.Duration, &subscription.Active); err != nil {
			log.Printf("Database.getSubscriptionsForUser: could not load subscriptions for user %d: reason: %v", userId, err)
			return nil, err
		}
		c := count
		go func() {
			key := fmt.Sprintf("user_subscription:%d:%d", userId, c)
			pipe.HSet(ctx, key, subscription).Result()
			pipe.Set(ctx, fmt.Sprintf("user_subscription_keys:%d", subscription.Id), key, 0).Result()
			typeMu.Lock()
			tc, ok := typeCount[subscription.Type]
			if !ok {
				typeCount[subscription.Type] = 0
				tc = 0
			}
			pipe.Set(ctx, fmt.Sprintf("user_subscription_type:%s:%d:%d", subscription.Type, userId, tc), key, 0)
			typeCount[subscription.Type]++
			typeMu.Unlock()
		}()
		count++
		subscriptions = append(subscriptions, subscription)
	}
	pipe.Exec(ctx)
	if err := rows.Err(); err != nil {
		log.Printf("Database.getSubscriptionsForUser: %v", err)
		return nil, err
	}
	return subscriptions, nil
}

func (data *Database) GetSubscriptionsForUser(ctx context.Context, userId int) ([]SubscriptionData, error) {
	exists, err := data.rdb.Exists(ctx, fmt.Sprintf("user_subscription:%d:0", userId)).Result()
	if err != nil || exists == 0 {
		return data.loadUserSubscriptionsFromDB(ctx, userId)
	}
	return data.getUserSubscriptionsFromCache(ctx, userId)
}

func (data *Database) UserHasClub(userId int, subscriptionType string) bool {
	log.Printf("Database.userHasClub: checking: %d", userId)
	row := data.db.QueryRow("SELECT * FROM users_subscriptions WHERE user_id = ? AND subscription_type = ? AND active = ?", userId, subscriptionType, 1)
	var subscription SubscriptionData
	if err := row.Scan(&subscription.Id, &subscription.UserId, &subscription.Type, &subscription.TimestampStart, &subscription.Duration, &subscription.Active); err != nil {
		log.Printf("%s", err)
		return false
	}
	return true
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (data *Database) GetSubscription(ctx context.Context, subscriptionId int) *SubscriptionData {
	var subscription SubscriptionData
	key_query := fmt.Sprintf("user_subscription_keys:%d", subscriptionId)
	exists, err := data.rdb.Exists(ctx, key_query).Result()
	if err == nil && exists != 0 {
		key, err := data.rdb.Get(ctx, key_query).Result()
		if err == nil {
			err = data.rdb.HGetAll(ctx, key).Scan(&subscription)
			if err == nil {
				return &subscription
			}
		}
	}
	row := data.db.QueryRow("SELECT * FROM users_subscriptions WHERE id = ?", subscriptionId)
	if err := row.Scan(&subscription.Id, &subscription.UserId, &subscription.Type, &subscription.TimestampStart, &subscription.Duration, &subscription.Active); err != nil {
		return nil
	}
	go data.loadUserSubscriptionsFromDB(ctx, subscription.UserId)
	return &subscription
}

func (data *Database) getUserSubscriptionsByTypeFromCache(ctx context.Context, userId int, t string) ([]SubscriptionData, error) {
	out := make([]SubscriptionData, 0)
	var cursor uint64
	var err error
	for {
		var keysFromScan []string
		keys := []string{}
		keysFromScan, cursor, err = data.rdb.Scan(ctx, cursor, fmt.Sprintf("user_subscription_type:%s:%d:*", t, userId), 100).Result()
		if err != nil {
			log.Printf("Database.getUserSubsriptionsFromCache: Got error: %v", err)
			break
		}

		pipe := data.rdb.Pipeline()

		for i := range keysFromScan {
			pipe.Get(ctx, keysFromScan[i])
		}
		cmds, err := pipe.Exec(ctx)
		if err != nil {
			log.Printf("Database.getUserSubsriptionsFromCache: Got error: %v", err)
			continue
		}

		for _, cmd := range cmds {
			key, err := cmd.(*redis.StringCmd).Result()
			if err == nil {
				keys = append(keys, key)
			}
		}

		pipe = data.rdb.Pipeline()

		for _, key := range keys {
			pipe.HGetAll(ctx, key)
		}

		cmds, err = pipe.Exec(ctx)
		if err != nil {
			log.Printf("Database.getUserSubsriptionsFromCache: Got error: %v", err)
			continue
		}

		for _, cmd := range cmds {
			var sub SubscriptionData
			cmd.(*redis.MapStringStringCmd).Scan(&sub)
			out = append(out, sub)
		}

		if cursor == 0 {
			break
		}
	}

	return out, nil
}

func (data *Database) GetUserSubscriptionsByType(ctx context.Context, userId int, t string) ([]SubscriptionData, error) {
	exists, err := data.rdb.Exists(ctx, fmt.Sprintf("user_subscription_type:%s:0", t)).Result()
	if err == nil {
		if exists != 0 {
			subs, err := data.getUserSubscriptionsByTypeFromCache(ctx, userId, t)
			if err == nil && len(subs) > 0 {
				return subs, nil
			}
		}
	}

	var subscriptions []SubscriptionData
	rows, err := data.db.Query("SELECT id, user_id, subscription_type, timestamp_start, duration, active FROM users_subscriptions WHERE user_id = ? AND subscription_type = ?", userId, t)
	if err != nil {
		log.Printf("Database.getSubscriptionsForUser: could not load subscriptions for user %d, reason: %v", userId, err)
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var subscription SubscriptionData
		if err := rows.Scan(&subscription.Id, &subscription.UserId, &subscription.Type, &subscription.TimestampStart, &subscription.Duration, &subscription.Active); err != nil {
			log.Printf("Database.getUserSubscriptionsByType: could not load subscriptions for user %d: reason: %v", userId, err)
			return nil, err
		}
		subscriptions = append(subscriptions, subscription)
	}
	if err := rows.Err(); err != nil {
		log.Printf("Database.getUserSubscriptionsByType: %v", err)
		return nil, err
	}
	return subscriptions, nil
}

func (data *Database) SetSubscriptionActive(ctx context.Context, subscriptionId int, act bool) *SubscriptionData {
	key_query := fmt.Sprintf("user_subscription_keys:%d", subscriptionId)
	exists, err := data.rdb.Exists(ctx, key_query).Result()
	if err == nil && exists != 0 {
		key, err := data.rdb.Get(ctx, key_query).Result()
		if err == nil {
			data.rdb.HSet(ctx, key, "active", act)
		}
	}

	_, err = data.db.Exec("UPDATE users_subscriptions SET active = ? WHERE id = ? LIMIT 1", boolToInt(act), subscriptionId)
	if err != nil {
		return nil
	}
	return data.GetSubscription(ctx, subscriptionId)
}

func (data *Database) AddDuration(ctx context.Context, subscriptionId int, amount int) *SubscriptionData {
	subscription := data.GetSubscription(ctx, subscriptionId)
	if subscription == nil {
		return nil
	}
	subscription.Duration += amount
	key_query := fmt.Sprintf("user_subscription_keys:%d", subscriptionId)
	exists, err := data.rdb.Exists(ctx, key_query).Result()
	if err == nil && exists != 0 {
		key, err := data.rdb.Get(ctx, key_query).Result()
		if err == nil {
			data.rdb.HSet(ctx, key, subscription)
		}
	} else if exists == 0 {
		go data.loadUserSubscriptionsFromDB(ctx, subscription.UserId)
	}
	_, err = data.db.Exec("UPDATE users_subscriptions SET duration = ? WHERE id = ? LIMIT 1", subscription.Duration, subscriptionId)
	if err != nil {
		return nil
	}
	return subscription
}

func (data *Database) GetSubscriptions(userId int) {
	data.userId <- userId
}

func (data *Database) hasClub(req *pb.SubscriptionRequest) {
	data.checkClub <- req
}

func (data *Database) activationRequest(req *pb.ActivationRequest) {
	data.setActive <- req
}

func (data *Database) durationRequest(req *pb.DurationRequest) {
	data.addDuration <- req
}

func (data *Database) Listen() {
	for {
		select {
		case id := <-data.userId:
			go func(data *Database) {
				sub, err := data.GetSubscriptionsForUser(context.Background(), id)
				if err != nil {
					data.Subscriptions <- nil
					return
				}
				data.Subscriptions <- sub
			}(data)
		case req := <-data.checkClub:
			go func(data *Database) {
				data.HasSubscription <- data.UserHasClub(int(req.UserId), req.SubscriptionType)
			}(data)
		case req := <-data.setActive:
			go func(data *Database) {
				data.Active <- data.SetSubscriptionActive(context.Background(), int(req.SubscriptionId), req.Active)
			}(data)
		case req := <-data.addDuration:
			go func(data *Database) {
				data.DurationAdded <- data.AddDuration(context.Background(), int(req.SubscriptionId), int(req.Duration))
			}(data)
		}
	}
}
