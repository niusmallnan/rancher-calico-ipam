// Copyright (c) 2016 Tigera, Inc. All rights reserved.

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package client

import (
	"encoding/hex"

	log "github.com/Sirupsen/logrus"

	"github.com/projectcalico/libcalico-go/lib/api"
	"github.com/projectcalico/libcalico-go/lib/api/unversioned"
	"github.com/projectcalico/libcalico-go/lib/backend/model"
	"github.com/projectcalico/libcalico-go/lib/errors"
	"github.com/projectcalico/libcalico-go/lib/net"
	"github.com/satori/go.uuid"
)

// NodeInterface has methods to work with Node resources.
type NodeInterface interface {
	List(api.NodeMetadata) (*api.NodeList, error)
	Get(api.NodeMetadata) (*api.Node, error)
	Create(*api.Node) (*api.Node, error)
	Update(*api.Node) (*api.Node, error)
	Apply(*api.Node) (*api.Node, error)
	Delete(api.NodeMetadata) error
}

// nodes implements NodeInterface
type nodes struct {
	c *Client
}

// newNodes returns a new NodeInterface bound to the supplied client.
func newNodes(c *Client) NodeInterface {
	return &nodes{c}
}

// Create creates a new node.
func (h *nodes) Create(a *api.Node) (*api.Node, error) {
	// When creating a new node, initialize global defaults if they are
	// not yet initiaized.
	err := h.initGlobalDefaults()
	if err != nil {
		return nil, err
	}
	return a, h.c.create(*a, h)
}

// Update updates an existing node.
func (h *nodes) Update(a *api.Node) (*api.Node, error) {
	return a, h.c.update(*a, h)
}

// Apply updates a node if it exists, or creates a new node if it does not exist.
func (h *nodes) Apply(a *api.Node) (*api.Node, error) {
	// When creating a new node, initialize global defaults if they are
	// not yet initiaized.
	err := h.initGlobalDefaults()
	if err != nil {
		return nil, err
	}
	return a, h.c.apply(*a, h)
}

// Delete deletes an existing node.
func (h *nodes) Delete(metadata api.NodeMetadata) error {
	// Make sure all workload endpoint configuration is deleted, and any IPs
	// that were assigned to these endpoints are deleted.  We check that the
	// node name has been specified, otherwise we'd end up listing all
	// endpoints across all nodes, and delete their config.
	if metadata.Name == "" {
		return errors.ErrorInsufficientIdentifiers{Name: "node"}
	}
	log.Debugf("Deleting node: %s", metadata.Name)

	// List endpoints.
	eps, err := h.c.WorkloadEndpoints().List(api.WorkloadEndpointMetadata{Node: metadata.Name})
	if err != nil {
		return err
	}

	// Collate all IPs across all endpoints, and then release those IPs.
	ips := []net.IP{}
	for _, ep := range eps.Items {
		for _, nw := range ep.Spec.IPNetworks {
			ips = append(ips, net.IP{nw.IP})
		}
	}

	log.Debugf("Releasing the following IPs from workload endpoints: %v", ips)
	_, err = h.c.IPAM().ReleaseIPs(ips)
	if err != nil {
		return err
	}

	// Remove the node from the IPAM data if it exists.
	log.Debug("Removing IPAM host data")
	err = h.c.IPAM().RemoveIPAMHost(metadata.Name)
	if err != nil {
		log.Debug("Error removing host data: %v", err)
		if _, ok := err.(errors.ErrorResourceDoesNotExist); ok {
			return err
		}
	}

	// Finally remove the node.
	return h.c.delete(metadata, h)
}

// Get returns information about a particular node.
func (h *nodes) Get(metadata api.NodeMetadata) (*api.Node, error) {
	if a, err := h.c.get(metadata, h); err != nil {
		return nil, err
	} else {
		return a.(*api.Node), nil
	}
}

// List takes a Metadata, and returns a NodeList that contains the list of nodes
// that match the Metadata (wildcarding missing fields).
func (h *nodes) List(metadata api.NodeMetadata) (*api.NodeList, error) {
	l := api.NewNodeList()
	err := h.c.list(metadata, h, l)
	return l, err
}

// convertMetadataToListInterface converts a NodeMetadata to a NodeListOptions.
// This is part of the conversionHelper interface.
func (h *nodes) convertMetadataToListInterface(m unversioned.ResourceMetadata) (model.ListInterface, error) {
	nm := m.(api.NodeMetadata)
	l := model.NodeListOptions{
		Hostname: nm.Name,
	}
	return l, nil
}

// convertMetadataToKey converts a NodeMetadata to a NodeKey
// This is part of the conversionHelper interface.
func (h *nodes) convertMetadataToKey(m unversioned.ResourceMetadata) (model.Key, error) {
	nm := m.(api.NodeMetadata)
	k := model.NodeKey{
		Hostname: nm.Name,
	}
	return k, nil
}

// convertAPIToKVPair converts an API Node structure to a KVPair containing a
// backend Node and NodeKey.
// This is part of the conversionHelper interface.
func (h *nodes) convertAPIToKVPair(a unversioned.Resource) (*model.KVPair, error) {
	an := a.(api.Node)
	k, err := h.convertMetadataToKey(an.Metadata)
	if err != nil {
		return nil, err
	}

	v := model.Node{}
	if an.Spec.BGP != nil {
		v.BGPIPv4 = an.Spec.BGP.IPv4Address
		v.BGPIPv6 = an.Spec.BGP.IPv6Address
		v.BGPASNumber = an.Spec.BGP.ASNumber
	}

	return &model.KVPair{Key: k, Value: &v}, nil
}

// convertKVPairToAPI converts a KVPair containing a backend Node and NodeKey
// to an API Node structure.
// This is part of the conversionHelper interface.
func (h *nodes) convertKVPairToAPI(d *model.KVPair) (unversioned.Resource, error) {
	bv := d.Value.(*model.Node)
	bk := d.Key.(model.NodeKey)

	apiNode := api.NewNode()
	apiNode.Metadata.Name = bk.Hostname

	if bv.BGPIPv4 != nil || bv.BGPIPv6 != nil {
		apiNode.Spec.BGP = &api.NodeBGPSpec{
			IPv4Address: bv.BGPIPv4,
			IPv6Address: bv.BGPIPv6,
			ASNumber:    bv.BGPASNumber,
		}
	}

	return apiNode, nil
}

// initGlobalDefaults initializes any global default value that is not yet
// initialized, this processing will not overwrite keys that are already
// present in the datastore.
func (h *nodes) initGlobalDefaults() error {
	// Make sure the Ready flag is initialized in the datastore
	_, err := h.c.backend.Create(&model.KVPair{
		Key:   model.ReadyFlagKey{},
		Value: true,
	})
	if err != nil {
		if _, ok := err.(errors.ErrorResourceAlreadyExists); !ok {
			return err
		}
	}

	// Make sure we have a global cluster ID set.
	_, err = h.c.backend.Create(&model.KVPair{
		Key:   model.GlobalConfigKey{Name: "ClusterGUID"},
		Value: hex.EncodeToString(uuid.NewV4().Bytes()),
	})
	if err != nil {
		if _, ok := err.(errors.ErrorResourceAlreadyExists); !ok {
			return err
		}
	}
	return nil
}
