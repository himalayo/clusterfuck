package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/go-sql-driver/mysql"

	pb "github.com/himalayo/clusterfuck/subscription/proto"
)

type Database struct {
	db              *sql.DB
	userId          chan int
	Subscriptions   chan []SubscriptionData
	checkClub       chan *pb.SubscriptionRequest
	setActive       chan *pb.ActivationRequest
	addDuration     chan *pb.DurationRequest
	HasSubscription chan bool
	Active          chan *SubscriptionData
	DurationAdded   chan *SubscriptionData
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

	return &Database{
		db:              db,
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
	Id             int
	UserId         int
	Type           string
	TimestampStart int
	Duration       int
	Active         bool
	data           *Database
}

func NewSubscription(id int, userId int, subscriptionType string, duration int, active bool) *SubscriptionData {
	return &SubscriptionData{Id: id, UserId: userId, Type: subscriptionType, Duration: duration, Active: active}
}

func (sub *SubscriptionData) String() string {
	return fmt.Sprintf("{id: %d, user_id: %d, type: %s, timestamp_start: %d, duration: %d, active: %v}", sub.Id, sub.UserId, sub.Type, sub.TimestampStart, sub.Duration, sub.Active)
}

func (sub *SubscriptionData) LastModified() int {
	if sub.data == nil {
		return int(time.Now().Unix())
	}
	var lastModified int
	row := sub.data.db.QueryRow("SELECT last_modified FROM users_subscriptions_logs WHERE subscription_id = ?", sub.Id)
	if err := row.Scan(&lastModified); err != nil {
		lastModified = int(time.Now().Unix())
		go func(s *SubscriptionData, modified int) {
			err := s.SetLastModified(modified)
			if err != nil {
				log.Printf("SubscriptionData.SetLastModified: %v", err)
			}
		}(sub, lastModified)
	}
	return lastModified
}

func (sub *SubscriptionData) HasLastModified() bool {
	if sub.data == nil {
		return false
	}
	var lastModified int
	row := sub.data.db.QueryRow("SELECT last_modified FROM users_subscriptions_logs WHERE subscription_id = ?", sub.Id)
	if err := row.Scan(&lastModified); err != nil {
		return false
	}
	return true
}

func (sub *SubscriptionData) SetLastModified(modified int) error {
	if sub.data == nil {
		return fmt.Errorf("No database assigned in subscription data object")
	}
	if sub.HasLastModified() {
		_, err := sub.data.db.Exec("UPDATE users_subscriptions_logs SET last_modified = ? WHERE subscription_id = ?", modified, sub.Id)
		return err
	}
	_, err := sub.data.db.Exec("INSERT INTO users_subscriptions_logs (subscription_id, last_modified) VALUES (?, ?)", sub.Id, modified)
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

func (data *Database) GetSubscriptionsForUser(userId int) []SubscriptionData {
	log.Printf("Database.getSubscriptionsForUser: loading subscriptions for user: %d", userId)

	var subscriptions []SubscriptionData
	rows, err := data.db.Query("SELECT * FROM users_subscriptions WHERE user_id = ?", userId)
	if err != nil {
		log.Printf("Database.getSubscriptionsForUser: could not load subscriptions for user %d, reason: %v", userId, err)
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		var subscription SubscriptionData
		if err := rows.Scan(&subscription.Id, &subscription.UserId, &subscription.Type, &subscription.TimestampStart, &subscription.Duration, &subscription.Active); err != nil {
			log.Printf("Database.getSubscriptionsForUser: could not load subscriptions for user %d: reason: %v", userId, err)
			return nil
		}
		subscription.data = data
		subscriptions = append(subscriptions, subscription)
	}
	if err := rows.Err(); err != nil {
		log.Printf("Database.getSubscriptionsForUser: %v", err)
		return nil
	}
	return subscriptions
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
	if b == true {
		return 1
	}
	return 0
}

func (data *Database) GetSubscription(subscriptionId int) *SubscriptionData {
	row := data.db.QueryRow("SELECT * FROM users_subscriptions WHERE id = ?", subscriptionId)
	var subscription SubscriptionData
	if err := row.Scan(&subscription.Id, &subscription.UserId, &subscription.Type, &subscription.TimestampStart, &subscription.Duration, &subscription.Active); err != nil {
		return nil
	}
	subscription.data = data
	return &subscription
}

func (data *Database) GetUserSubscriptionsByType(userId int, t string) ([]SubscriptionData, error) {
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
		subscription.data = data
		subscriptions = append(subscriptions, subscription)
	}
	if err := rows.Err(); err != nil {
		log.Printf("Database.getUserSubscriptionsByType: %v", err)
		return nil, err
	}
	return subscriptions, nil
}

func (data *Database) SetSubscriptionActive(subscriptionId int, act bool) *SubscriptionData {
	_, err := data.db.Exec("UPDATE users_subscriptions SET active = ? WHERE id = ? LIMIT 1", boolToInt(act), subscriptionId)
	if err != nil {
		return nil
	}
	return data.GetSubscription(subscriptionId)
}

func (data *Database) AddDuration(subscriptionId int, amount int) *SubscriptionData {
	subscription := data.GetSubscription(subscriptionId)
	subscription.Duration += amount
	_, err := data.db.Exec("UPDATE users_subscriptions SET duration = ? WHERE id = ? LIMIT 1", subscription.Duration, subscriptionId)
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
				data.Subscriptions <- data.GetSubscriptionsForUser(id)
			}(data)
		case req := <-data.checkClub:
			go func(data *Database) {
				data.HasSubscription <- data.UserHasClub(int(req.UserId), req.SubscriptionType)
			}(data)
		case req := <-data.setActive:
			go func(data *Database) {
				data.Active <- data.SetSubscriptionActive(int(req.SubscriptionId), req.Active)
			}(data)
		case req := <-data.addDuration:
			go func(data *Database) {
				data.DurationAdded <- data.AddDuration(int(req.SubscriptionId), int(req.Duration))
			}(data)
		}
	}
}
