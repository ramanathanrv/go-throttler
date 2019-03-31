package cache

import (
	"fmt"
	"time"

	"../types"
	"github.com/go-redis/redis"
)

// This store uses redis stream as the communication between the participating nodes.
// Every node publishes its status to the Redis Stream every once in a while (say 500ms).
// As all the nodes listen to Redis Stream, they continuously update the global status.
// Any decision is arrived after putting together the information from all the nodes.
// Note that this is an approximate solution and is prone to certain inaccuracies. Also, the penalty is realtime.
// This solution sacrifices accuracy & realtime(ness) for performance and throughput and resource optimization.

// RedisBackedMemory - This is pre-dominantly a memory based data structure with inputs from Redis pertaining
// to the global state of the counters
type RedisBackedMemory struct {
	cacheMapA map[string]string
	cacheMapB map[string]string

	hostDataMapA map[string]map[string]int
	// internal fields
	lastCleaned     string
	cleanupInterval time.Duration
}

type streamingRedisStore struct {
	client      *redis.Client
	checkMap    *types.Map
	hostDataMap *types.Map
}

// NewStreamingRedisStore - create a new streaming redis store
func NewStreamingRedisStore(config RedisConfig) *streamingRedisStore {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", config.Host, config.Port),
		Password: config.Password,
		DB:       config.DB,
	})
	checkMap := types.NewMap()
	return &streamingRedisStore{client: client, checkMap: checkMap}
}

func (r *streamingRedisStore) checkIfExists(key string) types.ResultCode {
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

func (r *streamingRedisStore) setTtlIfRequired(key string) int {
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

func (r *streamingRedisStore) IncrAndGet(key string) int {
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
