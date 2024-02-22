package shutter

import (
	"errors"

	shclient "github.com/ethereum-optimism/optimism/shutter-node/grpc/v1/client"
)

type Config struct {
	ServerAddress string
}

func (c *Config) Check() error {
	if c.ServerAddress == "" {
		return errors.New("server address missing")
	}
	return nil
}

func (c *Config) Setup() (*shclient.Client, error) {
	client, err := shclient.NewClient(shclient.WithServerAddress(c.ServerAddress))
	if err != nil {
		return nil, err
	}
	return client, nil
}
