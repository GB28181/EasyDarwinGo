package utils

import (
	"crypto/md5"
	"fmt"
)

// MD5 of string
func MD5(s string) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(s)))
}
