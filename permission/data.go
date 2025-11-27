package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"

	pb "github.com/himalayo/clusterfuck/api/permission/proto"
)

type Database struct {
	db  *sql.DB
	rdb *redis.Client
}

type PermissionSetting int

const (
	DISALLOWED PermissionSetting = iota
	ALLOWED
	ROOM_OWNER
)

type Permission struct {
	Key     string
	Setting PermissionSetting
}

func (p Permission) toProto() *pb.Permission {
	return &pb.Permission{Key: p.Key, Setting: pb.PermissionSetting(p.Setting)}
}

type Rank struct {
	Id                  int
	Level               int
	Permissions         map[string]Permission
	Variables           map[string]string
	Name                string
	Badge               string
	RoomEffect          int
	LogCommands         bool
	Prefix              string
	PrefixColor         string
	DiamondsTimerAmount int
	CreditsTimerAmount  int
	PixelsTimerAmount   int
	GotwTimerAmount     int
}

func (r Rank) String() string {
	return fmt.Sprintf("{id: %d, lvl: %d, name: %s}", r.Id, r.Level, r.Name)
}

func (r Rank) toProto() *pb.Rank {
	perms := make(map[string]*pb.Permission)
	for name, perm := range r.Permissions {
		perms[name] = perm.toProto()
	}

	return &pb.Rank{
		Id:                  int32(r.Id),
		Level:               int32(r.Level),
		Permissions:         perms,
		Variables:           r.Variables,
		Name:                r.Name,
		Badge:               r.Badge,
		RoomEffect:          int32(r.RoomEffect),
		LogCommands:         r.LogCommands,
		Prefix:              r.Prefix,
		PrefixColor:         r.PrefixColor,
		DiamondsTimerAmount: int32(r.DiamondsTimerAmount),
		CreditsTimerAmount:  int32(r.CreditsTimerAmount),
		PixelsTimerAmount:   int32(r.PixelsTimerAmount),
		GotwTimerAmount:     int32(r.GotwTimerAmount),
	}
}

type DatabaseConfig struct {
	sql_cfg *mysql.Config
	rd_cfg  *redis.Options
}

func ConfigDatabaseFromEnv() *DatabaseConfig {
	cfg := mysql.NewConfig()
	cfg.User = os.Getenv("DB_USER")
	cfg.Passwd = os.Getenv("DB_PASS")
	cfg.Net = "tcp"
	cfg.Addr = os.Getenv("DB_ADDR")
	cfg.DBName = os.Getenv("DB_NAME")
	redis_port, ok := os.LookupEnv("PERMISSION_REDIS_PORT")
	if !ok {
		redis_port = "6379"
	}
	redis_addr, ok := os.LookupEnv("PERMISSION_REDIS_ADDR")
	if !ok {
		redis_addr = net.JoinHostPort(os.Getenv("PERMISSION_REDIS_HOST"), redis_port)
	}
	password := os.Getenv("PERMISSION_REDIS_PASSWORD")
	rcfg := &redis.Options{
		Addr:     redis_addr,
		Password: password,
		DB:       0,
	}

	return &DatabaseConfig{
		sql_cfg: cfg,
		rd_cfg:  rcfg,
	}
}

func NewDatabase(cfg *DatabaseConfig) *Database {
	db, err := sql.Open("mysql", cfg.sql_cfg.FormatDSN())
	if err != nil {
		log.Fatal(err)
	}
	rdb := redis.NewClient(cfg.rd_cfg)

	return &Database{db: db, rdb: rdb}
}

