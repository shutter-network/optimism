package p2p

import (
	shp2p "github.com/shutter-network/rolling-shutter/rolling-shutter/p2p"
)

// TODO: needed?
func NewP2P(config *shp2p.Config) (*shp2p.P2PHandler, error) {
	return shp2p.New(config)
}
