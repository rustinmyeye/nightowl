package state

import (
	"context"
	"fmt"
	"sync"

	"github.com/go-redis/redis/v9"
)

const (
	notConfirmedRedisKey = "confirmed:false"
)

type NotifState struct {
	NotConfirmed map[string]bool

	ctx context.Context
	rdb *redis.Client
	mu sync.Mutex
}

func NewNotifState(ctx context.Context, rdb *redis.Client) *NotifState {
	return &NotifState{
		NotConfirmed: make(map[string]bool),

		ctx: ctx,
		rdb: rdb,
	}
}

func (ns *NotifState) DBSync() error {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	var err error
	var utxos []string

	utxos, err = ns.rdb.SMembers(ns.ctx, notConfirmedRedisKey).Result()
	if err != nil {
		return fmt.Errorf("failed to get not confirmed txs from redis db - %s", err.Error())
	}

	for _, notConf := range utxos {
		ns.NotConfirmed[notConf] = true
	}

	return nil
}

func (ns *NotifState) AddNotConfirmed(betKey string) error {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	
	var err error

	ns.NotConfirmed[betKey] = true

	err = ns.rdb.SAdd(ns.ctx, notConfirmedRedisKey, betKey).Err()
	if err != nil {
		return fmt.Errorf("failed to add not confirmed tx to redis db key - %s - %s", notConfirmedRedisKey, err.Error())
	}

	return nil
}

func (ns *NotifState) RemoveNotConfirmed(betKey string) error {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	
	var err error

	delete(ns.NotConfirmed, betKey)

	err = ns.rdb.SRem(ns.ctx, notConfirmedRedisKey, betKey).Err()
	if err != nil {
		return fmt.Errorf("failed to remove not confirmed tx from redis db key - %s - %s", notConfirmedRedisKey, err.Error())
	}

	return nil
}

