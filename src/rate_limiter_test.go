package gatekeeper

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"./cache"
)

func getCommonRules() []CommonRule {
	rule1 := CommonRule{id: "cr1", resourceId: "api/call1", quota: 60, interval: 10}
	rule2 := CommonRule{id: "cr2", resourceId: "api/call2", quota: 10, interval: 10}
	rule3 := CommonRule{id: "cr3", resourceId: "api/call3", quota: 20, interval: 10}
	cmrules := []CommonRule{rule1, rule2, rule3}
	for i := 4; i < 100; i++ {
		ruleId := fmt.Sprintf("cr%d", i)
		resourceId := fmt.Sprintf("api/call%d", i)
		rule := CommonRule{id: ruleId, resourceId: resourceId, quota: 50, interval: i}
		cmrules = append(cmrules, rule)
	}
	return cmrules
}

func getClientRules() []ClientRule {
	clrule1 := ClientRule{id: "cl1", clientId: "dp1", quota: 20, overridenCommonRuleId: "cr1"}
	clrules := []ClientRule{clrule1}
	return clrules
}

var result Result

func benchmarkRateCounting(times int, b *testing.B) {
	cmrules := getCommonRules()
	clrules := getClientRules()
	inst := Event{resourceId: "api/call1", clientId: "dp1"}
	limiter := NewApiRateLimiter(cmrules, clrules, STORE_MEMORY)
	for i := 0; i < times; i++ {
		res := limiter.RecordEventAndCheck(inst)
		result = res
	}
}

func benchmarkRateCountingOnStore(times int, storeType StoreType, b *testing.B) {
        log.Println("Times: ", times)
	cmrules := getCommonRules()
	clrules := getClientRules()
	limiter := NewApiRateLimiter(cmrules, clrules, storeType)
	for i := 0; i < times; i++ {
		newinst := Event{resourceId: fmt.Sprintf("api/call%d", 1+i%100), clientId: "dp1"}
		res := limiter.RecordEventAndCheck(newinst)
		result = res
	}
}

func init() {
        s := os.Getenv("SILENT")
        if len(s) > 0 {
	  log.SetOutput(ioutil.Discard)
        }
}
func BenchmarkEvents1k(b *testing.B)   { benchmarkRateCounting(1*1000, b) }
func BenchmarkEvents10k(b *testing.B)  { benchmarkRateCounting(10*1000, b) }
func BenchmarkEvents100k(b *testing.B) { benchmarkRateCounting(100*1000, b) }
func BenchmarkEvents1mn(b *testing.B)  { benchmarkRateCounting(1000*1000, b) }
func BenchmarkEvents100kWithMemory(b *testing.B) {
	benchmarkRateCountingOnStore(100*1000, STORE_MEMORY, b)
}
func BenchmarkEvents100kWithRedis(b *testing.B) {
	benchmarkRateCountingOnStore(100*1000, STORE_REDIS, b)
}
func BenchmarkEvents100kWithSyncMemoryOnce(b *testing.B) {
        log.Println("Benchmarking for 100k")
	benchmarkRateCountingOnStore(100000, STORE_SYNCED_MEMORY, b)
}
func BenchmarkEvents1mnWithMemory(b *testing.B) {
	benchmarkRateCountingOnStore(1000*1000, STORE_MEMORY, b)
}
func BenchmarkEvents1mnWithSyncMemory(b *testing.B) {
	benchmarkRateCountingOnStore(1 * 1000 * 1000, STORE_SYNCED_MEMORY, b)
}
func BenchmarkEvents1mnWithRedis(b *testing.B) {
	wg := new(sync.WaitGroup)
	concurrency := os.Getenv("CONCURRENCY")
	if len(concurrency) > 0 {
		cf, err := strconv.Atoi(concurrency)
		if err != nil {
			b.Fatal("Concurrency must be a valid number")
		}
		chunkSize := 1000000 / cf
		for i := 0; i < cf; i++ {
			wg.Add(1)
			go func(waitgroup *sync.WaitGroup) {
				defer waitgroup.Done()
				benchmarkRateCountingOnStore(chunkSize, STORE_REDIS, b)
			}(wg)
		}
		log.Println("Waiting for the routines to finish")
		wg.Wait()
	} else {
		benchmarkRateCountingOnStore(1000*1000, STORE_REDIS, b)
	}
}
func BenchmarkEvents5mn(b *testing.B) { benchmarkRateCounting(5*1000*1000, b) }

// 5million takes >100s to run on a typical laptop
// func BenchmarkEvents5mn(b *testing.B) { benchmarkRateCounting(5000 * 10000, b) }

