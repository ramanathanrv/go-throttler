package cache

import (
    "strconv"
    "fmt"
    "time"
)

type Cache struct {
  cacheMapA map[string]string
  cacheMapB map[string]string

  // internal fields
  lastCleaned string
  cleanupInterval time.Duration
}

type Store interface {
  IncrAndGet(key string) int 
}

func max(x, y int) int {
  if x > y {
      return x
  }
  return y
}

func NewCache(reloadInterval time.Duration) *Cache {
  var newInstance *Cache = &Cache{
    cacheMapA : make(map[string]string), 
    cacheMapB : make(map[string]string),
    cleanupInterval : reloadInterval, // sufficiently larger value to ensure that we don't delete live data 
    lastCleaned : "A",
  }
  fmt.Println("Returning Cache struct from NewCache")
  go newInstance.cleaner()
  return newInstance
}


func (c *Cache) cleaner() {
  nextTime := time.Now().Truncate(time.Second)
  nextTime = nextTime.Add(c.cleanupInterval)
  time.Sleep(time.Until(nextTime))
  fmt.Println("Performing cleanup now: ", time.Now())
  if c.lastCleaned == "A" {
    c.cacheMapB = make(map[string]string)
    fmt.Println("Cleaned up B")
    c.lastCleaned = "B"
  } else {
    c.cacheMapA = make(map[string]string)
    fmt.Println("Cleaned up A")
    c.lastCleaned = "A"
  }
  go c.cleaner()
}

func (c *Cache) IncrAndGet(key string) int {
  i1 := getAsInt(c.cacheMapA, key, 0)
  i2 := getAsInt(c.cacheMapB, key, 0)
  i := max(i1, i2)
  c.put(key, strconv.Itoa(i+1))
  return i+1
}

func (c *Cache) put(key string, value string) {
  c.cacheMapA[key] = value
  c.cacheMapB[key] = value
}

func getAsInt(cacheMap map[string]string, key string, defaultVal int) int {
  if val, ok := cacheMap[key]; ok {
    if intval, err := strconv.Atoi(val); (err == nil) {
      return intval
    }
  }
  return defaultVal
}
