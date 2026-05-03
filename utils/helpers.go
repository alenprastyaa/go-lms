package utils

import (
	"fmt"
	"strconv"
	"strings"
)

func ToInt(s string, def int) int {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return def
	}
	return n
}

func ToString(v interface{}) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

func StringPtr(v interface{}) *string {
	s := strings.TrimSpace(ToString(v))
	if s == "" {
		return nil
	}
	return &s
}
