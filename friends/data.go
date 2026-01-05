package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"

	"github.com/go-sql-driver/mysql"
	"github.com/himalayo/clusterfuck/api/permission/proto"
	"github.com/redis/go-redis/v9"
)

type Database struct {
	db  *sql.DB
	rdb *redis.Client
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
			Addr:     os.Getenv("FRIENDS_REDIS_ADDR"),
			Password: os.Getenv("FRIENDS_REDIS_PASSWORD"),
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
		db:  db,
		rdb: rdb,
	}
}

type Friend struct {
	Id         int    `redis:"id"`
	Username   string `redis:"username"`
	Gender     string `redis:"gender"`
	Online     int    `redis:"online"`
	Look       string `redis:"look"`
	Motto      string `redis:"motto"`
	Relation   int    `redis:"relation"`
	CategoryId int    `redis:"category_id"`
	UserId     int    `redis:"user_id"`
	InRoom     bool   `redis:"in_room"`
}

func (f *Friend) genderToInt() int {
	if f.Gender == "M" {
		return 0
	}
	return 1
}

func (f *Friend) getLookIfOnline() string {
	if f.Online == 1 {
		return f.Look
	}
	return ""
}

func (f *Friend) Serialize() []byte {
	return serializeValues(f.Id, f.Username, f.genderToInt(), f.Online == 1, f.InRoom, f.getLookIfOnline(), f.CategoryId, f.Motto, "", "", false, false, false, f.Relation)
}

func (data *Database) getUserId(ctx context.Context, sso string) (*int, error) {
	user_id_str, err := data.rdb.Get(ctx, fmt.Sprintf("user_id:%s", sso)).Result()
	if err != nil {
		return nil, err
	}
	user_id_i64, err := strconv.ParseInt(user_id_str, 10, 32)
	if err != nil {
		return nil, err
	}
	user_id := int(user_id_i64)
	return &user_id, nil
}

func (data *Database) getCachedFriendsWithKey(ctx context.Context, key string) ([]*Friend, error) {
	var friends []*Friend = nil
	var cursor uint64 = 0
	var err error
	for {
		var keysFromScan []string
		keysFromScan, cursor, err = data.rdb.SScan(ctx, key, cursor, "*", 100).Result()
		if err != nil {
			log.Printf("getCachedFriendsWithKey(): Got error while executing scanning for friends: %v", err)
			break
		}

		pipe := data.rdb.Pipeline()
		for _, key := range keysFromScan {
			pipe.HGetAll(ctx, key)
		}
		cmds, err := pipe.Exec(ctx)
		if err != nil {
			log.Printf("getCachedFriendsWithKey(): Got error while executing pipeline for friends: %v", err)
			continue
		}

		for _, c := range cmds {
			var friend Friend
			if err := c.(*redis.MapStringStringCmd).Scan(&friend); err != nil {
				log.Printf("getCachedFriendsWithKey(): Got error while scanning cached friend: %v", err)
				continue
			}
			friends = append(friends, &friend)
		}

		if cursor == 0 {
			break
		}
	}
	return friends, err
}

func (data *Database) GetFriendById(ctx context.Context, friendshipId int) (*Friend, error) {
	var friend Friend
	friend_id_string := fmt.Sprintf("friends:%d", friendshipId)
	exists, err := data.rdb.Exists(ctx, friend_id_string).Result()
	if err == nil && exists != 0 {
		err := data.rdb.HGetAll(ctx, friend_id_string).Scan(&friend)
		if err == nil {
			return &friend, nil
		}
	}

	row := data.db.QueryRowContext(ctx, "SELECT id, user_one_id, relation, category FROM messenger_friendships WHERE id = ?", friendshipId)
	if err := row.Scan(&friend.Id, &friend.UserId, &friend.Relation, &friend.CategoryId); err != nil {
		return nil, err
	}
	friendData, err := Player.GetUserDataById(ctx, friend.UserId)
	if err != nil {
		log.Printf("GetFriendsForUser(): Got error while fetching user data: %v", err)
		return &friend, err
	}
	friend.Username = friendData.Username
	friend.Gender = friendData.Gender
	friend.Look = friendData.Look
	friend.Motto = friendData.Motto
	return &friend, nil
}

func (data *Database) loadUserPermissionFromRPC(sso string, permission string) *bool {
	userData := Player.GetUserData(sso)
	if userData == nil {
		return nil
	}
	perm := Perm.GetPermission(int(userData.GetRank()), permission).GetSetting() == proto.PermissionSetting_ALLOWED
	return &perm
}

