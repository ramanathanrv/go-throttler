package cache

import (
    "strconv"
    // "fmt"
)

var cacheMap map[string]string

func InitCache() {
  if cacheMap == nil {
    cacheMap = make(map[string]string)
  }
}

func IncrAndGet(key string) int {
  i := getAsInt(key, 0)
  put(key, strconv.Itoa(i+1))
  return i+1
}

func put(key string, value string) {
  cacheMap[key] = value
}

func get(key string) string {
  return cacheMap[key]
}

func getAsInt(key string, defaultVal int) int {
  if val, ok := cacheMap[key]; ok {
    if intval, err := strconv.Atoi(val); (err == nil) {
      return intval
    }
  }
  return defaultVal
}
