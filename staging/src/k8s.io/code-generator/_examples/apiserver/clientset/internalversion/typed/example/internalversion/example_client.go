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

package internalversion

import (
	"time"

	rand "k8s.io/apimachinery/pkg/util/rand"
	rest "k8s.io/client-go/rest"
	"k8s.io/code-generator/_examples/apiserver/clientset/internalversion/scheme"
)

type ExampleInterface interface {
	RESTClient() rest.Interface
	RESTClients() []rest.Interface
	TestTypesGetter
}

// ExampleClient is used to interact with features provided by the example.apiserver.code-generator.k8s.io group.
type ExampleClient struct {
	restClients []rest.Interface
}

func (c *ExampleClient) TestTypes(namespace string) TestTypeInterface {
	return newTestTypesWithMultiTenancy(c, namespace, "default")
}

func (c *ExampleClient) TestTypesWithMultiTenancy(namespace string, tenant string) TestTypeInterface {
	return newTestTypesWithMultiTenancy(c, namespace, tenant)
}

// NewForConfig creates a new ExampleClient for the given config.
func NewForConfig(c *rest.Config) (*ExampleClient, error) {
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}

	clients := []rest.Interface{client}
	return &ExampleClient{clients}, nil
}

// NewForConfigOrDie creates a new ExampleClient for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(c *rest.Config) *ExampleClient {
	client, err := NewForConfig(c)
	if err != nil {
		panic(err)
	}
	return client
}

// New creates a new ExampleClient for the given RESTClient.
func New(c rest.Interface) *ExampleClient {
	clients := []rest.Interface{c}
	return &ExampleClient{clients}
}

func setConfigDefaults(config *rest.Config) error {
	config.APIPath = "/apis"
	if config.UserAgent == "" {
		config.UserAgent = rest.DefaultKubernetesUserAgent()
	}
	if config.GroupVersion == nil || config.GroupVersion.Group != scheme.Scheme.PrioritizedVersionsForGroup("example.apiserver.code-generator.k8s.io")[0].Group {
		gv := scheme.Scheme.PrioritizedVersionsForGroup("example.apiserver.code-generator.k8s.io")[0]
		config.GroupVersion = &gv
	}
	config.NegotiatedSerializer = scheme.Codecs

	if config.QPS == 0 {
		config.QPS = 5
	}
	if config.Burst == 0 {
		config.Burst = 10
	}

	return nil
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *ExampleClient) RESTClient() rest.Interface {
	if c == nil {
		return nil
	}

	max := len(c.restClients)
	if max == 0 {
		return nil
	}
	if max == 1 {
		return c.restClients[0]
	}

	rand.Seed(time.Now().UnixNano())
	ran := rand.IntnRange(0, max-1)
	return c.restClients[ran]
}

// RESTClients returns all RESTClient that are used to communicate
// with all API servers by this client implementation.
func (c *ExampleClient) RESTClients() []rest.Interface {
	if c == nil {
		return nil
	}

	return c.restClients
}