func rankFromQueryResult(cols []string, result []string, rdb *redis.Client) *Rank {
	var output Rank
	output.DiamondsTimerAmount = 1
	output.CreditsTimerAmount = 1
	output.PixelsTimerAmount = 1
	output.GotwTimerAmount = 1
	output.Permissions = make(map[string]Permission)
	output.Variables = make(map[string]string)

	data := []string{}

	for idx, column := range cols {
		data = append(data, column, result[idx])
		switch column {
		case "id":
			id, err := strconv.ParseInt(result[idx], 10, 32)
			if err == nil {
				output.Id = int(id)
			}
		case "level":
			level, err := strconv.ParseInt(result[idx], 10, 32)
			if err == nil {
				output.Level = int(level)
			}
		case "rank_name":
			output.Name = result[idx]
		case "badge":
			output.Badge = result[idx]
		case "room_effect":
			effect, err := strconv.ParseInt(result[idx], 10, 32)
			if err == nil {
				output.RoomEffect = int(effect)
			}
		case "prefix":
			output.Prefix = result[idx]
		case "prefix_color":
			output.PrefixColor = result[idx]
		case "auto_points_amount":
			amount, err := strconv.ParseInt(result[idx], 10, 32)
			if err == nil {
				output.DiamondsTimerAmount = int(amount)
			}
		case "auto_credits_amount":
			amount, err := strconv.ParseInt(result[idx], 10, 32)
			if err == nil {
				output.CreditsTimerAmount = int(amount)
			}
		case "auto_pixels_amount":
			amount, err := strconv.ParseInt(result[idx], 10, 32)
			if err == nil {
				output.PixelsTimerAmount = int(amount)
			}
		case "auto_gotw_amount":
			amount, err := strconv.ParseInt(result[idx], 10, 32)
			if err == nil {
				output.GotwTimerAmount = int(amount)
			}
		case "log_commands":
			output.LogCommands = result[idx] == "1"
		default:
			if strings.HasPrefix(column, "acc_") || strings.HasPrefix(column, "cmd_") {
				permission, err := strconv.ParseInt(result[idx], 10, 32)
				if err == nil {
					output.Permissions[column] = Permission{Key: column, Setting: PermissionSetting(permission)}
				}
				continue
			}
			output.Variables[column] = result[idx]

		}
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()
		rdb.Set(ctx, fmt.Sprintf("rank_id:%s", output.Name), fmt.Sprintf("%d", output.Id), 0).Err()
	}()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()
		rdb.HSet(ctx, fmt.Sprintf("rank:%d", output.Id), data).Err()
	}()

	return &output
}

func rankFromQueryResultFromCache(data map[string]string) *Rank {
	var output Rank
	output.DiamondsTimerAmount = 1
	output.CreditsTimerAmount = 1
	output.PixelsTimerAmount = 1
	output.GotwTimerAmount = 1
	output.Permissions = make(map[string]Permission)
	output.Variables = make(map[string]string)

	for column, result := range data {
		switch column {
		case "id":
			id, err := strconv.ParseInt(result, 10, 32)
			if err == nil {
				output.Id = int(id)
			}
		case "level":
			level, err := strconv.ParseInt(result, 10, 32)
			if err == nil {
				output.Level = int(level)
			}
		case "rank_name":
			output.Name = result
		case "badge":
			output.Badge = result
		case "room_effect":
			effect, err := strconv.ParseInt(result, 10, 32)
			if err == nil {
				output.RoomEffect = int(effect)
			}
		case "prefix":
			output.Prefix = result
		case "prefix_color":
			output.PrefixColor = result
		case "auto_points_amount":
			amount, err := strconv.ParseInt(result, 10, 32)
			if err == nil {
				output.DiamondsTimerAmount = int(amount)
			}
		case "auto_credits_amount":
			amount, err := strconv.ParseInt(result, 10, 32)
			if err == nil {
				output.CreditsTimerAmount = int(amount)
			}
		case "auto_pixels_amount":
			amount, err := strconv.ParseInt(result, 10, 32)
			if err == nil {
				output.PixelsTimerAmount = int(amount)
			}
		case "auto_gotw_amount":
			amount, err := strconv.ParseInt(result, 10, 32)
			if err == nil {
				output.GotwTimerAmount = int(amount)
			}
		case "log_commands":
			output.LogCommands = result == "1"
		default:
			if strings.HasPrefix(column, "acc_") || strings.HasPrefix(column, "cmd_") {
				permission, err := strconv.ParseInt(result, 10, 32)
				if err == nil {
					output.Permissions[column] = Permission{Key: column, Setting: PermissionSetting(permission)}
				}
				continue
			}
			output.Variables[column] = result

		}
	}
	return &output
}

