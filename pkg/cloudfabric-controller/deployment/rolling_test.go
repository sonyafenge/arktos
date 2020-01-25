/*
Copyright 2020 Authors of Alkaid.

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

package deployment

import (
	"github.com/grafov/bcast"
	"k8s.io/kubernetes/pkg/cloudfabric-controller/controllerframework"
	"testing"

	apps "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"
	core "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"
)

func TestDeploymentController_reconcileNewReplicaSet(t *testing.T) {
	testDeploymentController_reconcileNewReplicaSet(t, metav1.TenantDefault)
}

func TestDeploymentController_reconcileNewReplicaSetWithMultiTenancy(t *testing.T) {
	testDeploymentController_reconcileNewReplicaSet(t, "test-te")
}

func testDeploymentController_reconcileNewReplicaSet(t *testing.T, tenant string) {
	tests := []struct {
		deploymentReplicas  int
		maxSurge            intstr.IntOrString
		oldReplicas         int
		newReplicas         int
		scaleExpected       bool
		expectedNewReplicas int
	}{
		{
			// Should not scale up.
			deploymentReplicas: 10,
			maxSurge:           intstr.FromInt(0),
			oldReplicas:        10,
			newReplicas:        0,
			scaleExpected:      false,
		},
		{
			deploymentReplicas:  10,
			maxSurge:            intstr.FromInt(2),
			oldReplicas:         10,
			newReplicas:         0,
			scaleExpected:       true,
			expectedNewReplicas: 2,
		},
		{
			deploymentReplicas:  10,
			maxSurge:            intstr.FromInt(2),
			oldReplicas:         5,
			newReplicas:         0,
			scaleExpected:       true,
			expectedNewReplicas: 7,
		},
		{
			deploymentReplicas: 10,
			maxSurge:           intstr.FromInt(2),
			oldReplicas:        10,
			newReplicas:        2,
			scaleExpected:      false,
		},
		{
			// Should scale down.
			deploymentReplicas:  10,
			maxSurge:            intstr.FromInt(2),
			oldReplicas:         2,
			newReplicas:         11,
			scaleExpected:       true,
			expectedNewReplicas: 10,
		},
	}

	oldHandler := controllerframework.CreateControllerInstanceHandler
	controllerframework.CreateControllerInstanceHandler = controllerframework.MockCreateControllerInstance
	defer func() {
		controllerframework.CreateControllerInstanceHandler = oldHandler
	}()

	stopCh := make(chan struct{})
	defer close(stopCh)
	cimUpdateCh, informersResetChGrp := controllerframework.MockCreateControllerInstanceAndResetChs(stopCh)

	for i := range tests {
		test := tests[i]
		t.Logf("executing scenario %d", i)
		newRS := rs("foo-v2", test.newReplicas, nil, noTimestamp, tenant)
		oldRS := rs("foo-v2", test.oldReplicas, nil, noTimestamp, tenant)
		allRSs := []*apps.ReplicaSet{newRS, oldRS}
		maxUnavailable := intstr.FromInt(0)
		deployment := newDeployment("foo", test.deploymentReplicas, nil, &test.maxSurge, &maxUnavailable, map[string]string{"foo": "bar"}, tenant)
		fake := fake.Clientset{}

		resetCh := bcast.NewGroup()
		defer resetCh.Close()
		go resetCh.Broadcast(0)

		baseController, err := controllerframework.NewControllerBase("Deployment", &fake, cimUpdateCh, informersResetChGrp)
		controller := &DeploymentController{
			ControllerBase: baseController,
			eventRecorder:  &record.FakeRecorder{},
		}
		scaled, err := controller.reconcileNewReplicaSet(allRSs, newRS, deployment)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			continue
		}
		if !test.scaleExpected {
			if scaled || len(fake.Actions()) > 0 {
				t.Errorf("unexpected scaling: %v", fake.Actions())
			}
			continue
		}
		if test.scaleExpected && !scaled {
			t.Errorf("expected scaling to occur")
			continue
		}
		if len(fake.Actions()) != 1 {
			t.Errorf("expected 1 action during scale, got: %v", fake.Actions())
			continue
		}
		updated := fake.Actions()[0].(core.UpdateAction).GetObject().(*apps.ReplicaSet)
		if e, a := test.expectedNewReplicas, int(*(updated.Spec.Replicas)); e != a {
			t.Errorf("expected update to %d replicas, got %d", e, a)
		}
	}
}

func TestDeploymentController_reconcileOldReplicaSets(t *testing.T) {
	testDeploymentController_reconcileOldReplicaSets(t, metav1.TenantDefault)
}

func TestDeploymentController_reconcileOldReplicaSetsWithMultiTenancy(t *testing.T) {
	testDeploymentController_reconcileOldReplicaSets(t, "test-te")
}

func testDeploymentController_reconcileOldReplicaSets(t *testing.T, tenant string) {
	tests := []struct {
		deploymentReplicas  int
		maxUnavailable      intstr.IntOrString
		oldReplicas         int
		newReplicas         int
		readyPodsFromOldRS  int
		readyPodsFromNewRS  int
		scaleExpected       bool
		expectedOldReplicas int
	}{
		{
			deploymentReplicas:  10,
			maxUnavailable:      intstr.FromInt(0),
			oldReplicas:         10,
			newReplicas:         0,
			readyPodsFromOldRS:  10,
			readyPodsFromNewRS:  0,
			scaleExpected:       true,
			expectedOldReplicas: 9,
		},
		{
			deploymentReplicas:  10,
			maxUnavailable:      intstr.FromInt(2),
			oldReplicas:         10,
			newReplicas:         0,
			readyPodsFromOldRS:  10,
			readyPodsFromNewRS:  0,
			scaleExpected:       true,
			expectedOldReplicas: 8,
		},
		{ // expect unhealthy replicas from old replica sets been cleaned up
			deploymentReplicas:  10,
			maxUnavailable:      intstr.FromInt(2),
			oldReplicas:         10,
			newReplicas:         0,
			readyPodsFromOldRS:  8,
			readyPodsFromNewRS:  0,
			scaleExpected:       true,
			expectedOldReplicas: 8,
		},
		{ // expect 1 unhealthy replica from old replica sets been cleaned up, and 1 ready pod been scaled down
			deploymentReplicas:  10,
			maxUnavailable:      intstr.FromInt(2),
			oldReplicas:         10,
			newReplicas:         0,
			readyPodsFromOldRS:  9,
			readyPodsFromNewRS:  0,
			scaleExpected:       true,
			expectedOldReplicas: 8,
		},
		{ // the unavailable pods from the newRS would not make us scale down old RSs in a further step
			deploymentReplicas: 10,
			maxUnavailable:     intstr.FromInt(2),
			oldReplicas:        8,
			newReplicas:        2,
			readyPodsFromOldRS: 8,
			readyPodsFromNewRS: 0,
			scaleExpected:      false,
		},
	}

	oldHandler := controllerframework.CreateControllerInstanceHandler
	controllerframework.CreateControllerInstanceHandler = controllerframework.MockCreateControllerInstance
	defer func() {
		controllerframework.CreateControllerInstanceHandler = oldHandler
	}()

	stopCh := make(chan struct{})
	defer close(stopCh)
	cimUpdateCh, informersResetChGrp := controllerframework.MockCreateControllerInstanceAndResetChs(stopCh)

	for i := range tests {
		test := tests[i]
		t.Logf("executing scenario %d", i)

		newSelector := map[string]string{"foo": "new"}
		oldSelector := map[string]string{"foo": "old"}
		newRS := rs("foo-new", test.newReplicas, newSelector, noTimestamp, tenant)
		newRS.Status.AvailableReplicas = int32(test.readyPodsFromNewRS)
		oldRS := rs("foo-old", test.oldReplicas, oldSelector, noTimestamp, tenant)
		oldRS.Status.AvailableReplicas = int32(test.readyPodsFromOldRS)
		oldRSs := []*apps.ReplicaSet{oldRS}
		allRSs := []*apps.ReplicaSet{oldRS, newRS}
		maxSurge := intstr.FromInt(0)
		deployment := newDeployment("foo", test.deploymentReplicas, nil, &maxSurge, &test.maxUnavailable, newSelector, tenant)
		fakeClientset := fake.Clientset{}

		resetCh := bcast.NewGroup()
		defer resetCh.Close()
		go resetCh.Broadcast(0)

		baseController, err := controllerframework.NewControllerBase("Deployment", &fakeClientset, cimUpdateCh, informersResetChGrp)

		controller := &DeploymentController{
			ControllerBase: baseController,
			eventRecorder:  &record.FakeRecorder{},
		}

		scaled, err := controller.reconcileOldReplicaSets(allRSs, oldRSs, newRS, deployment)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			continue
		}
		if !test.scaleExpected && scaled {
			t.Errorf("unexpected scaling: %v", fakeClientset.Actions())
		}
		if test.scaleExpected && !scaled {
			t.Errorf("expected scaling to occur")
			continue
		}
		continue
	}
}

func TestDeploymentController_cleanupUnhealthyReplicas(t *testing.T) {
	testDeploymentController_cleanupUnhealthyReplicas(t, metav1.TenantDefault)
}

func TestDeploymentController_cleanupUnhealthyReplicasWithMultiTenancy(t *testing.T) {
	testDeploymentController_cleanupUnhealthyReplicas(t, "test-te")
}

func testDeploymentController_cleanupUnhealthyReplicas(t *testing.T, tenant string) {
	tests := []struct {
		oldReplicas          int
		readyPods            int
		unHealthyPods        int
		maxCleanupCount      int
		cleanupCountExpected int
	}{
		{
			oldReplicas:          10,
			readyPods:            8,
			unHealthyPods:        2,
			maxCleanupCount:      1,
			cleanupCountExpected: 1,
		},
		{
			oldReplicas:          10,
			readyPods:            8,
			unHealthyPods:        2,
			maxCleanupCount:      3,
			cleanupCountExpected: 2,
		},
		{
			oldReplicas:          10,
			readyPods:            8,
			unHealthyPods:        2,
			maxCleanupCount:      0,
			cleanupCountExpected: 0,
		},
		{
			oldReplicas:          10,
			readyPods:            10,
			unHealthyPods:        0,
			maxCleanupCount:      3,
			cleanupCountExpected: 0,
		},
	}

	oldHandler := controllerframework.CreateControllerInstanceHandler
	controllerframework.CreateControllerInstanceHandler = controllerframework.MockCreateControllerInstance
	defer func() {
		controllerframework.CreateControllerInstanceHandler = oldHandler
	}()

	stopCh := make(chan struct{})
	defer close(stopCh)
	cimUpdateCh, informersResetChGrp := controllerframework.MockCreateControllerInstanceAndResetChs(stopCh)

	for i, test := range tests {
		t.Logf("executing scenario %d", i)
		oldRS := rs("foo-v2", test.oldReplicas, nil, noTimestamp, tenant)
		oldRS.Status.AvailableReplicas = int32(test.readyPods)
		oldRSs := []*apps.ReplicaSet{oldRS}
		maxSurge := intstr.FromInt(2)
		maxUnavailable := intstr.FromInt(2)
		deployment := newDeployment("foo", 10, nil, &maxSurge, &maxUnavailable, nil, tenant)
		fakeClientset := fake.Clientset{}

		resetCh := bcast.NewGroup()
		defer resetCh.Close()
		go resetCh.Broadcast(0)

		baseController, err := controllerframework.NewControllerBase("Deployment", &fakeClientset, cimUpdateCh, informersResetChGrp)

		controller := &DeploymentController{
			ControllerBase: baseController,
			eventRecorder:  &record.FakeRecorder{},
		}
		_, cleanupCount, err := controller.cleanupUnhealthyReplicas(oldRSs, deployment, int32(test.maxCleanupCount))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			continue
		}
		if int(cleanupCount) != test.cleanupCountExpected {
			t.Errorf("expected %v unhealthy replicas been cleaned up, got %v", test.cleanupCountExpected, cleanupCount)
			continue
		}
	}
}

func TestDeploymentController_scaleDownOldReplicaSetsForRollingUpdate(t *testing.T) {
	testDeploymentController_scaleDownOldReplicaSetsForRollingUpdate(t, metav1.TenantDefault)
}

func TestDeploymentController_scaleDownOldReplicaSetsForRollingUpdateWithMultiTenancy(t *testing.T) {
	testDeploymentController_scaleDownOldReplicaSetsForRollingUpdate(t, "test-te")
}

func testDeploymentController_scaleDownOldReplicaSetsForRollingUpdate(t *testing.T, tenant string) {
	tests := []struct {
		deploymentReplicas  int
		maxUnavailable      intstr.IntOrString
		readyPods           int
		oldReplicas         int
		scaleExpected       bool
		expectedOldReplicas int
	}{
		{
			deploymentReplicas:  10,
			maxUnavailable:      intstr.FromInt(0),
			readyPods:           10,
			oldReplicas:         10,
			scaleExpected:       true,
			expectedOldReplicas: 9,
		},
		{
			deploymentReplicas:  10,
			maxUnavailable:      intstr.FromInt(2),
			readyPods:           10,
			oldReplicas:         10,
			scaleExpected:       true,
			expectedOldReplicas: 8,
		},
		{
			deploymentReplicas: 10,
			maxUnavailable:     intstr.FromInt(2),
			readyPods:          8,
			oldReplicas:        10,
			scaleExpected:      false,
		},
		{
			deploymentReplicas: 10,
			maxUnavailable:     intstr.FromInt(2),
			readyPods:          10,
			oldReplicas:        0,
			scaleExpected:      false,
		},
		{
			deploymentReplicas: 10,
			maxUnavailable:     intstr.FromInt(2),
			readyPods:          1,
			oldReplicas:        10,
			scaleExpected:      false,
		},
	}

	oldHandler := controllerframework.CreateControllerInstanceHandler
	controllerframework.CreateControllerInstanceHandler = controllerframework.MockCreateControllerInstance
	defer func() {
		controllerframework.CreateControllerInstanceHandler = oldHandler
	}()

	stopCh := make(chan struct{})
	defer close(stopCh)
	cimUpdateCh, informersResetChGrp := controllerframework.MockCreateControllerInstanceAndResetChs(stopCh)

	for i := range tests {
		test := tests[i]
		t.Logf("executing scenario %d", i)
		oldRS := rs("foo-v2", test.oldReplicas, nil, noTimestamp, tenant)
		oldRS.Status.AvailableReplicas = int32(test.readyPods)
		allRSs := []*apps.ReplicaSet{oldRS}
		oldRSs := []*apps.ReplicaSet{oldRS}
		maxSurge := intstr.FromInt(0)
		deployment := newDeployment("foo", test.deploymentReplicas, nil, &maxSurge, &test.maxUnavailable, map[string]string{"foo": "bar"}, tenant)
		fakeClientset := fake.Clientset{}

		resetCh := bcast.NewGroup()
		defer resetCh.Close()
		go resetCh.Broadcast(0)

		baseController, err := controllerframework.NewControllerBase("Deployment", &fakeClientset, cimUpdateCh, informersResetChGrp)

		controller := &DeploymentController{
			ControllerBase: baseController,
			eventRecorder:  &record.FakeRecorder{},
		}
		scaled, err := controller.scaleDownOldReplicaSetsForRollingUpdate(allRSs, oldRSs, deployment)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			continue
		}
		if !test.scaleExpected {
			if scaled != 0 {
				t.Errorf("unexpected scaling: %v", fakeClientset.Actions())
			}
			continue
		}
		if test.scaleExpected && scaled == 0 {
			t.Errorf("expected scaling to occur; actions: %v", fakeClientset.Actions())
			continue
		}
		// There are both list and update actions logged, so extract the update
		// action for verification.
		var updateAction core.UpdateAction
		for _, action := range fakeClientset.Actions() {
			switch a := action.(type) {
			case core.UpdateAction:
				if updateAction != nil {
					t.Errorf("expected only 1 update action; had %v and found %v", updateAction, a)
				} else {
					updateAction = a
				}
			}
		}
		if updateAction == nil {
			t.Errorf("expected an update action")
			continue
		}
		updated := updateAction.GetObject().(*apps.ReplicaSet)
		if e, a := test.expectedOldReplicas, int(*(updated.Spec.Replicas)); e != a {
			t.Errorf("expected update to %d replicas, got %d", e, a)
		}
	}
}
