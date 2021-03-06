package types

import (
	"fmt"
	"sync"
	"time"
)

type mapPtr int

var lock = sync.RWMutex{}

const (
	mapA mapPtr = iota
	mapB
)

type RevolvingMap struct {
	mapA            map[interface{}]interface{}
	mapB            map[interface{}]interface{}
	lastCleaned     mapPtr
	maxTTL          time.Duration
	cleanupInterval time.Duration
}

func (m *RevolvingMap) cleaner() {
	nextTime := time.Now().Truncate(time.Second)
	nextTime = nextTime.Add(m.cleanupInterval)
	time.Sleep(time.Until(nextTime))
	fmt.Println("Performing cleanup now: ", time.Now())
	if m.lastCleaned == mapA {
		m.mapB = make(map[interface{}]interface{})
		fmt.Println("Cleaned up B")
		m.lastCleaned = mapB
	} else {
		m.mapA = make(map[interface{}]interface{})
		fmt.Println("Cleaned up A")
		m.lastCleaned = mapA
	}
	go m.cleaner()
}

// NewRevolvingMap - returns a new instance of the RevolvingMap
func NewRevolvingMap(maxTTL time.Duration) *RevolvingMap {
	m := RevolvingMap{
		mapA:            make(map[interface{}]interface{}),
		mapB:            make(map[interface{}]interface{}),
		maxTTL:          maxTTL,
		cleanupInterval: maxTTL + maxTTL, // set the cleanupInterval longer
		lastCleaned:     mapA,
	}
	go m.cleaner()
	return &m
}

// GetCurrentMap - exposes the inner map
func (m *RevolvingMap) GetCurrentMap() map[interface{}]interface{} {
	return m.mapA
}

func (m *RevolvingMap) GetCurrentMapWithLock() (*map[interface{}]interface{}, *sync.RWMutex) {
	currentMap := m.getCurrentlyActiveMap()
	return currentMap, &lock
}

// PutInt - puts the given integer into the map
func (m *RevolvingMap) PutInt(key string, val int) int {
	lock.Lock()
	defer lock.Unlock()
	m.mapA[key] = val
	m.mapB[key] = val
	return val
}

// Put - generic put command to add any value to the map
func (m *RevolvingMap) Put(key string, val interface{}) interface{} {
	lock.Lock()
	defer lock.Unlock()
	m.mapA[key] = val
	m.mapB[key] = val
	return val
}

// GetInt - gets the value as int after applying type assertion
func (m *RevolvingMap) GetInt(key string) (int, bool) {
	currentMap := m.getCurrentlyActiveMap()
	lock.RLock()
	defer lock.RUnlock()
	val, ok := (*currentMap)[key]
	if ok {
		// https://stackoverflow.com/questions/18041334/convert-interface-to-int
		iVal, convOk := val.(int)
		return iVal, convOk
	}
	return -1, false
}

// Get - generic Get command to read any value from the map
func (m *RevolvingMap) Get(key string) (interface{}, bool) {
	lock.RLock()
	defer lock.RUnlock()
	currentMap := m.getCurrentlyActiveMap()
	val, ok := (*currentMap)[key]
	return val, ok
}

func (m *RevolvingMap) getCurrentlyActiveMap() *map[interface{}]interface{} {
	if m.lastCleaned == mapA {
		return &m.mapB
	} else {
		return &m.mapA
	}
}

// Keys - returns the keys in the map as an array
func (m *RevolvingMap) Keys() []interface{} {
	currentMap := m.getCurrentlyActiveMap()
	lock.RLock()
	defer lock.RUnlock()
	var keys []interface{} = make([]interface{}, len(m.mapA))
	for k, _ := range *currentMap {
		keys = append(keys, k)
	}
	return keys
}
