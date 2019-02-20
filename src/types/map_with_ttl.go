package types

import (
  "fmt"
  "time"
  // "reflect"
)

type Map struct {
  store map[string]string
  ttlStore map[string]time.Time
}

type ResultCode int
const (
  HIT ResultCode = iota
  MISS
)

type Result struct {
  resCode ResultCode
  value string
}

func (m *Map) hasExpired(originalKey string) bool {
  ttlValue := m.ttlStore[ttlKey(originalKey)]
  return time.Now().After(ttlValue)
}

func NewMap() *Map {
  return &Map{store : make(map[string]string), ttlStore : make(map[string]time.Time)}
}

func ttlKey(key string) string {
  return fmt.Sprintf("__ttl___%s", key)
}

func (m *Map) Put(key string, value string, duration time.Duration) {
  // we need to make two entries
  m.store[key] = value
  m.ttlStore[ttlKey(key)] = time.Now().Add(duration)
}

func (m *Map) Get(key string) (string, ResultCode) {
  if val, ok := m.store[key]; ok {
    // get the TTL
    if m.hasExpired(key) {
      // delete the key
      delete(m.store, key)
      delete(m.ttlStore, ttlKey(key))
      return "", MISS
    } else {
      return val, HIT
    }
  } else {
    return "", MISS
  }
}
