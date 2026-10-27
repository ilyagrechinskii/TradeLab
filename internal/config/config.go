package config

import (
	"errors"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port           string
	AllowedOrigins []string
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

	origins := make([]string, 0)
	if value := os.Getenv("CORS_ALLOWED_ORIGINS"); value != "" {
		for _, origin := range strings.Split(value, ",") {
			origin = strings.TrimSpace(origin)
			u, err := url.Parse(origin)
			if err != nil || u == nil || (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || strings.Contains(u.Host, "*") || u.Opaque != "" {
				return Config{}, errors.New("CORS_ALLOWED_ORIGINS must contain comma-separated HTTP(S) origins without paths or wildcards")
			}
			origins = append(origins, origin)
		}
	}
	return Config{Port: strconv.Itoa(number), AllowedOrigins: origins}, nil
}
