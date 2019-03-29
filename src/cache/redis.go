package cache

import (
	"fmt"
	"os"
	"time"

	"../types"
	"github.com/go-redis/redis"
)

type RedisConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
}

type redisStore struct {
	client   *redis.Client
	checkMap *types.Map
}

func DevConfig() *RedisConfig {
	redisHost := os.Getenv("REDIS_HOST")
	conf := RedisConfig{
		Host:     "127.0.0.1",
		Port:     6379,
		DB:       0,
		Password: "",
	}
	if len(redisHost) > 0 {
		conf.Host = redisHost
	}
	return &conf
}

func NewRedisStore(config RedisConfig) *redisStore {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", config.Host, config.Port),
		Password: config.Password,
		DB:       config.DB,
	})
	checkMap := types.NewMap()
	return &redisStore{client: client, checkMap: checkMap}
}

func getMaxAllowedTime() time.Duration {
	// this is constant. we are setting it to maximum possible instead of being accurate.
	// trade off: compute vs memory
	return time.Duration(300 * time.Second)
}

func (r *redisStore) checkIfExists(key string) types.ResultCode {
	_, err := r.client.Get(key).Result()
	if err == redis.Nil {
		// key doesn't exist
		return types.MISS
	} else if err != nil {
		// some issue with redis
		// swallow it for now
		return types.HIT // avoid going to redis much
	} else {
		return types.HIT
	}
}

func (r *redisStore) setTtlIfRequired(key string) int {
	_, ok := r.checkMap.Get(key)
	if ok != types.HIT {
		// its a miss. we need to check Redis
		resultCode := r.checkIfExists(key)
		if resultCode != types.HIT {
			// doesn't exist in redis
			r.client.SetXX(key, 1, getMaxAllowedTime())
			r.checkMap.Put(key, "0", getMaxAllowedTime())
			return 1
		}
		return 0
	}
	return 0
}

func (r *redisStore) IncrAndGet(key string) int {
	result := r.setTtlIfRequired(key)
	if result == 1 {
		// new entrant
		return result
	}
	val, err := r.client.Incr(key).Result()
	if err != nil {
		// something went wrong
		// we will swallow the error and respond with 0
		return 0
	} else {
		return int(val)
	}
}
