package cache

import (
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"time"

	"../types"
	"github.com/go-redis/redis"
)

type SyncMemoryConfig struct {
	MaxTTL        time.Duration
	FlushInterval time.Duration
	host          string
}

const (
	streamName string = "go-throttler"
)

type SyncedMemory struct {
	localMap          *types.RevolvingMap
	globalHostDataMap *types.RevolvingMap // [data_point] => [host] => value
	redisClient       *redis.Client
	config            *SyncMemoryConfig
	// internal
	lastReadStreamID string
}

// GetLocalIP returns the non loopback local IP of the host
func GetLocalIP() string {
	hostconf := os.Getenv("HOST")
	if len(hostconf) > 0 {
		return hostconf
	}
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, address := range addrs {
		// check the address type and if it is not a loopback the display it
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return ""
}

// NewSyncedMemory - constructs a new instance of SyncedMemory
func NewSyncedMemory(syncConfig *SyncMemoryConfig, redisConfig *RedisConfig) *SyncedMemory {
	localMap := types.NewRevolvingMap(syncConfig.MaxTTL)
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", redisConfig.Host, redisConfig.Port),
		Password: redisConfig.Password,
		DB:       redisConfig.DB,
	})
	globalDataMap := types.NewRevolvingMap(syncConfig.MaxTTL)
	syncConfig.host = GetLocalIP()

	sm := &SyncedMemory{localMap: localMap, redisClient: client, config: syncConfig, globalHostDataMap: globalDataMap}
	sm.initializeStreamPointer() // blocking operation
	go sm.scheduleFlush()
	go sm.scheduleReadFromStream()
	return sm
}

// IncrAndGet - increment the value pertaining to the given key
func (sm *SyncedMemory) IncrAndGet(key string) int {
	val, ok := sm.localMap.GetInt(key)
	gval := sm.GetGlobalCount(key)
	log.Println("Global value is: ", gval)
	if ok {
		sm.localMap.PutInt(key, val+1)
		return val + gval + 1
	}
	sm.localMap.PutInt(key, 1)
	return gval + 1
}

func (sm *SyncedMemory) GetGlobalCount(key string) int {
	hostLevelCount, ok := sm.globalHostDataMap.Get(key)
	if ok {
		var result int = 0
		hostMap := hostLevelCount.(*types.RevolvingMap)
		keys := hostMap.Keys()
		for _, key := range keys {
			if key == nil {
				continue
			}
			res, ok := hostMap.Get(key.(string))
			if ok {
				result += res.(int)
			}
		}
		return result
	}
	return 0
}

type flatEntry struct {
	key string
	val int
}

func (sm *SyncedMemory) scheduleFlush() {
	nextTime := time.Now().Truncate(time.Second)
	nextTime = nextTime.Add(sm.config.FlushInterval)
	time.Sleep(time.Until(nextTime))
	go sm.flush()
	go sm.scheduleFlush()
}

// this is a blocking call
// consider using channel for implementing this in a non-blocking way
func (sm *SyncedMemory) initializeStreamPointer() {
	ping := map[string]interface{}{"ping": "pong"}
	args := redis.XAddArgs{Values: ping, Stream: streamName}
	res, err := sm.redisClient.XAdd(&args).Result()

	if err != nil {
		log.Fatalf("Unable to obtain stream pointer. Unrecoverable error")
	}
	sm.lastReadStreamID = res
}

func (sm *SyncedMemory) scheduleReadFromStream() {
	nextTime := time.Now().Truncate(time.Second)
	nextTime = nextTime.Add(sm.config.FlushInterval)
	time.Sleep(time.Until(nextTime))
	go sm.readFromStream()
	go sm.scheduleReadFromStream()
}

func (sm *SyncedMemory) readFromStream() {
	log.Println("Beginning read from Redis stream")
	args := redis.XReadArgs{Count: 100, Streams: []string{streamName, sm.lastReadStreamID}}
	res, err := sm.redisClient.XRead(&args).Result()
	if err != nil {
		log.Println("Error while trying to read from stream. Rate limiting ability impaired.")
		return
	}
	// sample result
	// [{go-throttler [{1553681118002-0 map[host:172.16.44.151 2019/03/03_15:35:10_dp1_api/call1_cl1:40]}]}]
	// we expect only one entry as we are explicitly sending the stream name
	var streamMap redis.XStream = res[0]
	var streamEntries []redis.XMessage = streamMap.Messages
	var lastKnownID string
	var currentHost string = sm.config.host
	log.Println("Current host: ", currentHost)
	for _, entry := range streamEntries {
		// XMessage {ID string, Values map[string]interface{}}
		lastKnownID = entry.ID
		values := entry.Values
		host, hasHost := values["host"]
		if hasHost == false {
			// skip this
			continue
		}
		if host == currentHost {
			continue
		}
		log.Printf("Processing %+v\n", values)
		for k, v := range values {
			if k == "host" {
				continue
			}
			_, ok := sm.globalHostDataMap.Get(k)
			if ok == false { // new datapoint that we are seeing for the first time
				sm.globalHostDataMap.Put(k, types.NewRevolvingMap(sm.config.MaxTTL))
			}
			m, _ := sm.globalHostDataMap.Get(k)
			rmap := m.(*types.RevolvingMap)
			intVal, err := strconv.Atoi(v.(string))
			if err != nil {
				log.Printf("Cannot convert %s to int value. Impaired performance of the system.", v)
			} else {
				rmap.PutInt(host.(string), intVal)
			}
		}
	}
	// log.Printf("%+v\n", sm.globalHostDataMap)
	sm.lastReadStreamID = lastKnownID
	log.Println("Completed read from Redis stream")
}

// flush - pushes the local data into Redis Stream
func (sm *SyncedMemory) flush() {
	log.Println("Beginning Flush")
	// ip := GetLocalIP()
	internalMap, lock := sm.localMap.GetCurrentMapWithLock()
	pipe := sm.redisClient.Pipeline()
	lock.RLock()
	defer lock.RUnlock()
	dataPoints := make([]flatEntry, len(*internalMap))
	count := 0
	for k, v := range *internalMap {
		dataPoints[count] = flatEntry{key: k.(string), val: v.(int)}
		count = count + 1
	}
	// TODO - now stream it to Redis
	ip := sm.config.host
	totalDataPoints := len(dataPoints)
	chunkSize := 100
	chunks := totalDataPoints / chunkSize
	leftOver := totalDataPoints % chunkSize
	for i := 0; i < chunks; i++ {
		valueMap := make(map[string]interface{})
		valueMap["host"] = ip
		for j := 0; j < chunkSize; j++ {
			cur := i*chunkSize + j
			entry := dataPoints[cur]
			valueMap[entry.key] = entry.val
		}
		xargs := &redis.XAddArgs{Values: valueMap}
		pipe.XAdd(xargs)
	}

	valueMap := make(map[string]interface{})
	valueMap["host"] = ip
	for i := 0; i < leftOver; i++ {
		entry := dataPoints[i]
		valueMap[entry.key] = entry.val
	}
	xargs := &redis.XAddArgs{Values: valueMap, Stream: streamName}
	pipe.XAdd(xargs)
	_, err := pipe.Exec()
	if err != nil {
		log.Println("Error while streaming data via Redis Pipe")
	} else {
		log.Println("Completed flushing")
	}
}
