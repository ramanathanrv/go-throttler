package gatekeeper

import (
    "./cache"
    "./timeslice"
    "fmt"
    "time"
    "./types"
)

type ClientRule struct {
  id string;
  quota int; // only quota is overridden
  clientId string;
  overridenCommonRuleId string;
}

type CommonRule struct {
  id string;
  resourceId string;
  quota int;
  interval int;
}

type Instance struct {
  resourceId string;
  clientId string;
}

var localMap *types.Map

func getTimeWindowId(seconds int) {
  if(seconds <= 60) {
    // we need to go to the nearest minute and then slice it
  }
}

func getCurrentTimeWindow(interval int) string {
  // lookup the cache
  lookupKey := fmt.Sprintf("time_interval_%d", interval)
  val, resCode := localMap.Get(lookupKey)
  if resCode == types.HIT {
    return val
  } else {
    currentWindow := timeslice.GetTimeWindow(interval)
    localMap.Put(lookupKey, currentWindow, time.Duration(interval) * time.Second)
    return currentWindow
  }
}

func getTracker(inst Instance, ruleId string, interval int) string {
  window := getCurrentTimeWindow(interval)
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

type Result struct {
  hasBreached bool;
  breachedRuleId string;
  quota int;
  currentCount int;
}

type RateLimiter interface {
  clearCache()
  addRule()
  recordEvent()
}

func init() {
  cache.InitCache()
  localMap = types.NewMap()
}

func RecordInstanceAndCheck(inst Instance, cmrules []CommonRule, clrules []ClientRule) Result {
  matchingCommonRules := findMatchingCommonRules(inst, cmrules)
  matchingClientRules := findMatchingClientRules(inst, clrules, cmrules)
  prunedCommonRules := removeOverriddenCommonRules(matchingCommonRules, matchingClientRules)
  // now we have to execute the match against common & client specific
  // all matching rules are fair game
  for _,cmr := range prunedCommonRules {
    trackId := getTracker(inst, cmr.id, cmr.interval)
    val := cache.IncrAndGet(trackId)
    fmt.Printf("Current count is %s :: %d, quota is %d\n" , trackId, val, cmr.quota)
    if val > cmr.quota {
      // this is a breach
      return returnBreach(cmr.id, cmr.quota, val)
    }
  }

  for _,clr := range matchingClientRules {
    cmr := getCommonRuleById(clr.overridenCommonRuleId, cmrules)
    trackId := getTracker(inst, clr.id, cmr.interval)
    val := cache.IncrAndGet(trackId)
    fmt.Printf("Current count is %s :: %d, quota is %d\n" , trackId, val, clr.quota)
    if val > clr.quota {
      // this is a breach
      return returnBreach(clr.id, clr.quota, val)
    }
  }
  return returnNoBreach()
}

func returnBreach(ruleId string, quota int, currentCount int) Result {
  return Result{hasBreached: true, breachedRuleId: ruleId, quota: quota, currentCount: currentCount}
}

func returnNoBreach() Result {
  return Result{hasBreached: false}
}
