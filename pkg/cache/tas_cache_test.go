/*
Copyright 2024 The Kubernetes Authors.

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

package cache

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kueue "sigs.k8s.io/kueue/apis/kueue/v1beta1"
	tasindexer "sigs.k8s.io/kueue/pkg/controller/tas/indexer"
	"sigs.k8s.io/kueue/pkg/resources"
	utiltesting "sigs.k8s.io/kueue/pkg/util/testing"
	testingpod "sigs.k8s.io/kueue/pkg/util/testingjobs/pod"
)

func TestFindTopologyAssignment(t *testing.T) {
	const (
		tasBlockLabel = "cloud.com/topology-block"
		tasRackLabel  = "cloud.com/topology-rack"
	)

	//      b1                   b2
	//   /      \             /      \
	//  r1       r2          r1       r2
	//  |      /  |  \       |         |
	//  x1    x2  x3  x4     x5       x6
	defaultNodes := []corev1.Node{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "b1-r1-x1",
				Labels: map[string]string{
					tasBlockLabel:        "b1",
					tasRackLabel:         "r1",
					corev1.LabelHostname: "x1",
				},
			},
			Status: corev1.NodeStatus{
				Allocatable: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
				Conditions: []corev1.NodeCondition{
					{
						Type:   corev1.NodeReady,
						Status: corev1.ConditionTrue,
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "b1-r2-x2",
				Labels: map[string]string{
					tasBlockLabel:        "b1",
					tasRackLabel:         "r2",
					corev1.LabelHostname: "x2",
				},
			},
			Status: corev1.NodeStatus{
				Allocatable: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
				Conditions: []corev1.NodeCondition{
					{
						Type:   corev1.NodeReady,
						Status: corev1.ConditionTrue,
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "b1-r2-x3",
				Labels: map[string]string{
					tasBlockLabel:        "b1",
					tasRackLabel:         "r2",
					corev1.LabelHostname: "x3",
				},
			},
			Status: corev1.NodeStatus{
				Allocatable: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
				Conditions: []corev1.NodeCondition{
					{
						Type:   corev1.NodeReady,
						Status: corev1.ConditionTrue,
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "b1-r2-x4",
				Labels: map[string]string{
					tasBlockLabel:        "b1",
					tasRackLabel:         "r2",
					corev1.LabelHostname: "x4",
				},
			},
			Status: corev1.NodeStatus{
				Allocatable: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
				Conditions: []corev1.NodeCondition{
					{
						Type:   corev1.NodeReady,
						Status: corev1.ConditionTrue,
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "b2-r1-x5",
				Labels: map[string]string{
					tasBlockLabel:        "b2",
					tasRackLabel:         "r1",
					corev1.LabelHostname: "x5",
				},
			},
			Status: corev1.NodeStatus{
				Allocatable: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
				Conditions: []corev1.NodeCondition{
					{
						Type:   corev1.NodeReady,
						Status: corev1.ConditionTrue,
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "b2-r2-x6",
				Labels: map[string]string{
					tasBlockLabel:        "b2",
					tasRackLabel:         "r2",
					corev1.LabelHostname: "x6",
				},
			},
			Status: corev1.NodeStatus{
				Allocatable: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("2"),
					corev1.ResourceMemory: resource.MustParse("4Gi"),
				},
				Conditions: []corev1.NodeCondition{
					{
						Type:   corev1.NodeReady,
						Status: corev1.ConditionTrue,
					},
				},
			},
		},
	}
	defaultOneLevel := []string{
		corev1.LabelHostname,
	}
	defaultTwoLevels := []string{
		tasBlockLabel,
		tasRackLabel,
	}
	defaultThreeLevels := []string{
		tasBlockLabel,
		tasRackLabel,
		corev1.LabelHostname,
	}

	//           b1                    b2
	//       /        \             /      \
	//      r1         r2          r1       r2
	//     /  \      /   \        /   \    /   \
	//    x1   x2   x3    x4     x5   x6  x7    x6
	binaryTreesNodes := []corev1.Node{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "b1-r1-x1",
				Labels: map[string]string{
					tasBlockLabel:        "b1",
					tasRackLabel:         "r1",
					corev1.LabelHostname: "x1",
				},
			},
			Status: corev1.NodeStatus{
				Allocatable: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
				Conditions: []corev1.NodeCondition{
					{
						Type:   corev1.NodeReady,
						Status: corev1.ConditionTrue,
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "b1-r1-x2",
				Labels: map[string]string{
					tasBlockLabel:        "b1",
					tasRackLabel:         "r1",
					corev1.LabelHostname: "x2",
				},
			},
			Status: corev1.NodeStatus{
				Allocatable: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
				Conditions: []corev1.NodeCondition{
					{
						Type:   corev1.NodeReady,
						Status: corev1.ConditionTrue,
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "b1-r2-x3",
				Labels: map[string]string{
					tasBlockLabel:        "b1",
					tasRackLabel:         "r2",
					corev1.LabelHostname: "x3",
				},
			},
			Status: corev1.NodeStatus{
				Allocatable: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
				Conditions: []corev1.NodeCondition{
					{
						Type:   corev1.NodeReady,
						Status: corev1.ConditionTrue,
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "b1-r2-x4",
				Labels: map[string]string{
					tasBlockLabel:        "b1",
					tasRackLabel:         "r2",
					corev1.LabelHostname: "x4",
				},
			},
			Status: corev1.NodeStatus{
				Allocatable: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
				Conditions: []corev1.NodeCondition{
					{
						Type:   corev1.NodeReady,
						Status: corev1.ConditionTrue,
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "b2-r1-x5",
				Labels: map[string]string{
					tasBlockLabel:        "b2",
					tasRackLabel:         "r1",
					corev1.LabelHostname: "x5",
				},
			},
			Status: corev1.NodeStatus{
				Allocatable: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
				Conditions: []corev1.NodeCondition{
					{
						Type:   corev1.NodeReady,
						Status: corev1.ConditionTrue,
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "b2-r1-x6",
				Labels: map[string]string{
					tasBlockLabel:        "b2",
					tasRackLabel:         "r1",
					corev1.LabelHostname: "x6",
				},
			},
			Status: corev1.NodeStatus{
				Allocatable: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
				Conditions: []corev1.NodeCondition{
					{
						Type:   corev1.NodeReady,
						Status: corev1.ConditionTrue,
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "b2-r2-x7",
				Labels: map[string]string{
					tasBlockLabel:        "b2",
					tasRackLabel:         "r2",
					corev1.LabelHostname: "x7",
				},
			},
			Status: corev1.NodeStatus{
				Allocatable: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
				Conditions: []corev1.NodeCondition{
					{
						Type:   corev1.NodeReady,
						Status: corev1.ConditionTrue,
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "b2-r2-x8",
				Labels: map[string]string{
					tasBlockLabel:        "b2",
					tasRackLabel:         "r2",
					corev1.LabelHostname: "x8",
				},
			},
			Status: corev1.NodeStatus{
				Allocatable: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
				Conditions: []corev1.NodeCondition{
					{
						Type:   corev1.NodeReady,
						Status: corev1.ConditionTrue,
					},
				},
			},
		},
	}

	cases := map[string]struct {
		request        kueue.PodSetTopologyRequest
		levels         []string
		nodeLabels     map[string]string
		nodes          []corev1.Node
		pods           []corev1.Pod
		requests       resources.Requests
		count          int32
		wantAssignment *kueue.TopologyAssignment
		wantReason     string
	}{
		"minimize the number of used racks before optimizing the number of nodes": {
			// Solution by optimizing the number of racks then nodes: [r3]: [x3,x4,x5,x6]
			// Solution by optimizing the number of nodes: [r1,r2]: [x1,x2]
			//
			//       b1
			//   /   |    \
			//  r1   r2   r3
			//  |     |    |   \   \     \
			// x1:2,x2:2,x3:1,x4:1,x5:1,x6:1
			//
			nodes: []corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "b1-r1-x1",
						Labels: map[string]string{
							tasBlockLabel:        "b1",
							tasRackLabel:         "r1",
							corev1.LabelHostname: "x1",
						},
					},
					Status: corev1.NodeStatus{
						Allocatable: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("2"),
						},
						Conditions: []corev1.NodeCondition{
							{
								Type:   corev1.NodeReady,
								Status: corev1.ConditionTrue,
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "b1-r2-x2",
						Labels: map[string]string{
							tasBlockLabel:        "b1",
							tasRackLabel:         "r2",
							corev1.LabelHostname: "x2",
						},
					},
					Status: corev1.NodeStatus{
						Allocatable: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("2"),
						},
						Conditions: []corev1.NodeCondition{
							{
								Type:   corev1.NodeReady,
								Status: corev1.ConditionTrue,
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "b1-r3-x3",
						Labels: map[string]string{
							tasBlockLabel:        "b1",
							tasRackLabel:         "r3",
							corev1.LabelHostname: "x3",
						},
					},
					Status: corev1.NodeStatus{
						Allocatable: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("1"),
						},
						Conditions: []corev1.NodeCondition{
							{
								Type:   corev1.NodeReady,
								Status: corev1.ConditionTrue,
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "b1-r3-x4",
						Labels: map[string]string{
							tasBlockLabel:        "b1",
							tasRackLabel:         "r3",
							corev1.LabelHostname: "x4",
						},
					},
					Status: corev1.NodeStatus{
						Allocatable: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("1"),
						},
						Conditions: []corev1.NodeCondition{
							{
								Type:   corev1.NodeReady,
								Status: corev1.ConditionTrue,
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "b1-r3-x5",
						Labels: map[string]string{
							tasBlockLabel:        "b1",
							tasRackLabel:         "r3",
							corev1.LabelHostname: "x5",
						},
					},
					Status: corev1.NodeStatus{
						Allocatable: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("1"),
						},
						Conditions: []corev1.NodeCondition{
							{
								Type:   corev1.NodeReady,
								Status: corev1.ConditionTrue,
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "b1-r3-x6",
						Labels: map[string]string{
							tasBlockLabel:        "b1",
							tasRackLabel:         "r3",
							corev1.LabelHostname: "x6",
						},
					},
					Status: corev1.NodeStatus{
						Allocatable: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("1"),
						},
						Conditions: []corev1.NodeCondition{
							{
								Type:   corev1.NodeReady,
								Status: corev1.ConditionTrue,
							},
						},
					},
				},
			},
			request: kueue.PodSetTopologyRequest{
				Required: ptr.To(tasBlockLabel),
			},
			levels: defaultThreeLevels,
			requests: resources.Requests{
				corev1.ResourceCPU: 1000,
			},
			count: 4,
			wantAssignment: &kueue.TopologyAssignment{
				Levels: defaultThreeLevels,
				Domains: []kueue.TopologyDomainAssignment{
					{
						Count: 1,
						Values: []string{
							"b1",
							"r3",
							"x3",
						},
					},
					{
						Count: 1,
						Values: []string{
							"b1",
							"r3",
							"x4",
						},
					},
					{
						Count: 1,
						Values: []string{
							"b1",
							"r3",
							"x5",
						},
					},
					{
						Count: 1,
						Values: []string{
							"b1",
							"r3",
							"x6",
						},
					},
				},
			},
		},
		"block required; 4 pods fit into one host each": {
			nodes: binaryTreesNodes,
			request: kueue.PodSetTopologyRequest{
				Required: ptr.To(tasBlockLabel),
			},
			levels: defaultThreeLevels,
			requests: resources.Requests{
				corev1.ResourceCPU: 1000,
			},
			count: 4,
			wantAssignment: &kueue.TopologyAssignment{
				Levels: defaultThreeLevels,
				Domains: []kueue.TopologyDomainAssignment{
					{
						Count: 1,
						Values: []string{
							"b1",
							"r1",
							"x1",
						},
					},
					{
						Count: 1,
						Values: []string{
							"b1",
							"r1",
							"x2",
						},
					},
					{
						Count: 1,
						Values: []string{
							"b1",
							"r2",
							"x3",
						},
					},
					{
						Count: 1,
						Values: []string{
							"b1",
							"r2",
							"x4",
						},
					},
				},
			},
		},
		"host required; single Pod fits in the host": {
			nodes: defaultNodes,
			request: kueue.PodSetTopologyRequest{
				Required: ptr.To(corev1.LabelHostname),
			},
			levels: defaultThreeLevels,
			requests: resources.Requests{
				corev1.ResourceCPU: 1000,
			},
			count: 1,
			wantAssignment: &kueue.TopologyAssignment{
				Levels: defaultThreeLevels,
				Domains: []kueue.TopologyDomainAssignment{
					{
						Count: 1,
						Values: []string{
							"b2",
							"r2",
							"x6",
						},
					},
				},
			},
		},
		"rack required; single Pod fits in a rack": {
			nodes: defaultNodes,
			request: kueue.PodSetTopologyRequest{
				Required: ptr.To(tasRackLabel),
			},
			levels: defaultTwoLevels,
			requests: resources.Requests{
				corev1.ResourceCPU: 1000,
			},
			count: 1,
			wantAssignment: &kueue.TopologyAssignment{
				Levels: defaultTwoLevels,
				Domains: []kueue.TopologyDomainAssignment{
					{
						Count: 1,
						Values: []string{
							"b1",
							"r2",
						},
					},
				},
			},
		},
		"rack required; multiple Pods fits in a rack": {
			nodes: defaultNodes,
			request: kueue.PodSetTopologyRequest{
				Required: ptr.To(tasRackLabel),
			},
			levels: defaultTwoLevels,
			requests: resources.Requests{
				corev1.ResourceCPU: 1000,
			},
			count: 3,
			wantAssignment: &kueue.TopologyAssignment{
				Levels: defaultTwoLevels,
				Domains: []kueue.TopologyDomainAssignment{
					{
						Count: 3,
						Values: []string{
							"b1",
							"r2",
						},
					},
				},
			},
		},
		"rack required; too many pods to fit in any rack": {
			nodes: defaultNodes,
			request: kueue.PodSetTopologyRequest{
				Required: ptr.To(tasRackLabel),
			},
			levels: defaultTwoLevels,
			requests: resources.Requests{
				corev1.ResourceCPU: 1000,
			},
			count:      4,
			wantReason: `topology "default" allows to fit only 3 out of 4 pod(s)`,
		},
		"block required; single Pod fits in a block": {
			nodes: defaultNodes,
			request: kueue.PodSetTopologyRequest{
				Required: ptr.To(tasBlockLabel),
			},
			levels: defaultTwoLevels,
			requests: resources.Requests{
				corev1.ResourceCPU: 1000,
			},
			count: 1,
			wantAssignment: &kueue.TopologyAssignment{
				Levels: []string{
					tasBlockLabel,
					tasRackLabel,
				},
				Domains: []kueue.TopologyDomainAssignment{
					{
						Count: 1,
						Values: []string{
							"b1",
							"r2",
						},
					},
				},
			},
		},
		"block required; Pods fit in a block spread across two racks": {
			nodes: defaultNodes,
			request: kueue.PodSetTopologyRequest{
				Required: ptr.To(tasBlockLabel),
			},
			levels: defaultTwoLevels,
			requests: resources.Requests{
				corev1.ResourceCPU: 1000,
			},
			count: 4,
			wantAssignment: &kueue.TopologyAssignment{
				Levels: defaultTwoLevels,
				Domains: []kueue.TopologyDomainAssignment{
					{
						Count: 3,
						Values: []string{
							"b1",
							"r2",
						},
					},
					{
						Count: 1,
						Values: []string{
							"b1",
							"r1",
						},
					},
				},
			},
		},
		"block required; single Pod which cannot be split": {
			nodes: defaultNodes,
			request: kueue.PodSetTopologyRequest{
				Required: ptr.To(tasBlockLabel),
			},
			levels: defaultTwoLevels,
			requests: resources.Requests{
				corev1.ResourceCPU: 4000,
			},
			count:      1,
			wantReason: `topology "default" doesn't allow to fit any of 1 pod(s)`,
		},
		"block required; too many Pods to fit requested": {
			nodes: defaultNodes,
			request: kueue.PodSetTopologyRequest{
				Required: ptr.To(tasBlockLabel),
			},
			levels: defaultTwoLevels,
			requests: resources.Requests{
				corev1.ResourceCPU: 1000,
			},
			count:      5,
			wantReason: `topology "default" allows to fit only 4 out of 5 pod(s)`,
		},
		"rack required; single Pod requiring memory": {
			nodes: defaultNodes,
			request: kueue.PodSetTopologyRequest{
				Required: ptr.To(tasRackLabel),
			},
			levels: defaultTwoLevels,
			requests: resources.Requests{
				corev1.ResourceMemory: 1024,
			},
			count: 4,
			wantAssignment: &kueue.TopologyAssignment{
				Levels: defaultTwoLevels,
				Domains: []kueue.TopologyDomainAssignment{
					{
						Count: 4,
						Values: []string{
							"b2",
							"r2",
						},
					},
				},
			},
		},
		"rack preferred; but only block can accommodate the workload": {
			nodes: defaultNodes,
			request: kueue.PodSetTopologyRequest{
				Preferred: ptr.To(tasRackLabel),
			},
			levels: defaultTwoLevels,
			requests: resources.Requests{
				corev1.ResourceCPU: 1000,
			},
			count: 4,
			wantAssignment: &kueue.TopologyAssignment{
				Levels: defaultTwoLevels,
				Domains: []kueue.TopologyDomainAssignment{
					{
						Count: 3,
						Values: []string{
							"b1",
							"r2",
						},
					},
					{
						Count: 1,
						Values: []string{
							"b1",
							"r1",
						},
					},
				},
			},
		},
		"rack preferred; but only multiple blocks can accommodate the workload": {
			nodes: defaultNodes,
			request: kueue.PodSetTopologyRequest{
				Preferred: ptr.To(tasRackLabel),
			},
			levels: defaultTwoLevels,
			requests: resources.Requests{
				corev1.ResourceCPU: 1000,
			},
			count: 6,
			wantAssignment: &kueue.TopologyAssignment{
				Levels: defaultTwoLevels,
				Domains: []kueue.TopologyDomainAssignment{
					{
						Count: 3,
						Values: []string{
							"b1",
							"r2",
						},
					},
					{
						Count: 2,
						Values: []string{
							"b2",
							"r2",
						},
					},
					{
						Count: 1,
						Values: []string{
							"b1",
							"r1",
						},
					},
				},
			},
		},
		"block preferred; but only multiple blocks can accommodate the workload": {
			nodes: defaultNodes,
			request: kueue.PodSetTopologyRequest{
				Preferred: ptr.To(tasBlockLabel),
			},
			levels: defaultTwoLevels,
			requests: resources.Requests{
				corev1.ResourceCPU: 1000,
			},
			count: 6,
			wantAssignment: &kueue.TopologyAssignment{
				Levels: defaultTwoLevels,
				Domains: []kueue.TopologyDomainAssignment{
					{
						Count: 3,
						Values: []string{
							"b1",
							"r2",
						},
					},
					{
						Count: 2,
						Values: []string{
							"b2",
							"r2",
						},
					},
					{
						Count: 1,
						Values: []string{
							"b1",
							"r1",
						},
					},
				},
			},
		},
		"block preferred; but the workload cannot be accommodate in entire topology": {
			nodes: defaultNodes,
			request: kueue.PodSetTopologyRequest{
				Preferred: ptr.To(tasBlockLabel),
			},
			levels: defaultTwoLevels,
			requests: resources.Requests{
				corev1.ResourceCPU: 1000,
			},
			count:      10,
			wantReason: `topology "default" allows to fit only 7 out of 10 pod(s)`,
		},
		"only nodes with matching labels are considered; no matching node": {
			nodes: []corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "b1-r1-x1",
						Labels: map[string]string{
							"zone":               "zone-a",
							corev1.LabelHostname: "x1",
						},
					},
					Status: corev1.NodeStatus{
						Allocatable: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
					},
				},
			},
			request: kueue.PodSetTopologyRequest{
				Required: ptr.To(corev1.LabelHostname),
			},
			nodeLabels: map[string]string{
				"zone": "zone-b",
			},
			levels: defaultOneLevel,
			requests: resources.Requests{
				corev1.ResourceCPU: 1000,
			},
			count:      1,
			wantReason: "no topology domains at level: kubernetes.io/hostname",
		},
		"only nodes with matching labels are considered; matching node is found": {
			nodes: []corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "b1-r1-x1",
						Labels: map[string]string{
							"zone":               "zone-a",
							corev1.LabelHostname: "x1",
						},
					},
					Status: corev1.NodeStatus{
						Allocatable: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
						Conditions: []corev1.NodeCondition{
							{
								Type:   corev1.NodeReady,
								Status: corev1.ConditionTrue,
							},
						},
					},
				},
			},
			request: kueue.PodSetTopologyRequest{
				Required: ptr.To(corev1.LabelHostname),
			},
			nodeLabels: map[string]string{
				"zone": "zone-a",
			},
			levels: defaultOneLevel,
			requests: resources.Requests{
				corev1.ResourceCPU: 1000,
			},
			count: 1,
			wantAssignment: &kueue.TopologyAssignment{
				Levels: defaultOneLevel,
				Domains: []kueue.TopologyDomainAssignment{
					{
						Count: 1,
						Values: []string{
							"x1",
						},
					},
				},
			},
		},
		"only nodes with matching levels are considered; no host label on node": {
			nodes: []corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "b1-r1-x1",
						Labels: map[string]string{
							tasBlockLabel: "b1",
							tasRackLabel:  "r1",
							// the node doesn't have the 'kubernetes.io/hostname' required by topology
						},
					},
					Status: corev1.NodeStatus{
						Allocatable: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
						Conditions: []corev1.NodeCondition{
							{
								Type:   corev1.NodeReady,
								Status: corev1.ConditionTrue,
							},
						},
					},
				},
			},
			request: kueue.PodSetTopologyRequest{
				Required: ptr.To(tasRackLabel),
			},
			levels: defaultThreeLevels,
			requests: resources.Requests{
				corev1.ResourceCPU: 1000,
			},
			count:      1,
			wantReason: "no topology domains at level: cloud.com/topology-rack",
		},
		"don't consider unscheduled Pods when computing capacity": {
			// the Pod is not scheduled (no NodeName set, so is not blocking capacity)
			nodes: []corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "x1",
						Labels: map[string]string{
							corev1.LabelHostname: "x1",
						},
					},
					Status: corev1.NodeStatus{
						Allocatable: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
						Conditions: []corev1.NodeCondition{
							{
								Type:   corev1.NodeReady,
								Status: corev1.ConditionTrue,
							},
						},
					},
				},
			},
			pods: []corev1.Pod{
				*testingpod.MakePod("test-unscheduled", "test-ns").
					Request(corev1.ResourceCPU, "600m").
					Obj(),
			},
			request: kueue.PodSetTopologyRequest{
				Required: ptr.To(corev1.LabelHostname),
			},
			levels: defaultOneLevel,
			requests: resources.Requests{
				corev1.ResourceCPU: 600,
			},
			count: 1,
			wantAssignment: &kueue.TopologyAssignment{
				Levels: defaultOneLevel,
				Domains: []kueue.TopologyDomainAssignment{
					{
						Count: 1,
						Values: []string{
							"x1",
						},
					},
				},
			},
		},
		"don't consider terminal pods when computing the capacity": {
			nodes: []corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "x1",
						Labels: map[string]string{
							corev1.LabelHostname: "x1",
						},
					},
					Status: corev1.NodeStatus{
						Allocatable: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
						Conditions: []corev1.NodeCondition{
							{
								Type:   corev1.NodeReady,
								Status: corev1.ConditionTrue,
							},
						},
					},
				},
			},
			pods: []corev1.Pod{
				*testingpod.MakePod("test-failed", "test-ns").NodeName("x1").
					Request(corev1.ResourceCPU, "600m").
					StatusPhase(corev1.PodFailed).
					Obj(),
				*testingpod.MakePod("test-succeeded", "test-ns").NodeName("x1").
					Request(corev1.ResourceCPU, "600m").
					StatusPhase(corev1.PodSucceeded).
					Obj(),
			},
			request: kueue.PodSetTopologyRequest{
				Required: ptr.To(corev1.LabelHostname),
			},
			levels: defaultOneLevel,
			requests: resources.Requests{
				corev1.ResourceCPU: 600,
			},
			count: 1,
			wantAssignment: &kueue.TopologyAssignment{
				Levels: defaultOneLevel,
				Domains: []kueue.TopologyDomainAssignment{
					{
						Count: 1,
						Values: []string{
							"x1",
						},
					},
				},
			},
		},
		"include usage from pending scheduled non-TAS pods, blocked assignment": {
			// there is not enough free capacity on the only node x1
			nodes: []corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "x1",
						Labels: map[string]string{
							corev1.LabelHostname: "x1",
						},
					},
					Status: corev1.NodeStatus{
						Allocatable: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
						Conditions: []corev1.NodeCondition{
							{
								Type:   corev1.NodeReady,
								Status: corev1.ConditionTrue,
							},
						},
					},
				},
			},
			pods: []corev1.Pod{
				*testingpod.MakePod("test-pending", "test-ns").NodeName("x1").
					StatusPhase(corev1.PodPending).
					Request(corev1.ResourceCPU, "600m").
					Obj(),
			},
			request: kueue.PodSetTopologyRequest{
				Required: ptr.To(corev1.LabelHostname),
			},
			levels: defaultOneLevel,
			requests: resources.Requests{
				corev1.ResourceCPU: 600,
			},
			count:      1,
			wantReason: `topology "default" doesn't allow to fit any of 1 pod(s)`,
		},
		"include usage from running non-TAS pods, blocked assignment": {
			// there is not enough free capacity on the only node x1
			nodes: []corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "x1",
						Labels: map[string]string{
							corev1.LabelHostname: "x1",
						},
					},
					Status: corev1.NodeStatus{
						Allocatable: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
						Conditions: []corev1.NodeCondition{
							{
								Type:   corev1.NodeReady,
								Status: corev1.ConditionTrue,
							},
						},
					},
				},
			},
			pods: []corev1.Pod{
				*testingpod.MakePod("test-running", "test-ns").NodeName("x1").
					StatusPhase(corev1.PodRunning).
					Request(corev1.ResourceCPU, "600m").
					Obj(),
			},
			request: kueue.PodSetTopologyRequest{
				Required: ptr.To(corev1.LabelHostname),
			},
			levels: defaultOneLevel,
			requests: resources.Requests{
				corev1.ResourceCPU: 600,
			},
			count:      1,
			wantReason: `topology "default" doesn't allow to fit any of 1 pod(s)`,
		},
		"include usage from running non-TAS pods, found free capacity on another node": {
			// there is not enough free capacity on the node x1 as the
			// assignments lends on the free x2
			nodes: []corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "x1",
						Labels: map[string]string{
							corev1.LabelHostname: "x1",
						},
					},
					Status: corev1.NodeStatus{
						Allocatable: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
						Conditions: []corev1.NodeCondition{
							{
								Type:   corev1.NodeReady,
								Status: corev1.ConditionTrue,
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "x2",
						Labels: map[string]string{
							corev1.LabelHostname: "x2",
						},
					},
					Status: corev1.NodeStatus{
						Allocatable: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
						Conditions: []corev1.NodeCondition{
							{
								Type:   corev1.NodeReady,
								Status: corev1.ConditionTrue,
							},
						},
					},
				},
			},
			pods: []corev1.Pod{
				*testingpod.MakePod("test-pod", "test-ns").NodeName("x1").
					Request(corev1.ResourceCPU, "600m").
					Obj(),
			},
			request: kueue.PodSetTopologyRequest{
				Required: ptr.To(corev1.LabelHostname),
			},
			levels: defaultOneLevel,
			requests: resources.Requests{
				corev1.ResourceCPU: 600,
			},
			count: 1,
			wantAssignment: &kueue.TopologyAssignment{
				Levels: defaultOneLevel,
				Domains: []kueue.TopologyDomainAssignment{
					{
						Count: 1,
						Values: []string{
							"x2",
						},
					},
				},
			},
		},
		"no assignment as node is not ready": {
			nodes: []corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "b1-r1-x1",
						Labels: map[string]string{
							"zone":               "zone-a",
							corev1.LabelHostname: "x1",
						},
					},
					Status: corev1.NodeStatus{
						Allocatable: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
						Conditions: []corev1.NodeCondition{
							{
								Type:   corev1.NodeReady,
								Status: corev1.ConditionFalse,
							},
							{
								Type:   corev1.NodeNetworkUnavailable,
								Status: corev1.ConditionTrue,
							},
						},
					},
				},
			},
			request: kueue.PodSetTopologyRequest{
				Required: ptr.To(corev1.LabelHostname),
			},
			nodeLabels: map[string]string{
				"zone": "zone-a",
			},
			levels: defaultOneLevel,
			requests: resources.Requests{
				corev1.ResourceCPU: 1000,
			},
			count:      1,
			wantReason: "no topology domains at level: kubernetes.io/hostname",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()

			initialObjects := make([]client.Object, 0)
			for i := range tc.nodes {
				initialObjects = append(initialObjects, &tc.nodes[i])
			}
			for i := range tc.pods {
				initialObjects = append(initialObjects, &tc.pods[i])
			}
			clientBuilder := utiltesting.NewClientBuilder()
			clientBuilder.WithObjects(initialObjects...)
			_ = tasindexer.SetupIndexes(ctx, utiltesting.AsIndexer(clientBuilder))
			client := clientBuilder.Build()

			tasCache := NewTASCache(client)
			tasFlavorCache := tasCache.NewTASFlavorCache("default", tc.levels, tc.nodeLabels)

			snapshot, err := tasFlavorCache.snapshot(ctx)
			if err != nil {
				t.Fatalf("failed to build the snapshot: %v", err)
			}
			gotAssignment, reason := snapshot.FindTopologyAssignment(&tc.request, tc.requests, tc.count)
			if diff := cmp.Diff(tc.wantAssignment, gotAssignment); diff != "" {
				t.Errorf("unexpected topology assignment (-want,+got): %s", diff)
			}
			if diff := cmp.Diff(tc.wantReason, reason); diff != "" {
				t.Errorf("unexpected error (-want,+got): %s", diff)
			}
		})
	}
}