func (data *Database) GetAllRanks() ([]Rank, error) {
	ranks := make([]Rank, 0)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	var cursor uint64
	var err error
	ch := make(chan *Rank)
	var wg sync.WaitGroup
	for {
		var keysFromScan []string
		keysFromScan, cursor, err = data.rdb.ScanType(ctx, cursor, "rank:*", 10, "hash").Result()
		if err != nil {
			break
		}

		for i := range keysFromScan {
			wg.Add(1)
			go func() {
				res, err := data.getRankFromCache(keysFromScan[i])
				if err == nil {
					ch <- res
				}
				wg.Done()
			}()
		}

		go func() {
			wg.Wait()
			close(ch)
		}()

		for rank := range ch {
			ranks = append(ranks, *rank)
		}

		if cursor == 0 {
			break
		}
	}

	if len(ranks) > 0 {
		return ranks, nil
	}

	rs, err := data.db.Query("SELECT * FROM permissions")
	if err != nil {
		return nil, err
	}

	cols, err := rs.Columns()
	if err != nil {
		return nil, err
	}
	result := make([]string, len(cols))
	resultPtrs := make([]interface{}, len(cols))

	for rs.Next() {
		for i := range cols {
			resultPtrs[i] = &result[i]
		}
		if err := rs.Scan(resultPtrs...); err != nil {
			return nil, err
		}
		ranks = append(ranks, *rankFromQueryResult(cols, result, data.rdb))
	}

	return ranks, nil
}

func (data *Database) getRankFromCache(key string) (*Rank, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	res, err := data.rdb.HGetAll(ctx, key).Result()
	if err == nil {
		return rankFromQueryResultFromCache(res), nil
	}
	return nil, err
}

func (data *Database) GetRank(id int) (*Rank, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	res, err := data.rdb.HGetAll(ctx, fmt.Sprintf("rank:%d", id)).Result()
	if err == nil {
		return rankFromQueryResultFromCache(res), nil
	}

	var rank Rank
	rs, err := data.db.Query("SELECT * FROM permissions WHERE id = ? LIMIT 1", id)
	if err != nil {
		return nil, err
	}

	cols, err := rs.Columns()
	if err != nil {
		return nil, err
	}
	result := make([]string, len(cols))
	resultPtrs := make([]interface{}, len(cols))
	if !rs.Next() {
		return nil, nil
	}

	for i := range cols {
		resultPtrs[i] = &result[i]
	}
	if err := rs.Scan(resultPtrs...); err != nil {
		return nil, err
	}
	rank = *rankFromQueryResult(cols, result, data.rdb)
	return &rank, nil
}

func (data *Database) GetRankByName(name string) (*Rank, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	rank_id, err := data.rdb.Get(ctx, fmt.Sprintf("rank_id:%s", name)).Result()
	if err == nil {
		res, err := data.rdb.HGetAll(ctx, fmt.Sprintf("rank:%s", rank_id)).Result()
		if err == nil {
			return rankFromQueryResultFromCache(res), nil
		}
	}

	var rank Rank
	rs, err := data.db.Query("SELECT * FROM permissions WHERE rank_name = ? LIMIT 1", name)
	if err != nil {
		return nil, err
	}

	cols, err := rs.Columns()
	if err != nil {
		return nil, err
	}
	result := make([]string, len(cols))
	resultPtrs := make([]interface{}, len(cols))
	if !rs.Next() {
		return nil, nil
	}
	for i := range cols {
		resultPtrs[i] = &result[i]
	}

	if err := rs.Scan(resultPtrs...); err != nil {
		return nil, err
	}
	rank = *rankFromQueryResult(cols, result, data.rdb)

	return &rank, nil
}

func (data *Database) GetPermission(rankId int, permissionKey string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	perm, err := data.rdb.HGet(ctx, fmt.Sprintf("rank:%d", rankId), permissionKey).Result()
	if err == nil {
		permission, err := strconv.ParseInt(perm, 10, 32)
		if err == nil {
			return int(permission), nil
		}
	}

	firstRow := data.db.QueryRow("SELECT ? FROM permissions WHERE id = ? LIMIT 1", permissionKey, rankId)
	if err := firstRow.Scan(&permissionKey); err != nil {
		return -1, err
	}
	var raw string
	row := data.db.QueryRow("SELECT `"+permissionKey+"` FROM permissions WHERE id = ? LIMIT 1", rankId)
	if err := row.Scan(&raw); err != nil {
		return -1, err
	}
	permission, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		return -1, err
	}
	return int(permission), nil
}

func (data *Database) GetRankLevel(rankId int) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	perm, err := data.rdb.HGet(ctx, fmt.Sprintf("rank:%d", rankId), "level").Result()
	if err == nil {
		permission, err := strconv.ParseInt(perm, 10, 32)
		if err == nil {
			return int(permission), nil
		}
	}
	var result int
	row := data.db.QueryRow("SELECT level FROM permissions WHERE id = ? LIMIT 1", rankId)
	if err := row.Scan(&result); err != nil {
		return -1, err
	}
	return result, nil
}
