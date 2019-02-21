package gatekeeper

import (
  "testing"
  "time"
  "fmt"
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
  inst := Instance{resourceId: "api/call1", clientId: "dp1"}
  for i:=0;i<times;i++ {
    res := RecordInstanceAndCheck(inst, cmrules, clrules)
    result = res
  }
}

func BenchmarkEvents1k(b *testing.B) { benchmarkRateCounting(1 * 1000, b) }
func BenchmarkEvents10k(b *testing.B) { benchmarkRateCounting(10 * 1000, b) }
func BenchmarkEvents100k(b *testing.B) { benchmarkRateCounting(100 * 10000, b) }
func BenchmarkEvents1mn(b *testing.B) { benchmarkRateCounting(1000 * 10000, b) }
// 5million takes >100s to run on a typical laptop
// func BenchmarkEvents5mn(b *testing.B) { benchmarkRateCounting(5000 * 10000, b) }

// func BenchmarkRateCounting10k()

func TestBreachAndReset(t *testing.T) {

  cmrules := getCommonRules()
  clrules := getClientRules()

  rule1 := cmrules[0]

  inst := Instance{resourceId: "api/call1", clientId: "dp1"}

  for i:=0;i<40;i++ {
    result := RecordInstanceAndCheck(inst, cmrules, clrules)
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
  result := RecordInstanceAndCheck(inst, cmrules, clrules)
  if result.hasBreached == true {
    t.Fatalf("The count is not clearing as expected")
  }
}