func (data *Database) GetPermission(ctx context.Context, sso string, permission string) (*bool, error) {
	key := fmt.Sprintf("user_permissions:%s:%s", sso, permission)
	exists, err := data.rdb.Exists(ctx, key).Result()
	if err != nil || exists == 0 {
		perm := data.loadUserPermissionFromRPC(sso, permission)
		if perm != nil {
			return perm, nil
		}
		return nil, err
	}
	permission_string, err := data.rdb.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	permission_i64, err := strconv.ParseInt(permission_string, 10, 32)
	if err != nil {
		return nil, err
	}
	allowed := permission_i64 == int64(proto.PermissionSetting_ALLOWED)
	return &allowed, nil
}

func (data *Database) GetFriendsForUser(ctx context.Context, sso string) ([]*Friend, error) {
	user_id, err := data.getUserId(ctx, sso)
	if err != nil {
		return nil, err
	}

	friend_id_list := fmt.Sprintf("friends_by_user_id:%d", *user_id)
	exists, err := data.rdb.Exists(ctx, friend_id_list).Result()
	if err == nil && exists != 0 {
		friends, err := data.getCachedFriendsWithKey(ctx, friend_id_list)
		if err == nil && friends != nil {
			return friends, nil
		}
	}

	friends := make([]*Friend, 0)
	rows, err := data.db.QueryContext(ctx, "SELECT messenger_friendships.id, users.username, users.gender, users.look, users.motto, user_one_id, relation, category FROM messenger_friendships INNER JOIN users ON messenger_friendships.user_one_id = users.id WHERE user_two_id = ?", *user_id)
	if err != nil {
		return nil, err
	}
	var wg sync.WaitGroup
	for rows.Next() {
		var friend Friend
		if err := rows.Scan(&friend.Id, &friend.Username, &friend.Gender, &friend.Look, &friend.Motto, &friend.UserId, &friend.Relation, &friend.CategoryId); err != nil {
			log.Printf("GetFriendsForUser(): Got error: %v", err)
			continue
		}
		friends = append(friends, &friend)
	}
	wg.Wait()
	go func() {
		pipe := data.rdb.Pipeline()
		pipe.Del(ctx, friend_id_list)
		for _, friend := range friends {
			pipe.HSet(ctx, fmt.Sprintf("friend:%d", friend.Id), *friend)
			pipe.SAdd(ctx, friend_id_list, friend.Id)
		}
		cmds, err := pipe.Exec(ctx)
		if err != nil {
			log.Printf("Got error executing friends cache pipeline: %v", err)
			return
		}
		for _, c := range cmds {
			if err := c.Err(); err != nil {
				log.Printf("Got error caching friends: %v", err)
			}
		}
	}()
	return friends, nil
}

type MessengerCategory struct {
	Id     int    `redis:"id"`
	UserId int    `redis:"user_id"`
	Name   string `redis:"name"`
}

func (x *MessengerCategory) Serialize() []byte {
	return serializeValues(x.Id, x.Name)
}

func (data *Database) getCachedCategoriesWithKey(ctx context.Context, key string) ([]*MessengerCategory, error) {
	var categories []*MessengerCategory = nil
	var cursor uint64 = 0
	var err error
	for {
		var keysFromScan []string
		keysFromScan, cursor, err = data.rdb.SScan(ctx, key, cursor, "*", 100).Result()
		if err != nil {
			log.Printf("getCachedCategoriesWithKey(): Got error while executing scanning for categories: %v", err)
			break
		}

		pipe := data.rdb.Pipeline()
		for _, key := range keysFromScan {
			pipe.HGetAll(ctx, key)
		}
		cmds, err := pipe.Exec(ctx)
		if err != nil {
			log.Printf("getCachedCategoriesWithKey(): Got error while executing pipeline for categories: %v", err)
			continue
		}

		for _, c := range cmds {
			var category MessengerCategory
			if err := c.(*redis.MapStringStringCmd).Scan(&category); err != nil {
				log.Printf("getCachedCategoriesWithKey(): Got error while scanning cached category: %v", err)
				continue
			}
			categories = append(categories, &category)
		}

		if cursor == 0 {
			break
		}
	}
	return categories, err
}

