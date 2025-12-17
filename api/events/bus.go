package events

import (
	"context"
	"sync"
)

type Event interface {
	GetId() string
	GetType() string
	GetValues() map[string]interface{}
}

type GenericEvent struct {
	Id     string
	Type   string
	Values map[string]interface{}
}

func (evt *GenericEvent) GetId() string {
	return evt.Id
}

func (evt *GenericEvent) GetType() string {
	return evt.Type
}

func (evt *GenericEvent) GetValues() map[string]interface{} {
	return evt.Values
}

type EventHandler func(ctx context.Context, evt Event)
type HandlerInstance struct {
	handler EventHandler
}

type Publisher interface {
	Publish(ctx context.Context, evt Event) error
}

type Subscription interface {
	Unsubscribe()
}

type Subscriber interface {
	Subscribe(eventType string, handler EventHandler) Subscription
}

type EventBus interface {
	Publisher
	Subscriber
}

type LocalBus struct {
	mu       sync.RWMutex
	handlers map[string][]*HandlerInstance
}

type LocalSubscription struct {
	Bus     *LocalBus
	Type    string
	Handler *HandlerInstance
}

func (s *LocalSubscription) Unsubscribe() {
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

func (b *LocalBus) Subscribe(eventType string, handler EventHandler) Subscription {
	b.mu.Lock()
	defer b.mu.Unlock()

	handlerInstance := &HandlerInstance{handler: handler}
	b.handlers[eventType] = append(b.handlers[eventType], handlerInstance)
	return &LocalSubscription{
		Bus:     b,
		Type:    eventType,
		Handler: handlerInstance,
	}
}

func (b *LocalBus) Publish(ctx context.Context, evt Event) error {
	b.mu.RLock()
	handlers, ok := b.handlers[evt.GetType()]
	b.mu.Unlock()

	if !ok {
		return nil
	}

	for _, instance := range handlers {
		instance.handler(ctx, evt)
	}

	return nil
}
