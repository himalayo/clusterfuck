package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/go-sql-driver/mysql"
	pb "github.com/himalayo/clusterfuck/api/guilds/proto"
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
			Addr:     os.Getenv("GUILDS_REDIS_ADDR"),
			Password: os.Getenv("GUILDS_REDIS_PASSWORD"),
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

type GuildPart struct {
	Id     int    `redis:"id"`
	Type   string `redis:"type"`
	ValueA string `redis:"value_a"`
	ValueB string `redis:"value_b"`
	Packet string `redis:"serialized"`
}

func (p *GuildPart) ToProto() *pb.GuildPart {
	return &pb.GuildPart{
		Id:     int32(p.Id),
		ValueA: p.ValueA,
		ValueB: p.ValueB,
	}
}

func (p GuildPart) toBytes() []byte {
	packet := appendString(integerToBytes(p.Id), p.ValueA)
	if !strings.HasSuffix(p.Type, "color") {
		packet = appendString(packet, p.ValueB)
	}
	//log.Printf("Id: %d ([% x]) ValueA: %s ([% x] [% x]) ValueB: %s ([%x ][% x]) Result: [% x]", p.Id, integerToBytes(p.Id), p.ValueA, shortToBytes(len(p.ValueA)), []byte(p.ValueA), p.ValueB, shortToBytes(len(p.ValueB)), []byte(p.ValueB), packet)
	return packet
}

func (p GuildPart) Serialize() []byte {
	return []byte(p.Packet)
}

func (data *Database) loadGuildPartsFromDB(ctx context.Context) (map[string]map[int]GuildPart, error) {
	out := make(map[string]map[int]GuildPart)
	rows, err := data.db.QueryContext(ctx, "SELECT type, id, firstvalue, secondvalue from guilds_elements")
	if err != nil {
		log.Printf("loadGuildPartsFromDB: Got error querying for guild_elements: %v", err)
		return nil, err
	}
	count := 0

	for rows.Next() {
		var part GuildPart
		if err := rows.Scan(&part.Type, &part.Id, &part.ValueA, &part.ValueB); err != nil {
			log.Printf("loadGuildPartsFromDB: Got error scanning guild_elements: %v", err)
			return nil, err
		}
		part.Packet = string(part.toBytes())
		_, ok := out[part.Type]
		if !ok {
			out[part.Type] = make(map[int]GuildPart)
		}
		out[part.Type][part.Id] = part
		_, err = data.rdb.HSet(ctx, fmt.Sprintf("guild_elements:%d", count), part).Result()
		if err != nil {
			log.Printf("loadGuildPartsFromDB: Got error caching guild_elements: %v", err)
		}
		_, err = data.rdb.SAdd(ctx, fmt.Sprintf("guild_elements_by_type:%s", part.Type), count).Result()
		if err != nil {
			log.Printf("loadGuildPartsFromDB: Got error indexing guild_elements: %v", err)
		}
		count++
	}
	return out, nil
}

func (data *Database) loadGuildPartsFromRedis(ctx context.Context) (map[string]map[int]GuildPart, error) {
	out := make(map[string]map[int]GuildPart)

	var cursor uint64
	var err error
	var wg sync.WaitGroup
	for {
		ch := make(chan *GuildPart)
		var keysFromScan []string
		keysFromScan, cursor, err = data.rdb.ScanType(ctx, cursor, "guild_elements:*", 10, "hash").Result()
		if err != nil {
			break
		}

		for i := range keysFromScan {
			wg.Add(1)
			go func() {
				var res GuildPart
				err := data.rdb.HGetAll(ctx, keysFromScan[i]).Scan(&res)
				if err == nil {
					log.Printf("loadGuildPartsFromRedis(): Loaded GuildParts %s from cache successfuly", keysFromScan[i])
					ch <- &res
				}
				wg.Done()
			}()
		}

		go func() {
			wg.Wait()
			close(ch)
		}()

		for part := range ch {
			parts, ok := out[part.Type]
			if !ok {
				out[part.Type] = make(map[int]GuildPart)
				parts = out[part.Type]
			}
			parts[part.Id] = *part
		}

		if cursor == 0 {
			break
		}
	}
	return out, nil
}

func (data *Database) GetGuildParts(ctx context.Context) (map[string]map[int]GuildPart, error) {
	exists, err := data.rdb.Exists(ctx, "guild_elements:0").Result()
	if err != nil || exists == 0 {
		return data.loadGuildPartsFromDB(ctx)
	}
	return data.loadGuildPartsFromRedis(ctx)
}

func (data *Database) LoadGuildPartsByType(ctx context.Context, partType string) ([]GuildPart, error) {
	ids_string, err := data.rdb.SMembers(ctx, fmt.Sprintf("guild_elements_by_type:%s", partType)).Result()
	if err != nil {
		parts, err := data.loadGuildPartsFromDB(ctx)
		if err != nil {
			return nil, err
		}
		partsList := make([]GuildPart, 0, len(parts[partType]))
		for _, part := range parts[partType] {
			partsList = append(partsList, part)
		}
		return partsList, nil
	}
	cmds := make([]*redis.MapStringStringCmd, len(ids_string))

	_, err = data.rdb.Pipelined(ctx, func(pipeline redis.Pipeliner) error {
		for i, id_string := range ids_string {
			cmds[i] = pipeline.HGetAll(ctx, fmt.Sprintf("guild_elements:%s", id_string))
		}
		return nil
	})

	if err != nil {
		log.Printf("LoadGuildPartsByType(%s): Got error: %v", partType, err)
	}

	parts := make([]GuildPart, 0, len(ids_string))
	for _, cmd := range cmds {
		var part GuildPart
		err := cmd.Scan(&part)
		if err == nil {
			parts = append(parts, part)
		} else {
			log.Printf("LoadGuildPartsByType(%s): Got error: %v", partType, err)
		}
	}

	return parts, nil
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
