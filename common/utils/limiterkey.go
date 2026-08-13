package utils

import (
	"strconv"
)

func BuildUserIdKey(prefix string, userId int64) string {
	return prefix + ":userId:" + strconv.FormatInt(userId, 10)
}

func BuildIPKey(prefix, ip string) string {
	return prefix + ":ip:" + ip
}

func BuildStringKey(prefix, value string) string {
	return prefix + ":" + value
}
