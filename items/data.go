package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/go-sql-driver/mysql"
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
			Addr:     os.Getenv("ITEMS_REDIS_ADDR"),
			Password: os.Getenv("ITEMS_REDIS_PASSWORD"),
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

type Item struct {
	Id                  int     `redis:"id"`
	SpriteId            int     `redis:"sprite_id"`
	ItemName            string  `redis:"item_name"`
	PublicName          string  `redis:"public_name"`
	Type                string  `redis:"type"`
	Width               int     `redis:"width"`
	Length              int     `redis:"length"`
	Height              float64 `redis:"height"`
	AllowStack          bool    `redis:"allow_stack"`
	AllowWalk           int     `redis:"allow_walk"`
	AllowSit            int     `redis:"allow_sit"`
	AllowLay            int     `redis:"allow_lay"`
	AllowRecycle        bool    `redis:"allow_recycle"`
	AllowTrade          bool    `redis:"allow_trade"`
	AllowMarketplace    bool    `redis:"allow_marketplace_sell"`
	AllowGift           bool    `redis:"allow_gift"`
	AllowInventoryStack bool    `redis:"allow_inventory_stack"`
	InteractionType     string  `redis:"interaction_type"`
	StateCount          int     `redis:"interaction_modes_count"`
	EffectM             int     `redis:"effect_m"`
	EffectF             int     `redis:"effect_f"`
	CustomParams        string  `redis:"custom_params"`
	ClothingOnWalk      string  `redis:"clothing_on_walk"`
	VendingIds          string  `redis:"vending_ids"`
	MultiHeight         string  `redis:"multiheight"`
	Rotations           int     `reids:"rotations"`
}

func (data *Database) loadBaseItemsFromDB(ctx context.Context) ([]Item, error) {
	items := make([]Item, 0)
	rows, err := data.db.QueryContext(ctx, "SELECT `id`, `sprite_id`, `item_name`, `public_name`, `type`, `width`, `length`, `stack_height`, `allow_stack`, `allow_walk`, `allow_sit`, `allow_lay`, `allow_recycle`, `allow_trade`, `allow_marketplace_sell`, `allow_gift`, `allow_inventory_stack`, `interaction_type`, `interaction_modes_count`, `effect_id_male`, `effect_id_female`, `customparams`, `clothing_on_walk`, `vending_ids`, `multiheight` FROM items_base")
	if err != nil {
		log.Printf("loadBaseItemsFromDB(): Got error: %s", err)
		return nil, err
	}
	pipe := data.rdb.Pipeline()
	count := 0
	for rows.Next() {
		var item Item
		if err := rows.Scan(&item.Id, &item.SpriteId, &item.ItemName, &item.PublicName, &item.Type, &item.Width, &item.Length, &item.Height, &item.AllowStack, &item.AllowWalk, &item.AllowSit, &item.AllowLay, &item.AllowRecycle, &item.AllowTrade, &item.AllowMarketplace, &item.AllowGift, &item.AllowInventoryStack, &item.InteractionType, &item.StateCount, &item.EffectM, &item.EffectF, &item.CustomParams, &item.ClothingOnWalk, &item.VendingIds, &item.MultiHeight); err != nil {
			log.Printf("loadBaseItemsFromDB(): Got error when scanning: %v", err)
			continue
		}
		item.Rotations = 4
		items = append(items, item)
		pipe.HSet(ctx, fmt.Sprintf("items_base:%d", item.Id), item)
		pipe.Set(ctx, fmt.Sprintf("items_base_id_by_name:%s", item.ItemName), item.Id, 0)
		count++
	}
	cmds, err := pipe.Exec(ctx)
	if err != nil {
		log.Printf("loadBaseItemsFromDB(): Got error when executing cache pipeline: %v", err)
	} else {
		for _, c := range cmds {
			if err := c.Err(); err != nil {
				log.Printf("loadBaseItemsFromDB(): Got error when caching: %v", err)
			}
		}
	}
	log.Printf("loadBaseItemsFromDB(): Successfully loaded %d items from Database", count)
	return items, nil
}

func (data *Database) loadItemByIdFromDB(ctx context.Context, id int) (*Item, error) {
	var item Item
	row := data.db.QueryRowContext(ctx, "SELECT `id`, `sprite_id`, `item_name`, `public_name`, `type`, `width`, `length`, `stack_height`, `allow_stack`, `allow_walk`, `allow_sit`, `allow_lay`, `allow_recycle`, `allow_trade`, `allow_marketplace_sell`, `allow_gift`, `allow_inventory_stack`, `interaction_type`, `interaction_modes_count`, `effect_id_male`, `effect_id_female`, `customparams`, `clothing_on_walk`, `vending_ids`, `multiheight` FROM items_base WHERE `id` = ?", id)
	if err := row.Scan(&item.Id, &item.SpriteId, &item.ItemName, &item.PublicName, &item.Type, &item.Width, &item.Length, &item.Height, &item.AllowStack, &item.AllowWalk, &item.AllowSit, &item.AllowLay, &item.AllowRecycle, &item.AllowTrade, &item.AllowMarketplace, &item.AllowGift, &item.AllowInventoryStack, &item.InteractionType, &item.StateCount, &item.EffectM, &item.EffectF, &item.CustomParams, &item.ClothingOnWalk, &item.VendingIds, &item.MultiHeight); err != nil {
		log.Printf("loadItemByIdFromDB(): Got error: %s", err)
		return nil, err
	}
	return &item, nil
}

