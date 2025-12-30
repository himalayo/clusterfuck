package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/go-sql-driver/mysql"
	pb "github.com/himalayo/clusterfuck/api/configuration/proto"
	"github.com/himalayo/clusterfuck/api/events"
	"github.com/redis/go-redis/v9"
)

type Database struct {
	db                   *sql.DB
	rdb                  *redis.Client
	networking_publisher *events.RedisPublisher
}

type DatabaseConfig struct {
	mysql_config          *mysql.Config
	redis_config          *redis.Options
	network_events_config *redis.Options
}

func ConfigDatabaseFromEnv() *DatabaseConfig {
	cfg := mysql.NewConfig()
	cfg.User = os.Getenv("DB_USER")
	cfg.Passwd = os.Getenv("DB_PASS")
	cfg.Net = "tcp"
	cfg.Addr = os.Getenv("DB_ADDR")
	cfg.DBName = os.Getenv("DB_NAME")

	redis_config := &redis.Options{
		Addr:     os.Getenv("CONFIGURATION_REDIS_ADDR"),
		Password: os.Getenv("CONFIGURATION_REDIS_PASSWORD"),
		DB:       0,
	}

	network_events_config := &redis.Options{
		Addr:     os.Getenv("CONFIGURATION_EVENTS_REDIS_ADDR"),
		Password: os.Getenv("CONFIGURATION_EVENTS_REDIS_PASSWORD"),
		DB:       0,
	}

	return &DatabaseConfig{
		redis_config:          redis_config,
		mysql_config:          cfg,
		network_events_config: network_events_config,
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
	db, err := sql.Open("mysql", cfg.mysql_config.FormatDSN())
	if err != nil {
		log.Fatal(err)
	}
	rdb := redis.NewClient(cfg.redis_config)
	return &Database{db: db, rdb: rdb,
		networking_publisher: events.NewRedisPublisher(cfg.network_events_config, "networking-configuration-events")}
}

func (data *Database) GetInt(key string) (int, error) {
	var out int
	row := data.db.QueryRow("SELECT value FROM emulator_settings WHERE `key` = ?", key)
	if err := row.Scan(&out); err != nil {
		return out, err
	}
	return out, nil
}

func (data *Database) GetDouble(key string) (float64, error) {
	var out float64
	row := data.db.QueryRow("SELECT value FROM emulator_settings WHERE `key` = ?", key)
	if err := row.Scan(&out); err != nil {
		return out, err
	}
	return out, nil
}

func (data *Database) GetString(key string) (string, error) {
	var out string
	row := data.db.QueryRow("SELECT value FROM emulator_settings WHERE `key` = ?", key)
	if err := row.Scan(&out); err != nil {
		return out, err
	}
	return out, nil
}

func (data *Database) GetBoolean(key string) (bool, error) {
	rawValue, err := data.GetString(key)
	if err != nil {
		return false, err
	}
	return rawValue == "1", nil
}

func (data *Database) getIncomingServiceFromRedis(ctx context.Context, key string) (*IncomingService, error) {
	var service IncomingService
	err := data.rdb.HGetAll(ctx, key).Scan(&service)
	if err != nil {
		return nil, err
	}
	return &service, nil
}

func (data *Database) getOutgoingServiceFromRedis(ctx context.Context, key string) (*OutgoingService, error) {
	var service OutgoingService
	err := data.rdb.HGetAll(ctx, key).Scan(&service)
	if err != nil {
		return nil, err
	}
	return &service, nil
}

func (data *Database) GetServices(ctx context.Context) ([]*pb.IncomingService, []*pb.OutgoingService, error) {
	incoming := make([]*pb.IncomingService, 0)
	outgoing := make([]*pb.OutgoingService, 0)

	var wg sync.WaitGroup

	wg.Go(
		func() {
			var scan_wg sync.WaitGroup
			var cursor uint64
			var err error
			for {
				ch := make(chan *IncomingService)
				var keysFromScan []string
				keysFromScan, cursor, err = data.rdb.ScanType(ctx, cursor, "incoming_service:*", 10, "hash").Result()
				if err != nil {
					break
				}

				for i := range keysFromScan {
					scan_wg.Go(func() {
						service, err := data.getIncomingServiceFromRedis(ctx, keysFromScan[i])
						if err == nil {
							ch <- service
						}
					})
				}

				go func() {
					scan_wg.Wait()
					close(ch)
				}()

				for service := range ch {
					incoming = append(incoming, service.toProto())
				}

				if cursor == 0 {
					break
				}
			}
		},
	)

	wg.Go(
		func() {
			var scan_wg sync.WaitGroup
			var cursor uint64
			var err error
			for {
				ch := make(chan *OutgoingService)
				var keysFromScan []string
				keysFromScan, cursor, err = data.rdb.ScanType(ctx, cursor, "outgoing_service:*", 10, "hash").Result()
				if err != nil {
					break
				}

				for i := range keysFromScan {
					scan_wg.Go(func() {
						service, err := data.getOutgoingServiceFromRedis(ctx, keysFromScan[i])
						if err == nil {
							ch <- service
						}
					})
				}

				go func() {
					scan_wg.Wait()
					close(ch)
				}()

				for service := range ch {
					outgoing = append(outgoing, service.toProto())
				}

				if cursor == 0 {
					break
				}
			}
		},
	)

	wg.Wait()

	return incoming, outgoing, nil
}

func (data *Database) StoreIncomingService(ctx context.Context, service *IncomingService) error {
	_, err := data.rdb.HSet(ctx, fmt.Sprintf("incoming_service:%s", service.Service), *service).Result()
	return err
}

func (data *Database) StoreOutgoingService(ctx context.Context, service *OutgoingService) error {
	_, err := data.rdb.HSet(ctx, fmt.Sprintf("outgoing_service:%s", service.Service), *service).Result()
	return err
}

type OutgoingService struct {
	Service  string `redis:"service"`
	Address  string `redis:"address"`
	Password string `redis:"password"`
	DB       int    `redis:"db"`
}

func (service *OutgoingService) toProto() *pb.OutgoingService {
	return &pb.OutgoingService{
		Service: service.Service,
		Instance: &pb.RedisInstance{
			Address:  service.Address,
			Password: service.Password,
			Db:       int32(service.DB),
		},
	}
}

type IncomingService struct {
	Service  string `redis:"service"`
	Stream   string `redis:"stream"`
	Group    string `redis:"group"`
	Address  string `redis:"address"`
	Password string `redis:"password"`
	DB       int    `redis:"db"`
	Headers  string `redis:"headers"`
}

func (service *IncomingService) toProto() *pb.IncomingService {
	headers_strings := strings.Split(service.Headers, ";")
	headers := make([]int32, 0, len(headers_strings))
	for _, header_string := range headers_strings {
		header_i64, err := strconv.ParseInt(header_string, 10, 32)
		if err == nil {
			headers = append(headers, int32(header_i64))
		}
	}
	return &pb.IncomingService{
		Service: service.Service,
		Stream: &pb.RedisStream{
			Stream: service.Stream,
			Group:  service.Group,
			Instance: &pb.RedisInstance{
				Address:  service.Address,
				Password: service.Password,
				Db:       int32(service.DB),
			},
		},
		Headers: headers,
	}
}
