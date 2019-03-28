package gatekeeper

import (
	"fmt"
	"time"

	"./cache"
	"./timeslice"
	"./types"
)

type ClientRule struct {
	id                    string
	quota                 int // only quota is overridden
	clientId              string
	overridenCommonRuleId string
}

type CommonRule struct {
	id         string
	resourceId string
	quota      int
	interval   int
}

type Event struct {
	resourceId string
	clientId   string
}

var localMap *types.Map

func getCurrentTimeWindow(interval int) string {
	// lookup the cache
	lookupKey := fmt.Sprintf("time_interval_%d", interval)
	val, resCode := localMap.Get(lookupKey)
	if resCode == types.HIT {
		return val
	} else {
		currentWindow := timeslice.GetTimeWindow(interval)
		localMap.Put(lookupKey, currentWindow, time.Duration(interval)*time.Second)
		return currentWindow
	}
}

func getTracker(inst Event, ruleId string, interval int) string {
	window := getCurrentTimeWindow(interval)
	return fmt.Sprintf("%s_%s_%s_%s", window, inst.clientId, inst.resourceId, ruleId)
}

func (r *ApiRateLimiter) getCommonRuleById(id string) (CommonRule, bool) {
	if cmr, ok := r.commonRulesIdxById[id]; ok {
		return cmr, true
	} else {
		return CommonRule{}, false
	}
}

func (r *ApiRateLimiter) findMatchingClientRules(inst Event) []ClientRule {
	result := []ClientRule{}
	for _, clientRule := range r.clrules {
		cmr, ok := r.getCommonRuleById(clientRule.overridenCommonRuleId)
		if ok && cmr.resourceId == inst.resourceId {
			result = append(result, clientRule)
		}
	}
	return result
}

func (r *ApiRateLimiter) findMatchingCommonRules(evt Event) []CommonRule {
	if cmr, ok := r.commonRulesIdxByResourceId[evt.resourceId]; ok {
		return []CommonRule{cmr}
	} else {
		return []CommonRule{}
	}
}

func removeOverriddenCommonRules(commonRules []CommonRule, clientRules []ClientRule) []CommonRule {
	matching := []CommonRule{}
	for _, commonRule := range commonRules {
		matches := false
		for _, clientRule := range clientRules {
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

type StoreType int

const (
	STORE_REDIS StoreType = iota
	STORE_MEMORY
	STORE_SYNCED_MEMORY
)

type Result struct {
	hasBreached    bool
	breachedRuleId string
	quota          int
	currentCount   int
}

type RateLimiter interface {
	// TODO
	// AddCommonRules(cmrules []CommonRule)
	// AddClientRules(clrules []ClientRule)
	RecordEventAndCheck(evt Event) Result
}

type ApiRateLimiter struct {
	commonRulesIdxById         map[string]CommonRule
	commonRulesIdxByResourceId map[string]CommonRule
	cmrules                    []CommonRule
	clrules                    []ClientRule
	store                      cache.Store
}

func init() {
	// store = cache.NewRedisStore(*cache.DevConfig())
	// store = cache.NewCache(time.Duration(300 * time.Second))
	localMap = types.NewMap()
}

func NewApiRateLimiter(cmrs []CommonRule, clrs []ClientRule, storeType StoreType) *ApiRateLimiter {
	maxTTL := time.Duration(300 * time.Second)
	var store cache.Store
	if storeType == STORE_REDIS {
		store = cache.NewRedisStore(*cache.DevConfig())
	} else if storeType == STORE_SYNCED_MEMORY {
		config := cache.SyncMemoryConfig{MaxTTL: maxTTL, FlushInterval: time.Duration(1 * time.Second)}
		store = cache.NewSyncedMemory(&config, cache.DevConfig())
	} else if storeType == STORE_MEMORY {
		store = cache.NewCache(time.Duration(300 * time.Second))
	}
	limiter := ApiRateLimiter{cmrules: cmrs, clrules: clrs}
	limiter.store = store
	limiter.commonRulesIdxById = make(map[string]CommonRule)
	limiter.commonRulesIdxByResourceId = make(map[string]CommonRule)
	for _, cmr := range cmrs {
		limiter.commonRulesIdxById[cmr.id] = cmr
		limiter.commonRulesIdxByResourceId[cmr.resourceId] = cmr
	}
	return &limiter
}

func (r *ApiRateLimiter) RecordEventAndCheck(inst Event) Result {
	matchingCommonRules := r.findMatchingCommonRules(inst)
	matchingClientRules := r.findMatchingClientRules(inst)
	prunedCommonRules := removeOverriddenCommonRules(matchingCommonRules, matchingClientRules)
	// now we have to execute the match against common & client specific
	// all matching rules are fair game
	for _, cmr := range prunedCommonRules {
		trackId := getTracker(inst, cmr.id, cmr.interval)
		val := r.store.IncrAndGet(trackId)
		// fmt.Printf("Current count is %s :: %d, quota is %d\n" , trackId, val, cmr.quota)
		if val > cmr.quota {
			// this is a breach
			return returnBreach(cmr.id, cmr.quota, val)
		}
	}

	for _, clr := range matchingClientRules {
		cmr, _ := r.getCommonRuleById(clr.overridenCommonRuleId)
		trackId := getTracker(inst, clr.id, cmr.interval)
		val := r.store.IncrAndGet(trackId)
		// fmt.Printf("Current count is %s :: %d, quota is %d\n" , trackId, val, clr.quota)
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
