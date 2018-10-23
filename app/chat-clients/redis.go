package main

import (
	"fmt"
	"time"
)

const TIME_FORMAT = "2006-01-02 15:04:05.999999999 -0700 MST"

func PingPods(env *env) {
	for {
		if env.config.debug {
			env.redisClient.HSet("pods", env.config.podName, time.Now().Format(TIME_FORMAT))

		} else {
			env.redisClusterClient.HSet("pods", env.config.podName, time.Now().Format(TIME_FORMAT))
		}

		time.Sleep(time.Second * 2)
	}
}
func DeleteSleepingPods(env *env) {
	for {
		hashMap := env.redisClient.HGetAll("pods").Val()
		for k, v := range hashMap {
			fmt.Printf("k: %v\nv: %v\n\n", k, v)
			lastCheckin, err := time.Parse(TIME_FORMAT, v)
			if err != nil {
				env.log.Criticalf("Error parsing time. Deleting offending entry(%s): %v", k, err)
				if env.config.debug {

					env.redisClient.HDel("pods", k)
				} else {
					env.redisClusterClient.HDel("pods", k)
				}
				continue
			}
			if time.Now().Sub(lastCheckin).Seconds() >= 10 {
				if env.config.debug {

					env.redisClient.HDel("pods", k)
				} else {
					env.redisClusterClient.HDel("pods", k)
				}
			}
		}
		time.Sleep(time.Second * 2)
	}
}

func GetPods(env *env) []string {
	if env.config.debug {
		return env.redisClient.HKeys("pods").Val()
	} else {
		return env.redisClusterClient.HKeys("pods").Val()
	}
}