func (data *Database) GetMessengerCategories(ctx context.Context, sso string) ([]*MessengerCategory, error) {
	user_id, err := data.getUserId(ctx, sso)
	if err != nil {
		return nil, err
	}

	key := fmt.Sprintf("messenger_categories_by_user:%d", *user_id)
	exists, err := data.rdb.Exists(ctx, key).Result()
	if err == nil && exists != 0 {
		cats, err := data.getCachedCategoriesWithKey(ctx, key)
		if err == nil {
			return cats, nil
		}
	}

	rows, err := data.db.QueryContext(ctx, "SELECT id, user_id, name FROM messenger_categories WHERE user_id = ?", user_id)
	if err != nil {
		log.Printf("GetMessengerCategories(): Got error while fetching data from database: %v", err)
		return nil, err
	}

	var categories []*MessengerCategory = nil
	pipe := data.rdb.Pipeline()
	for rows.Next() {
		var category MessengerCategory
		if err := rows.Scan(category.Id, category.UserId, category.Name); err != nil {
			log.Printf("GetMessengerCategories(): Got error while scanning result data: %v", err)
			continue
		}

		categories = append(categories, &category)
		pipe.HSet(ctx, fmt.Sprintf("messenger_categories:%d", category.Id))
		pipe.SAdd(ctx, key, category.Id)
	}

	cmds, err := pipe.Exec(ctx)
	if err != nil {
		log.Printf("GetMessengerCategories(): Got error while executing results cache pipeline: %v", err)
		return categories, nil
	}

	for _, c := range cmds {
		if err := c.Err(); err != nil {
			log.Printf("GetMessengerCategories(): Got error while caching categories: %v", err)
		}
	}

	return categories, nil
}

type FriendRequest struct {
	Id         int    `redis:"id"`
	Username   string `redis:"username"`
	Look       string `redis:"look"`
	Serialized string `redis:"serialized"`
}

func (req *FriendRequest) toBytes() []byte {
	return serializeValues(req.Id, req.Username, req.Look)
}

func (req *FriendRequest) Serialize() []byte {
	if req.Serialized == "" {
		req.Serialized = string(req.toBytes())
	}
	return []byte(req.Serialized)
}

func (data *Database) getCachedFriendRequestsWithKey(ctx context.Context, key string) ([]*FriendRequest, error) {
	var requests []*FriendRequest = nil
	var cursor uint64 = 0
	var err error
	for {
		var keysFromScan []string
		keysFromScan, cursor, err = data.rdb.SScan(ctx, key, cursor, "*", 100).Result()
		if err != nil {
			log.Printf("getCachedFriendRequestsWithKey(): Got error while executing scanning for requests: %v", err)
			break
		}

		pipe := data.rdb.Pipeline()
		for _, key := range keysFromScan {
			pipe.HGetAll(ctx, key)
		}
		cmds, err := pipe.Exec(ctx)
		if err != nil {
			log.Printf("getCachedFriendRequestsWithKey(): Got error while executing pipeline for friend requests: %v", err)
			continue
		}

		for _, c := range cmds {
			var request FriendRequest
			if err := c.(*redis.MapStringStringCmd).Scan(&request); err != nil {
				log.Printf("getCachedFriendRequestsWithKey(): Got error while scanning cached friend requests: %v", err)
				continue
			}
			requests = append(requests, &request)
		}

		if cursor == 0 {
			break
		}
	}
	return requests, nil
}

func (data *Database) GetFriendRequestsForUser(ctx context.Context, sso string) ([]*FriendRequest, error) {
	user_id, err := data.getUserId(ctx, sso)
	if err != nil {
		return nil, err
	}

	requests := make([]*FriendRequest, 0)

	request_id_list := fmt.Sprintf("requests_by_user_id:%d", *user_id)
	exists, err := data.rdb.Exists(ctx, request_id_list).Result()
	if err == nil && exists != 0 {
		requests, err := data.getCachedFriendRequestsWithKey(ctx, request_id_list)
		if err == nil && requests != nil {
			return requests, nil
		}
	}

	rows, err := data.db.QueryContext(ctx, "SELECT users.id, users.username, users.look FROM messenger_friendrequests INNER JOIN users ON user_from_id = users.id WHERE user_to_id = ?", user_id)
	if err != nil {
		log.Printf("GetFriendRequestsForUser(): Got error while fetching data from database: %v", err)
		return nil, err
	}

	pipe := data.rdb.Pipeline()
	for rows.Next() {
		var req FriendRequest
		if err := rows.Scan(&req.Id, &req.Username, &req.Look); err != nil {
			log.Printf("GetFriendRequestsForUser(): Got error while scanning data from database: %v", err)
			continue
		}
		req.Serialize()
		requests = append(requests, &req)
		pipe.HSet(ctx, fmt.Sprintf("requests:%d", req.Id), req)
		pipe.SAdd(ctx, request_id_list, req.Id)
	}
	cmds, err := pipe.Exec(ctx)
	if err != nil {
		log.Printf("GetFriendRequestsForUser(): Got error while executing results cache pipeline: %v", err)
		return requests, nil
	}

	for _, c := range cmds {
		if err := c.Err(); err != nil {
			log.Printf("GetFriendRequestsForUser(): Got error while caching categories: %v", err)
		}
	}

	return requests, nil
}
