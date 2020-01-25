/*
Copyright The Kubernetes Authors.
Copyright 2020 Authors of Alkaid - file modified.

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

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
	v1alpha1 "k8s.io/sample-controller/pkg/generated/clientset/versioned/typed/samplecontroller/v1alpha1"
)

type FakeSamplecontrollerV1alpha1 struct {
	*testing.Fake
}

func (c *FakeSamplecontrollerV1alpha1) Foos(namespace string) v1alpha1.FooInterface {
	return &FakeFoos{c, namespace, "default"}
}

func (c *FakeSamplecontrollerV1alpha1) FoosWithMultiTenancy(namespace string, tenant string) v1alpha1.FooInterface {
	return &FakeFoos{c, namespace, tenant}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeSamplecontrollerV1alpha1) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}

// RESTClients returns all RESTClient that are used to communicate
// with all API servers by this client implementation.
func (c *FakeSamplecontrollerV1alpha1) RESTClients() []rest.Interface {
	var ret *rest.RESTClient
	return []rest.Interface{ret}
}
