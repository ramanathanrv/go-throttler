package main

import (
    "./cache"
    "./timeslice"
    "fmt"
    "time"
)

type ClientRule struct {
  id string;
  quota uint; // only quota is overridden
  clientId string;
  overridenCommonRuleId string;
}

type CommonRule struct {
  id string;
  resourceId string;
  quota uint;
  interval int;
}

type Instance struct {
  resourceId string;
  clientId string;
}

func getTimeWindowId(seconds int) {
  if(seconds <= 60) {
    // we need to go to the nearest minute and then slice it
  }
}

func getTracker(inst Instance, ruleId string, interval int) string {
  window := timeslice.GetTimeWindow(interval)
  return fmt.Sprintf("%s_%s_%s_%s", window, inst.clientId, inst.resourceId, ruleId)
}

func getCommonRuleById(id string, allRules []CommonRule) CommonRule {
  for _,commonRule := range allRules {
    if commonRule.id == id {
      return commonRule
    }
  }
  return CommonRule{}
}

func findMatchingClientRules(inst Instance, allRules []ClientRule, cmrules []CommonRule) []ClientRule {
  result := []ClientRule{}
  for _,clientRule := range allRules {
    cmr := getCommonRuleById(clientRule.overridenCommonRuleId, cmrules)
    if cmr.resourceId == inst.resourceId {
      result = append(result, clientRule)
    }
  }
  return result
}

func findMatchingCommonRules(inst Instance, allRules []CommonRule) []CommonRule {
  result := []CommonRule{}
  for _, commonRule := range allRules {
    if commonRule.resourceId == inst.resourceId {
      result = append(result, commonRule)
    }
  }
  return result
}

func removeOverriddenCommonRules(commonRules []CommonRule, clientRules []ClientRule) []CommonRule {
  matching := []CommonRule{}
  for _,commonRule := range commonRules {
    matches := false
    for _,clientRule := range clientRules {
      if clientRule.overridenCommonRuleId == commonRule.id {
        matches = true
      }
    }
    if matches == false {
      matching = append(matching, commonRule)
    }
  }
  return matching
}


func recordInstanceAndCheck(inst Instance, cmrules []CommonRule, clrules []ClientRule) bool {
  matchingCommonRules := findMatchingCommonRules(inst, cmrules)
  matchingClientRules := findMatchingClientRules(inst, clrules, cmrules)
  prunedCommonRules := removeOverriddenCommonRules(matchingCommonRules, matchingClientRules)
  // now we have to execute the match against common & client specific
  // all matching rules are fair game
  for _,cmr := range prunedCommonRules {
    trackId := getTracker(inst, cmr.id, cmr.interval)
    val := cache.IncrAndGet(trackId)
    fmt.Printf("Current count is %s :: %d\n" , trackId, val)
  }

  for _,clr := range matchingClientRules {
    cmr := getCommonRuleById(clr.overridenCommonRuleId, cmrules)
    trackId := getTracker(inst, clr.id, cmr.interval)
    val := cache.IncrAndGet(trackId)
    fmt.Println("Current count is %s :: %d" , trackId, val)
  }
  return true
}

func main() {
  cache.InitCache()
  rule1 := CommonRule{id: "cr1", resourceId: "api/getBill", quota: 60, interval: 10}
  rule2 := CommonRule{id: "cr2", resourceId: "api/createBill", quota: 10, interval: 10}
  rule3 := CommonRule{id: "cr3", resourceId: "api/validateBill", quota: 20, interval: 10}
  cmrules := []CommonRule{rule1, rule2, rule3}

  clrule1 := ClientRule {id: "cl1", clientId: "dp1", quota: 20, overridenCommonRuleId: "cr2"}
  clrules := []ClientRule{clrule1}

  inst := Instance{resourceId: "api/getBill", clientId: "dp1"}

  for i:=0;i<100;i++ {
    recordInstanceAndCheck(inst, cmrules, clrules)
    time.Sleep(1000 * time.Millisecond)
  }

}
// read rules from the files
