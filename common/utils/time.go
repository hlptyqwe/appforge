package utils

import "time"

func NowMillis() int64 {
	return time.Now().UnixMilli()
}
