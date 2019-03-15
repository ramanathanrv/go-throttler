package gatekeeper

import (
  "testing"
  "time"
  "fmt"
  "log"
  "./cache"
)

func getCommonRules() []CommonRule {
  rule1 := CommonRule{id: "cr1", resourceId: "api/call1", quota: 60, interval: 10}
  rule2 := CommonRule{id: "cr2", resourceId: "api/call2", quota: 10, interval: 10}
  rule3 := CommonRule{id: "cr3", resourceId: "api/call3", quota: 20, interval: 10}
  cmrules := []CommonRule{rule1, rule2, rule3}
  for i:=4;i<100;i++ {
    ruleId := fmt.Sprintf("cr%d", i)
    resourceId := fmt.Sprintf("api/call%d", i)
    rule := CommonRule{id: ruleId, resourceId: resourceId, quota: 50, interval: i}
    cmrules = append(cmrules, rule)
  }
  return cmrules
}

func getClientRules() []ClientRule {
  clrule1 := ClientRule {id: "cl1", clientId: "dp1", quota: 20, overridenCommonRuleId: "cr1"}
  clrules := []ClientRule{clrule1}
  return clrules
}

var result Result

func benchmarkRateCounting(times int, b *testing.B) {
  cmrules := getCommonRules()
  clrules := getClientRules()
  inst := Event{resourceId: "api/call1", clientId: "dp1"}
  limiter := NewApiRateLimiter(cmrules, clrules)
  for i:=0;i<times;i++ {
    res := limiter.RecordEventAndCheck(inst)
    result = res
  }
}

func BenchmarkEvents1k(b *testing.B) { benchmarkRateCounting(1 * 1000, b) }
func BenchmarkEvents10k(b *testing.B) { benchmarkRateCounting(10 * 1000, b) }
func BenchmarkEvents100k(b *testing.B) { benchmarkRateCounting(100 * 1000, b) }
func BenchmarkEvents1mn(b *testing.B) { benchmarkRateCounting(1000 * 1000, b) }
func BenchmarkEvents5mn(b *testing.B) { benchmarkRateCounting(5 * 1000 * 1000, b) }
// 5million takes >100s to run on a typical laptop
// func BenchmarkEvents5mn(b *testing.B) { benchmarkRateCounting(5000 * 10000, b) }

func TestBreachAndReset(t *testing.T) {

  cmrules := getCommonRules()
  clrules := getClientRules()
  limiter := NewApiRateLimiter(cmrules, clrules)

  rule1 := cmrules[0]

  inst := Event{resourceId: "api/call1", clientId: "dp1"}

  for i:=0;i<40;i++ {
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

var logger *log.Logger
func TestCacheCleanup(t *testing.T) {
  var c = cache.NewCache(time.Duration(30 * time.Second))
  key := "test_key"
  for i := 0;i<10;i++ {
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
    c cache.Store 
    val int
  )
  c = cache.NewRedisStore(cache.RedisConfig{Host: "localhost", Port: 6379, DB: 0, Password: ""})
  for i:=0;i<10;i++ {
    val = c.IncrAndGet("hello")
  }
  fmt.Println("Value is: ", val)
} 
