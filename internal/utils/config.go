package utils

import "encoding/json"

type Config struct {
	Email       string `json:"email"`
	Credentials []byte `json:"credentials"`
}

func LoadConfig(paths []string) (*Config, error) {

	c := &Config{}

	b, err := GetFileContents(paths)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(b, c)
	if err != nil {
		return nil, err
	}

	return c, nil
}
