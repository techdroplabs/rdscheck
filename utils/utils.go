package utils

import (
	"os"
	"strconv"
	"time"
)

// GetEnvString returns a string from the provided environment variable
func GetEnvString(envVar string, defaults string) string {
	value := os.Getenv(envVar)
	if value == "" {
		return defaults
	}
	return value
}

// GetEnvInt returns an int from the provided environment variable.
func GetEnvInt(envVar string, defaults int) int {
	value := os.Getenv(envVar)
	if value == "" {
		return defaults
	}
	i, err := strconv.Atoi(value)
	if err != nil {
		panic(err)
	}
	return i
}

// GetUnixTimeAsString returns the current unix time as a string
func GetUnixTimeAsString() string {
	currentTime := time.Now().Unix()
	str := strconv.FormatInt(currentTime, 10)
	return str
}
