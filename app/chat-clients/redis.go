package main

import (
	"context"
	"fmt"
	"time"
)

const TIME_FORMAT = "2006-01-02 15:04:05.999999999 -0700 MST"

func PingPods(env *env, freq time.Duration) {
	ticker := time.NewTicker(freq)
	defer ticker.Stop()
	for tick := range ticker.C {
		if env.ShuttingDown {
			return
		}
		if env.isLocal() {
			fmt.Println("Pod tick at", tick)
			env.redisClient.HSet("pods", env.config.podName, tick.Format(TIME_FORMAT))
		} else {
			env.redisClusterClient.HSet("pods", env.config.podName, tick.Format(TIME_FORMAT))
		}
	}
}
func PingTeams(env *env, freq time.Duration) {
	ticker := time.NewTicker(freq)
	defer ticker.Stop()
	for tick := range ticker.C {
		if env.ShuttingDown {
			return
		}
		teams, err := getAllTeams(context.Background(), env)
		if err != nil {
			env.log.Criticalf("could not get teams from datastore: %s", err)
			continue
		}
		for teamID := range teams {
			go func() {
				if env.isLocal() {
					fmt.Println("Teams tick at", tick)
					env.redisClient.HSet("teams", teamID, tick.Format(TIME_FORMAT))
				} else {
					env.redisClusterClient.HSet("teams", teamID, tick.Format(TIME_FORMAT))
				}
			}()
		}
	}
}

func Reap(key string, env *env, freq time.Duration, age time.Duration) {
	ticker := time.NewTicker(freq)
	defer ticker.Stop()
	for range ticker.C {
		if env.ShuttingDown {
			return
		}
		var hashMap map[string]string
		if env.isLocal() {
			hashMap = env.redisClient.HGetAll(key).Val()
		} else {
			hashMap = env.redisClusterClient.HGetAll(key).Val()
		}
		for k, v := range hashMap {
			if env.isLocal() {
				fmt.Printf("k: %v\nv: %v\n\n", k, v)
			}
			lastCheckin, err := time.Parse(TIME_FORMAT, v)
			if err != nil {
				env.log.Criticalf("Error parsing time. Deleting offending entry(%s): %v", k, err)
				if env.isLocal() {
					env.redisClient.HDel(key, k)
				} else {
					env.redisClusterClient.HDel(key, k)
				}
				continue
			}
			if time.Now().Sub(lastCheckin) >= age {
				if env.isLocal() {
					env.redisClient.HDel(key, k)
				} else {
					res := env.redisClusterClient.HDel(key, k)
					fmt.Printf("DelResult: %+v", res)
				}
			}
		}
	}
}

func GetHashKeys(key string, env *env) []string {
	if env.isLocal() {
		return env.redisClient.HKeys(key).Val()
	} else {
		return env.redisClusterClient.HKeys(key).Val()
	}
}
func AssignTeamToPod(env *env, teamID string, podName string, expirey time.Duration) error {
	key := fmt.Sprintf("team-assignment:%s", teamID)
	var err error
	if env.isLocal() {
		err = env.redisClient.Set(key, podName, expirey).Err()
	} else {
		err = env.redisClusterClient.Set(key, podName, expirey).Err()
	}
	if err != nil {
		return err
	}
	return nil
}
func (env *env) GetTeamsAssignedtoPod() ([]string, error) {
	teams := GetHashKeys("teams", env)
	var (
		assignedTeams []string
		podName       string
		err           error
	)
	for _, teamID := range teams {
		key := fmt.Sprintf("team-assignment:%s", teamID)

		if env.isLocal() {
			podName, err = env.redisClient.Get(key).Result()
		} else {
			podName, err = env.redisClusterClient.Get(key).Result()
		}
		if err != nil {
			return nil, err
		}
		if podName == env.config.podName {
			assignedTeams = append(assignedTeams, teamID)
		}
	}
	return assignedTeams, nil
}

func (env *env) DeletePod() {
	if env.isLocal() {
		env.redisClient.HDel("pods", env.config.podName)
	} else {
		env.redisClusterClient.HDel("pods", env.config.podName)
	}
}
