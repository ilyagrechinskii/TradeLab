package config

import (
	"errors"
	"os"
	"strconv"
)

type Config struct {
	Port string
}

func Load() (Config, error) {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	number, err := strconv.Atoi(port)
	if err != nil || number < 1 || number > 65535 {
		return Config{}, errors.New("PORT must be an integer between 1 and 65535")
	}

	return Config{Port: strconv.Itoa(number)}, nil
}
