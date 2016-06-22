package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
)

type Config struct {
	Cube        string `json:"cube"`
	Addr        string `json:"addr"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	Label       string `json:"label"`
	GreenIfMore uint32 `json:"green-if-more"`
	RedIfMore   uint32 `json:"red-if-more"`
}

func LoadConfig(path string) (*Config, error) {
	file, err := ioutil.ReadFile(path)
	if err != nil {
		fmt.Printf("Unable to open config file '%s': %s\n", path, err)
		return nil, err
	}

	var config Config
	err = json.Unmarshal(file, &config)
	if err != nil {
		fmt.Printf("Malformed config file '%s': %s\n", path, err)
		return nil, err
	}

	return &config, nil
}
