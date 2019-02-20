package gatekeeper

import (
  "testing"
  "time"
  "fmt"
)

func TestBreachAndReset(t *testing.T) {
  rule1 := CommonRule{id: "cr1", resourceId: "api/call1", quota: 60, interval: 10}
  rule2 := CommonRule{id: "cr2", resourceId: "api/call2", quota: 10, interval: 10}
  rule3 := CommonRule{id: "cr3", resourceId: "api/call3", quota: 20, interval: 10}
  cmrules := []CommonRule{rule1, rule2, rule3}

  clrule1 := ClientRule {id: "cl1", clientId: "dp1", quota: 20, overridenCommonRuleId: "cr1"}
  clrules := []ClientRule{clrule1}

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
