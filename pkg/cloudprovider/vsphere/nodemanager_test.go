/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package vsphere

import (
	"context"
	"strings"
	"testing"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	pb "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphere/proto"
	cm "k8s.io/cloud-provider-vsphere/pkg/common/connectionmanager"
)

func TestRegUnregNode(t *testing.T) {
	cfg, ok := configFromEnvOrSim()
	defer ok()

	vsphere, err := newVSphere(cfg)
	if err != nil {
		t.Fatalf("Failed to construct/authenticate vSphere: %s", err)
	}
	vsphere.connectionManager = cm.NewConnectionManager(&cfg, nil)

	nm := newNodeManager(vsphere.connectionManager, nil)

	vm := simulator.Map.Any("VirtualMachine").(*simulator.VirtualMachine)
	name := vm.Name
	UUID := vm.Config.Uuid
	k8sUUID := ConvertK8sUUIDtoNormal(UUID)

	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Status: v1.NodeStatus{
			NodeInfo: v1.NodeSystemInfo{
				SystemUUID: k8sUUID,
			},
		},
	}

	nm.RegisterNode(node)

	if len(nm.nodeNameMap) != 1 {
		t.Errorf("Failed: nodeNameMap should be a length of 1")
	}
	if len(nm.nodeUUIDMap) != 1 {
		t.Errorf("Failed: nodeUUIDMap should be a length of  1")
	}
	if len(nm.nodeRegUUIDMap) != 1 {
		t.Errorf("Failed: nodeRegUUIDMap should be a length of  1")
	}

	nm.UnregisterNode(node)

	if len(nm.nodeNameMap) != 1 {
		t.Errorf("Failed: nodeNameMap should be a length of  1")
	}
	if len(nm.nodeUUIDMap) != 1 {
		t.Errorf("Failed: nodeUUIDMap should be a length of  1")
	}
	if len(nm.nodeRegUUIDMap) != 0 {
		t.Errorf("Failed: nodeRegUUIDMap should be a length of 0")
	}
}

type SearchIndex struct {
	*simulator.SearchIndex
	vm *simulator.VirtualMachine
}

func (s *SearchIndex) FindByDnsName(req *types.FindByDnsName) soap.HasFault {
	res := &methods.FindByDnsNameBody{Res: new(types.FindByDnsNameResponse)}
	if req.VmSearch && strings.EqualFold(req.DnsName, s.vm.Name) {
		res.Res.Returnval = &s.vm.Self
	}
	return res
}

func TestDiscoverNodeByName(t *testing.T) {
	cfg, ok := configFromEnvOrSim()
	defer ok()

	vsphere, err := newVSphere(cfg)
	if err != nil {
		t.Errorf("Failed to construct/authenticate vSphere: %s", err)
	}
	vsphere.connectionManager = cm.NewConnectionManager(&cfg, nil)
	defer vsphere.connectionManager.Logout()

	nm := newNodeManager(vsphere.connectionManager, nil)

	vm := simulator.Map.Any("VirtualMachine").(*simulator.VirtualMachine)
	name := vm.Name

	vsi := nm.connectionManager.VsphereInstanceMap[cfg.Global.VCenterIP]
	err = nm.connectionManager.Connect(context.Background(), cfg.Global.VCenterIP)
	if err != nil {
		t.Errorf("Failed to Connect to vSphere: %s", err)
	}

	search := object.NewSearchIndex(vsi.Conn.Client)
	si := simulator.Map.Get(search.Reference()).(*simulator.SearchIndex)
	simulator.Map.Put(&SearchIndex{si, vm})

	err = nm.DiscoverNode(name, FindVMByName)
	if err != nil {
		t.Errorf("Failed DiscoverNode: %s", err)
	}

	if len(nm.nodeNameMap) != 1 {
		t.Errorf("Failed: nodeNameMap should be a length of 1")
	}
	if len(nm.nodeUUIDMap) != 1 {
		t.Errorf("Failed: nodeUUIDMap should be a length of  1")
	}
}

func TestExport(t *testing.T) {
	cfg, ok := configFromEnvOrSim()
	defer ok()

	vsphere, err := newVSphere(cfg)
	if err != nil {
		t.Fatalf("Failed to construct/authenticate vSphere: %s", err)
	}
	vsphere.connectionManager = cm.NewConnectionManager(&cfg, nil)
	defer vsphere.connectionManager.Logout()

	nm := newNodeManager(vsphere.connectionManager, nil)

	vm := simulator.Map.Any("VirtualMachine").(*simulator.VirtualMachine)
	name := vm.Name
	UUID := vm.Config.Uuid
	k8sUUID := ConvertK8sUUIDtoNormal(UUID)

	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Status: v1.NodeStatus{
			NodeInfo: v1.NodeSystemInfo{
				SystemUUID: k8sUUID,
			},
		},
	}

	nm.RegisterNode(node)

	nodeList := make([]*pb.Node, 0)
	_ = nm.ExportNodes("", "", &nodeList)

	found := false
	for _, node := range nodeList {
		if node.Uuid == UUID {
			found = true
		}
	}

	if !found {
		t.Errorf("Node was not converted to protobuf")
	}

	nm.UnregisterNode(node)
}
