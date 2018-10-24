package main

import (
	"os"
	"strconv"
)

type envReader struct {
	missingKeys []string
	errors      bool
}

func (r *envReader) getEnv(key string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	r.errors = true
	r.missingKeys = append(r.missingKeys, key)
	return ""
}
func (r *envReader) getEnvOpt(key string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return ""
}
func (r *envReader) getEnvBool(key string) bool {
	text := r.getEnv(key)
	if value, err := strconv.ParseBool(text); err == nil {
		return value
	}
	return false
}
func (r *envReader) getEnvBoolOpt(key string) bool {
	text := r.getEnvOpt(key)
	if value, err := strconv.ParseBool(text); err == nil {
		return value
	}
	return false
}
func (r *envReader) getEnvFloat(key string) float64 {
	text := r.getEnv(key)

	if value, err := strconv.ParseFloat(text, 64); err != nil {
		return value
	}
	return 0
}
