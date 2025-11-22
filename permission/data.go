package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/go-sql-driver/mysql"

	pb "github.com/himalayo/clusterfuck/permission/proto"
)


type Database struct {
	db *sql.DB
}

type PermissionSetting int

const (
	DISALLOWED PermissionSetting = iota
	ALLOWED
	ROOM_OWNER
)

type Permission struct {
	Key string
	Setting PermissionSetting
}

func (p Permission) toProto() *pb.Permission {
	return &pb.Permission{Key: p.Key, Setting: pb.PermissionSetting(p.Setting)}
}

type Rank struct {
	Id int
	Level int
	Permissions map[string]Permission
	Variables map[string]string
	Name string
	Badge string
	RoomEffect int
	LogCommands bool
	Prefix string
	PrefixColor string
	DiamondsTimerAmount int
	CreditsTimerAmount int
	PixelsTimerAmount int
	GotwTimerAmount int
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
		Id: int32(r.Id),
		Level: int32(r.Level),
		Permissions: perms,
		Variables: r.Variables,
		Name: r.Name,
		Badge: r.Badge,
		RoomEffect: int32(r.RoomEffect),
		LogCommands: r.LogCommands,
		Prefix: r.Prefix,
		PrefixColor: r.PrefixColor,
		DiamondsTimerAmount: int32(r.DiamondsTimerAmount),
		CreditsTimerAmount: int32(r.CreditsTimerAmount),
		PixelsTimerAmount: int32(r.PixelsTimerAmount),
		GotwTimerAmount: int32(r.GotwTimerAmount),
	}
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

func rankFromQueryResult(cols []string, result []string) *Rank {
	var output Rank
	output.DiamondsTimerAmount = 1
	output.CreditsTimerAmount = 1
	output.PixelsTimerAmount = 1
	output.GotwTimerAmount = 1
	output.Permissions = make(map[string]Permission)
	output.Variables = make(map[string]string)

	for idx, column := range cols {
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
				if err == nil{
					output.Permissions[column] = Permission{Key: column, Setting: PermissionSetting(permission)}
				}
				continue
			}
			output.Variables[column] = result[idx]

		}
	}

	return &output
}

func (data *Database) GetAllRanks() ([]Rank, error) {
	ranks := make([]Rank, 0)
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
		ranks = append(ranks, *rankFromQueryResult(cols, result))
	}

	return ranks, nil
}

func (data *Database) GetRank(id int) (*Rank, error) {
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
	if rs.Next() == false {
		return nil, nil
	}

	for i := range cols {
		resultPtrs[i] = &result[i]
	}
	if err := rs.Scan(resultPtrs...); err != nil {
		return nil, err
	}
	rank = *rankFromQueryResult(cols, result)
	return &rank, nil
}

func (data *Database) GetRankByName(name string) (*Rank, error) {
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
	if rs.Next() == false {
		return nil, nil
	}
	for i := range cols {
		resultPtrs[i] = &result[i]
	}

	if err := rs.Scan(resultPtrs...); err != nil {
		return nil, err
	}
	rank = *rankFromQueryResult(cols, result)

	return &rank, nil
}

func (data *Database) GetPermission(rankId int, permissionKey string) (int, error) {
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
	var result int
	row := data.db.QueryRow("SELECT level FROM permissions WHERE id = ? LIMIT 1", rankId)
	if err := row.Scan(&result); err != nil {
		return -1, err
	}
	return result, nil
}