func (data *Database) loadItemByIdFromRedis(ctx context.Context, id int) (*Item, error) {
	var item Item
	err := data.rdb.HGetAll(ctx, fmt.Sprintf("items_base:%d", id)).Scan(&item)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (data *Database) GetItemById(ctx context.Context, id int) (*Item, error) {
	exists, err := data.rdb.Exists(ctx, fmt.Sprintf("items_base:%d", id)).Result()
	if err == nil && exists != 0 {
		return data.loadItemByIdFromRedis(ctx, id)
	}
	return data.loadItemByIdFromDB(ctx, id)
}

func (data *Database) loadItemByNameFromDB(ctx context.Context, name string) (*Item, error) {
	var item Item
	row := data.db.QueryRowContext(ctx, "SELECT `id`, `sprite_id`, `item_name`, `public_name`, `type`, `width`, `length`, `stack_height`, `allow_stack`, `allow_walk`, `allow_sit`, `allow_lay`, `allow_recycle`, `allow_trade`, `allow_marketplace_sell`, `allow_gift`, `allow_inventory_stack`, `interaction_type`, `interaction_modes_count`, `effect_id_male`, `effect_id_female`, `customparams`, `clothing_on_walk`, `vending_ids`, `multiheight` FROM items_base WHERE `item_name`= ?", name)
	if err := row.Scan(&item.Id, &item.SpriteId, &item.ItemName, &item.PublicName, &item.Type, &item.Width, &item.Length, &item.Height, &item.AllowStack, &item.AllowWalk, &item.AllowSit, &item.AllowLay, &item.AllowRecycle, &item.AllowTrade, &item.AllowMarketplace, &item.AllowGift, &item.AllowInventoryStack, &item.InteractionType, &item.StateCount, &item.EffectM, &item.EffectF, &item.CustomParams, &item.ClothingOnWalk, &item.VendingIds, &item.MultiHeight); err != nil {
		log.Printf("loadItemByIdFromDB(): Got error: %s", err)
		return nil, err
	}
	return &item, nil
}

func (data *Database) loadItemByNameFromRedis(ctx context.Context, name string) (*Item, error) {
	var item Item
	id_string, err := data.rdb.Get(ctx, fmt.Sprintf("items_base_id_by_name:%s", name)).Result()
	if err != nil {
		log.Printf("loadItemByNameFromRedis(): Got error when getting id: %v", err)
		return nil, err
	}

	err = data.rdb.HGetAll(ctx, fmt.Sprintf("items_base:%s", id_string)).Scan(&item)
	if err != nil {
		log.Printf("loadItemByNameFromRedis(): Got error when getting item: %v", err)
		return nil, err
	}
	return &item, nil
}

func (data *Database) GetItemByName(ctx context.Context, name string) (*Item, error) {
	exists, err := data.rdb.Exists(ctx, fmt.Sprintf("items_base_id_by_name:%s", name)).Result()
	if err == nil && exists != 0 {
		return data.loadItemByNameFromRedis(ctx, name)
	}
	return data.loadItemByNameFromDB(ctx, name)
}

func (data *Database) loadItemsFromRedis(ctx context.Context) ([]Item, error) {
	items := make([]Item, 0)
	var cursor uint64
	var err error
	var wg sync.WaitGroup
	for {
		ch := make(chan *Item)
		var keysFromScan []string
		keysFromScan, cursor, err = data.rdb.ScanType(ctx, cursor, "items_base:*", 100, "hash").Result()
		if err != nil {
			break
		}

		for i := range keysFromScan {
			wg.Add(1)
			go func() {
				var res Item
				err := data.rdb.HGetAll(ctx, keysFromScan[i]).Scan(&res)
				if err == nil {
					log.Printf("loadItemsFromRedis(): Loaded Item %s from cache successfuly", keysFromScan[i])
					ch <- &res
				}
				wg.Done()
			}()
		}

		go func() {
			wg.Wait()
			close(ch)
		}()

		for item := range ch {
			items = append(items, *item)
		}

		if cursor == 0 {
			break
		}
	}
	return items, err
}

func (data *Database) GetItems(ctx context.Context) ([]Item, error) {
	exists, err := data.rdb.Exists(ctx, "items_base:0").Result()
	if err == nil && exists != 0 {
		return data.loadItemsFromRedis(ctx)
	}
	return data.loadBaseItemsFromDB(ctx)
}
