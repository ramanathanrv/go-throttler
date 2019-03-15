package cache

type Store interface {
	IncrAndGet(key string) int 
}