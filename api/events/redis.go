package events

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type RedisPublisher struct {
	rdb    *redis.Client
	Stream string
}

func NewRedisPublisher(redis_config *redis.Options, stream string) *RedisPublisher {
	return &RedisPublisher{
		rdb:    redis.NewClient(redis_config),
		Stream: stream,
	}
}

type RedisSubscriber struct {
	rdb      *redis.Client
	Stream   string
	Group    string
	Consumer string
	handlers map[string][]*HandlerInstance
	mu       sync.RWMutex
}

func NewRedisSubscriber(redis_config *redis.Options, stream string, group string) (*RedisSubscriber, error) {
	consumer_uuid, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}
	consumer := consumer_uuid.String()
	rdb := redis.NewClient(redis_config)
	handlers := make(map[string][]*HandlerInstance)

	rdb.XGroupCreateMkStream(context.Background(), stream, group, "$").Result()
	return &RedisSubscriber{
		rdb:      rdb,
		Stream:   stream,
		Group:    group,
		Consumer: consumer,
		handlers: handlers,
	}, nil
}

type RedisSubscription struct {
	Bus     *RedisSubscriber
	Type    string
	Handler *HandlerInstance
}

func (s *RedisSubscription) Unsubscribe() {
	s.Bus.mu.Lock()
	defer s.Bus.mu.Unlock()

	entries := s.Bus.handlers[s.Type]
	for i, h := range entries {
		if h == s.Handler {
			s.Bus.handlers[s.Type] = append(entries[:i], entries[i+1:]...)
			break
		}
	}
}

func (b *RedisPublisher) Publish(ctx context.Context, evt Event) error {
	_, err := b.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: b.Stream,
		Values: evt.GetValues(),
		MaxLen: 20000,
	}).Result()

	return err
}

func (b *RedisSubscriber) Subscribe(eventType string, handler EventHandler) Subscription {
	b.mu.Lock()
	defer b.mu.Unlock()

	handlerInstance := &HandlerInstance{handler: handler}
	b.handlers[eventType] = append(b.handlers[eventType], handlerInstance)
	return &RedisSubscription{
		Bus:     b,
		Type:    eventType,
		Handler: handlerInstance,
	}
}

var (
	ErrKeyNotFound = errors.New("not found")
	ErrInvalidType = errors.New("invalid type")
)

func getRedisStringValue(values map[string]interface{}, key string) (string, error) {
	val, ok := values[key]
	if !ok {
		return "", ErrKeyNotFound
	}
	str, ok := val.(string)
	if !ok {
		byt, ok := val.([]byte)
		if !ok {
			return "", ErrInvalidType
		}
		str = string(byt)
	}
	return str, nil
}

func FromRedisEvent(values map[string]interface{}) (*GenericEvent, error) {
	id, err := getRedisStringValue(values, "id")
	if err != nil {
		return nil, err
	}

	eventType, err := getRedisStringValue(values, "type")
	if err != nil {
		return nil, err
	}

	return &GenericEvent{
		Id:     id,
		Type:   eventType,
		Values: values,
	}, nil
}

func safeCall(h EventHandler, ctx context.Context, evt Event) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("safeCall(): event handler panic: %v", r)
		}
	}()
	h(ctx, evt)
}

var MICROSECOND_THRESHOLD = time.Second.Microseconds()

func (b *RedisSubscriber) Recover(ctx context.Context) error {
	pending_data, err := b.rdb.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: b.Stream,
		Group:  b.Group,
		Count:  200,
		Start:  "-",
		End:    "+",
	}).Result()
	if err != nil {
		return err
	}

	pending_ids := make([]string, 0, len(pending_data))
	for _, pending := range pending_data {
		if pending.Idle.Microseconds() > MICROSECOND_THRESHOLD {
			pending_ids = append(pending_ids, pending.ID)
		}
	}
	if len(pending_ids) == 0 {
		return nil
	}

	messages, err := b.rdb.XClaim(ctx, &redis.XClaimArgs{
		Stream:   b.Stream,
		Group:    b.Group,
		Consumer: b.Consumer,
		Messages: pending_ids,
	}).Result()
	if err != nil {
		return err
	}

	b.handleMessages(ctx, messages)
	return nil
}

func (b *RedisSubscriber) handleMessages(ctx context.Context, msgs []redis.XMessage) error {
	ids := make([]string, len(msgs))

	for i, msg := range msgs {
		ids[i] = msg.ID
		evt, err := FromRedisEvent(msg.Values)
		if err != nil {
			log.Printf("RedisSubscriber.handleMessages(): Group: %s Consumer: %s Stream: %s Got error event: %v", b.Group, b.Consumer, b.Stream, err)
			continue
		}
		b.mu.RLock()
		handlers := append([]*HandlerInstance(nil), b.handlers[evt.Type]...)
		b.mu.RUnlock()
		if len(handlers) == 0 {
			continue
		}

		for _, instance := range handlers {
			safeCall(instance.handler, ctx, evt)
		}
	}
	_, err := b.rdb.XAck(ctx, b.Stream, b.Group, ids...).Result()
	if err != nil {
		return err
	}
	return nil
}

func (b *RedisSubscriber) RecoverInterval(ctx context.Context, interval time.Duration) *time.Ticker {
	ticker := time.NewTicker(interval)
	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				err := b.Recover(ctx)
				if err != nil {
					log.Printf("RedisSubscriber.RecoverInterval(): Group: %s Consumer: %s Stream: %s Got error: %v", b.Group, b.Consumer, b.Stream, err)
				}
			}
		}
	}()

	return ticker
}

func (b *RedisSubscriber) Listen(ctx context.Context) {
	go b.RecoverInterval(ctx, 2*time.Second)

	for {
		results, err := b.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    b.Group,
			Consumer: b.Consumer,
			Streams:  []string{b.Stream, ">"},
			Block:    time.Second,
		}).Result()

		if err == redis.Nil {
			continue
		}

		if errors.Is(err, context.Canceled) {
			return
		}

		if err != nil {
			log.Printf("RedisSubscriber.Listen(): Group: %s Consumer: %s Stream: %s Got error: %v", b.Group, b.Consumer, b.Stream, err)
			continue
		}

		for _, result := range results {
			b.handleMessages(ctx, result.Messages)
		}
	}
}
