package main

import (
	"log"
	"strconv"
	"strings"
)

func stringToInt64(stringToInt string) int64 {
	parsedInt, err := strconv.ParseInt(stringToInt, 10, 64)
	if err != nil {
		log.Fatal("Error converting string to int64: ", err)
	}
	return parsedInt
}

func checkIfItemIsKeyOfArray(item string, array []string) bool {
	for _, arrayItem := range array {
		if item == arrayItem {
			return true
		}
	}
	return false
}

func ip2long(ip string) uint32 {
	ipLong, _ := strconv.ParseUint(strings.Join(strings.Split(ip, "."), ""), 10, 32)
	return uint32(ipLong)
}