func TestBreachAndReset(t *testing.T) {
	fmt.Println("Commencing test for breach & reset")
	cmrules := getCommonRules()
	clrules := getClientRules()
	limiter := NewApiRateLimiter(cmrules, clrules, STORE_SYNCED_MEMORY)
	rule1 := cmrules[0]

	inst := Event{resourceId: "api/call1", clientId: "dp1"}

	for i := 0; i < 40; i++ {
		result := limiter.RecordEventAndCheck(inst)
		time.Sleep(1 * time.Millisecond)
		// given quota is 20. So breach is when i exceeds 20
		if i > 20 {
			var expectedResult = true
			var actualResult = result.hasBreached
			if actualResult != expectedResult {
				t.Fatalf("Expected %t but got %t", expectedResult, actualResult)
			}
		}
	}
	fmt.Printf("Waiting for %d seconds\n", rule1.interval)
	time.Sleep(time.Duration(10) * time.Second)
	result := limiter.RecordEventAndCheck(inst)
	if result.hasBreached == true {
		t.Fatalf("The count is not clearing as expected")
	}
}

func isEqual(expected interface{}, actual interface{}, t *testing.T) {
	if expected == actual {
		// all good
	} else {
		t.Fatalf("Expected %v but go %v", expected, actual)
	}
}
func TestSyncMemoryStrategy(t *testing.T) {
	rule1 := CommonRule{id: "cr1", resourceId: "api/call1", quota: 20, interval: 60}
	cmrules := []CommonRule{rule1}
	clrules := []ClientRule{}
	inst := Event{resourceId: "api/call1", clientId: "dp1"}

	os.Setenv("HOST", "H1")
	limiter1 := NewApiRateLimiter(cmrules, clrules, STORE_SYNCED_MEMORY)
	result1 := limiter1.RecordEventAndCheck(inst)

	os.Setenv("HOST", "H2")
	limiter2 := NewApiRateLimiter(cmrules, clrules, STORE_SYNCED_MEMORY)
	result2 := limiter2.RecordEventAndCheck(inst)

	isEqual(1, result1.currentCount, t)
	isEqual(1, result2.currentCount, t)

	time.Sleep(4 * time.Second)

	// BOTH of them should have synced now
	result11 := limiter1.RecordEventAndCheck(inst)
	result22 := limiter2.RecordEventAndCheck(inst)
	isEqual(3, result11.currentCount, t)
	isEqual(3, result22.currentCount, t)

	// breach in limiter1, ensure it reflects in limiter2
	for i := 0; i < 25; i++ {
		limiter1.RecordEventAndCheck(inst)
	}
	b1 := limiter1.RecordEventAndCheck(inst)
	isEqual(true, b1.hasBreached, t)
	log.Printf("%+v\n", b1)
	// sleep now & allow time for sync (min 2 seconds)
	for i := 0; i < 4; i++ {
		time.Sleep(1 * time.Second)
		limiter2.RecordEventAndCheck(inst)
	}
	b2 := limiter2.RecordEventAndCheck(inst)
	log.Printf("%+v\n", b2)
	isEqual(true, b2.hasBreached, t)
}

var logger *log.Logger

func TestCacheCleanup(t *testing.T) {
	var c = cache.NewCache(time.Duration(30 * time.Second))
	key := "test_key"
	for i := 0; i < 10; i++ {
		c.IncrAndGet(key)
	}
	fmt.Println("Current Value is: ", c.IncrAndGet(key))
	fmt.Println(time.Now())
	fmt.Println("Sleeping for 20 seconds")
	time.Sleep(20 * time.Second)
	var expectedResult = 12
	var actualResult = c.IncrAndGet(key)
	fmt.Println("Value is: ", actualResult)
	if actualResult != expectedResult {
		t.Fatalf("Expected %d but got %d", expectedResult, actualResult)
	}

	fmt.Println("Sleeping for 40 seconds")
	time.Sleep(40 * time.Second)

	expectedResult = 1
	actualResult = c.IncrAndGet(key)
	fmt.Println("Value is: ", actualResult)
	if actualResult != expectedResult {
		t.Fatalf("Expected %d but got %d", expectedResult, actualResult)
	}
}

func TestRedisStoreInit(t *testing.T) {
	var (
		c   cache.Store
		val int
	)
	c = cache.NewRedisStore(cache.RedisConfig{Host: "127.0.0.1", Port: 6379, DB: 0, Password: ""})
	for i := 0; i < 10; i++ {
		val = c.IncrAndGet("hello")
	}
	fmt.Println("Value is: ", val)
}
