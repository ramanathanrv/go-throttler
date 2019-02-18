package timeslice

import (
  // "fmt"
  "time"
)

// greatest common divisor (GCD) via Euclidean algorithm
func gcd(a, b int) int {
  for b != 0 {
    t := b
    b = a % b
    a = t
  }
  return int(a)
}

func lcm(a, b int, integers ...int) int {
  result := a * b / gcd(a, b)
  for i := 0; i < len(integers); i++ {
    result = lcm(result, integers[i])
  }
  return result
}

func lcm64(a, b int) int64 {
  return int64(lcm(a, b))
}

func GetTimeWindow(timeslice int) string {
  now := time.Now()
  unix := now.Unix()
  epoch := now.Unix()

  // find the big window
  windowLength := lcm64(60, timeslice)
  windowsSinceEpoch := epoch/windowLength
  epoch -= ((windowsSinceEpoch * windowLength) + (epoch % windowLength))

  // now find the small window
  windows := int(unix - epoch) / (timeslice)
  currentWindow := epoch + int64(windows * timeslice)
  windowStr := time.Unix(currentWindow, 0).Format("2006/01/01_15:04:05")
  return windowStr
}
