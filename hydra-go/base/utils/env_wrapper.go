package utils

import "os"

func EnvWrapper(env, value string) func() {
	original := os.Getenv(env)
	os.Setenv(env, value)
	return func() {
		os.Setenv(env, original)
	}
}
