package metadata

import (
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/rancher/go-rancher-metadata/metadata"
)

const (
	metadataURL         = "http://169.254.169.250/2015-12-19"
	multiplierForTwoMin = 240
	emptyIPAddress      = ""
)

// IPFinderFromMetadata is used to hold information related to
// Metadata client and other stuff.
type IPFinderFromMetadata struct {
	m *metadata.Client
}

// NewIPFinderFromMetadata returns a new instance of the IPFinderFromMetadata
func NewIPFinderFromMetadata() (*IPFinderFromMetadata, error) {
	m, err := metadata.NewClientAndWait(metadataURL)
	if err != nil {
		return nil, err
	}
	return &IPFinderFromMetadata{m}, nil
}

// GetIP returns the IP address for the given container id, return an empty string
// if not found
func (ipf *IPFinderFromMetadata) GetIP(cid, rancherid string) string {
	for i := 0; i < multiplierForTwoMin; i++ {
		containers, err := ipf.m.GetContainers()
		if err != nil {
			log.Errorf("rancher-cni-ipam: Error getting metadata containers: %v", err)
			return emptyIPAddress
		}

		for _, container := range containers {
			if container.ExternalId == cid && container.PrimaryIp != "" {
				log.Infof("rancher-cni-ipam: got ip: %v", container.PrimaryIp)
				return container.PrimaryIp
			}
			if rancherid != "" && container.UUID == rancherid && container.PrimaryIp != "" {
				log.Infof("rancher-cni-ipam: got ip from rancherid: %v", container.PrimaryIp)
				return container.PrimaryIp
			}
		}
		log.Infof("Waiting to find IP for container: %s, %s", cid, rancherid)
		time.Sleep(500 * time.Millisecond)
	}
	log.Infof("ip not found for cid: %v", cid)
	return emptyIPAddress
}
