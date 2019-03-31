# go-throttler
Simple rate limiter implementation in Golang. Provides multiple strategies with different trade-offs.

# Dependencies

```
go get -u github.com/go-redis/redis // Redis driver
```

# Test
To run the benchmark on your machine, use the following command inside the source directory.
```
go test -bench=. -cpuprofile cpu.prof -memprofile mem.prof
```
