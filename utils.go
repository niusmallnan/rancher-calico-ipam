package main

import (
	"fmt"
	"net"

	"github.com/Sirupsen/logrus"
	"github.com/containernetworking/cni/pkg/skel"
	"github.com/rancher/rancher-cni-ipam/ipfinder/metadata"
)

func setIpByRancher(args *skel.CmdArgs, ipamArgs *ipamArgs) error {
	ipf, err := metadata.NewIPFinderFromMetadata()
	if err != nil {
		return err
	}
	ipString := ipf.GetIP(args.ContainerID, string(ipamArgs.RancherContainerUUID))
	if len(ipString) > 0 {
		logrus.Debugf("rancher-calico-ipam: %s", fmt.Sprintf("ip: %#v", ipString))
		ip, _, err := net.ParseCIDR(ipString + "/32")
		if err != nil {
			return err
		}
		ipamArgs.IP = ip
	}
	return nil
}
