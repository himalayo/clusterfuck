package main

import (
	"sync"

	"google.golang.org/grpc/resolver"
)

var (
	ResolverManager = NewResolverManager()
)

type ClusterfuckResolver struct {
	target resolver.Target
	cc     resolver.ClientConn
}

type ClusterfuckResolverManager struct {
	addresses map[string][]string
	resolvers map[string][]*ClusterfuckResolver
	mu        sync.Mutex
}

func NewResolverManager() *ClusterfuckResolverManager {
	return &ClusterfuckResolverManager{
		addresses: make(map[string][]string),
		resolvers: make(map[string][]*ClusterfuckResolver),
	}
}

func (man *ClusterfuckResolverManager) AddAddress(addr string, app string) {
	man.mu.Lock()
	defer man.mu.Unlock()
	for _, a := range man.addresses[app] {
		if a == addr {
			return
		}
	}
	man.addresses[app] = append(man.addresses[app], addr)
	curr_add := man.addresses[app]
	addrs := make([]resolver.Address, len(curr_add))
	for i, add := range curr_add {
		addrs[i] = resolver.Address{Addr: add}
	}
	state := resolver.State{Addresses: addrs}
	for _, res := range man.resolvers[app] {
		res.cc.UpdateState(state)
	}
}

type ClusterfuckResolverBuilder struct{}

func (*ClusterfuckResolverBuilder) Build(target resolver.Target, cc resolver.ClientConn, _ resolver.BuildOptions) (resolver.Resolver, error) {
	res := &ClusterfuckResolver{
		target: target,
		cc:     cc,
	}
	res.start()
	ResolverManager.mu.Lock()
	ResolverManager.resolvers[target.Endpoint()] = append(ResolverManager.resolvers[target.Endpoint()], res)
	ResolverManager.mu.Unlock()
	return res, nil
}

func (*ClusterfuckResolverBuilder) Scheme() string {
	return "clusterfuck"
}

func (*ClusterfuckResolver) ResolveNow(resolver.ResolveNowOptions) {}
func (*ClusterfuckResolver) Close()                                {}

func (r *ClusterfuckResolver) start() {
	addrsOriginal := ResolverManager.addresses[r.target.Endpoint()]
	addrs := make([]resolver.Address, len(addrsOriginal))
	for i, s := range addrsOriginal {
		addrs[i] = resolver.Address{Addr: s}
	}
	r.cc.UpdateState(resolver.State{Addresses: addrs})
}

func initResolving() {
	resolver.Register(&ClusterfuckResolverBuilder{})
}
