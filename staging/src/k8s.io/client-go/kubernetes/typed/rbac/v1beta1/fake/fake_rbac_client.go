/*
Copyright The Kubernetes Authors.
Copyright 2020 Authors of Arktos - file modified.

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
	v1beta1 "k8s.io/client-go/kubernetes/typed/rbac/v1beta1"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeRbacV1beta1 struct {
	*testing.Fake
}

func (c *FakeRbacV1beta1) ClusterRoles() v1beta1.ClusterRoleInterface {
	return &FakeClusterRoles{c, "system"}
}

func (c *FakeRbacV1beta1) ClusterRolesWithMultiTenancy(tenant string) v1beta1.ClusterRoleInterface {
	return &FakeClusterRoles{c, tenant}
}

func (c *FakeRbacV1beta1) ClusterRoleBindings() v1beta1.ClusterRoleBindingInterface {
	return &FakeClusterRoleBindings{c, "system"}
}

func (c *FakeRbacV1beta1) ClusterRoleBindingsWithMultiTenancy(tenant string) v1beta1.ClusterRoleBindingInterface {
	return &FakeClusterRoleBindings{c, tenant}
}

func (c *FakeRbacV1beta1) Roles(namespace string) v1beta1.RoleInterface {
	return &FakeRoles{c, namespace, "system"}
}

func (c *FakeRbacV1beta1) RolesWithMultiTenancy(namespace string, tenant string) v1beta1.RoleInterface {
	return &FakeRoles{c, namespace, tenant}
}

func (c *FakeRbacV1beta1) RoleBindings(namespace string) v1beta1.RoleBindingInterface {
	return &FakeRoleBindings{c, namespace, "system"}
}

func (c *FakeRbacV1beta1) RoleBindingsWithMultiTenancy(namespace string, tenant string) v1beta1.RoleBindingInterface {
	return &FakeRoleBindings{c, namespace, tenant}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeRbacV1beta1) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}

// RESTClients returns all RESTClient that are used to communicate
// with all API servers by this client implementation.
func (c *FakeRbacV1beta1) RESTClients() []rest.Interface {
	var ret *rest.RESTClient
	return []rest.Interface{ret}
}
