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

package v1

import (
	fmt "fmt"
	strings "strings"
	sync "sync"
	"time"

	v1 "k8s.io/api/core/v1"
	errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	diff "k8s.io/apimachinery/pkg/util/diff"
	watch "k8s.io/apimachinery/pkg/watch"
	scheme "k8s.io/client-go/kubernetes/scheme"
	rest "k8s.io/client-go/rest"
	klog "k8s.io/klog"
)

// NamespacesGetter has a method to return a NamespaceInterface.
// A group's client should implement this interface.
type NamespacesGetter interface {
	Namespaces() NamespaceInterface
	NamespacesWithMultiTenancy(tenant string) NamespaceInterface
}

// NamespaceInterface has methods to work with Namespace resources.
type NamespaceInterface interface {
	Create(*v1.Namespace) (*v1.Namespace, error)
	Update(*v1.Namespace) (*v1.Namespace, error)
	UpdateStatus(*v1.Namespace) (*v1.Namespace, error)
	Delete(name string, options *metav1.DeleteOptions) error
	Get(name string, options metav1.GetOptions) (*v1.Namespace, error)
	List(opts metav1.ListOptions) (*v1.NamespaceList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.Namespace, err error)
	NamespaceExpansion
}

// namespaces implements NamespaceInterface
type namespaces struct {
	client  rest.Interface
	clients []rest.Interface
	te      string
}

// newNamespaces returns a Namespaces
func newNamespaces(c *CoreV1Client) *namespaces {
	return newNamespacesWithMultiTenancy(c, "system")
}

func newNamespacesWithMultiTenancy(c *CoreV1Client, tenant string) *namespaces {
	return &namespaces{
		client:  c.RESTClient(),
		clients: c.RESTClients(),
		te:      tenant,
	}
}

// Get takes name of the namespace, and returns the corresponding namespace object, and an error if there is any.
func (c *namespaces) Get(name string, options metav1.GetOptions) (result *v1.Namespace, err error) {
	result = &v1.Namespace{}
	err = c.client.Get().
		Tenant(c.te).
		Resource("namespaces").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)

	return
}

// List takes label and field selectors, and returns the list of Namespaces that match those selectors.
func (c *namespaces) List(opts metav1.ListOptions) (result *v1.NamespaceList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1.NamespaceList{}

	wgLen := 1
	// When resource version is not empty, it reads from api server local cache
	// Need to check all api server partitions
	if opts.ResourceVersion != "" && len(c.clients) > 1 {
		wgLen = len(c.clients)
	}

	if wgLen > 1 {
		var listLock sync.Mutex

		var wg sync.WaitGroup
		wg.Add(wgLen)
		results := make(map[int]*v1.NamespaceList)
		errs := make(map[int]error)
		for i, client := range c.clients {
			go func(c *namespaces, ci rest.Interface, opts metav1.ListOptions, lock *sync.Mutex, pos int, resultMap map[int]*v1.NamespaceList, errMap map[int]error) {
				r := &v1.NamespaceList{}
				err := ci.Get().
					Tenant(c.te).
					Resource("namespaces").
					VersionedParams(&opts, scheme.ParameterCodec).
					Timeout(timeout).
					Do().
					Into(r)

				lock.Lock()
				resultMap[pos] = r
				errMap[pos] = err
				lock.Unlock()
				wg.Done()
			}(c, client, opts, &listLock, i, results, errs)
		}
		wg.Wait()

		// consolidate list result
		itemsMap := make(map[string]v1.Namespace)
		for j := 0; j < wgLen; j++ {
			currentErr, isOK := errs[j]
			if isOK && currentErr != nil {
				if !(errors.IsForbidden(currentErr) && strings.Contains(currentErr.Error(), "no relationship found between node")) {
					err = currentErr
					return
				} else {
					continue
				}
			}

			currentResult, _ := results[j]
			if result.ResourceVersion == "" {
				result.TypeMeta = currentResult.TypeMeta
				result.ListMeta = currentResult.ListMeta
			} else {
				isNewer, errCompare := diff.RevisionStrIsNewer(currentResult.ResourceVersion, result.ResourceVersion)
				if errCompare != nil {
					err = errors.NewInternalError(fmt.Errorf("Invalid resource version [%v]", errCompare))
					return
				} else if isNewer {
					// Since the lists are from different api servers with different partition. When used in list and watch,
					// we cannot watch from the biggest resource version. Leave it to watch for adjustment.
					result.ResourceVersion = currentResult.ResourceVersion
				}
			}
			for _, item := range currentResult.Items {
				if _, exist := itemsMap[item.ResourceVersion]; !exist {
					itemsMap[item.ResourceVersion] = item
				}
			}
		}

		for _, item := range itemsMap {
			result.Items = append(result.Items, item)
		}
		return
	}

	// The following is used for single api server partition and/or resourceVersion is empty
	// When resourceVersion is empty, objects are read from ETCD directly and will get full
	// list of data if no permission issue. The list needs to done sequential to avoid increasing
	// system load.
	err = c.client.Get().
		Tenant(c.te).
		Resource("namespaces").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do().
		Into(result)
	if err == nil {
		return
	}

	if !(errors.IsForbidden(err) && strings.Contains(err.Error(), "no relationship found between node")) {
		return
	}

	// Found api server that works with this list, keep the client
	for _, client := range c.clients {
		if client == c.client {
			continue
		}

		err = client.Get().
			Tenant(c.te).
			Resource("namespaces").
			VersionedParams(&opts, scheme.ParameterCodec).
			Timeout(timeout).
			Do().
			Into(result)

		if err == nil {
			c.client = client
			return
		}

		if err != nil && errors.IsForbidden(err) &&
			strings.Contains(err.Error(), "no relationship found between node") {
			klog.V(6).Infof("Skip error %v in list", err)
			continue
		}
	}

	return
}

// Watch returns a watch.Interface that watches the requested namespaces.
func (c *namespaces) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	aggWatch := watch.NewAggregatedWatcher()
	for _, client := range c.clients {
		watcher, err := client.Get().
			Tenant(c.te).
			Resource("namespaces").
			VersionedParams(&opts, scheme.ParameterCodec).
			Timeout(timeout).
			Watch()
		if err != nil && opts.AllowPartialWatch && errors.IsForbidden(err) {
			// watch error was not returned properly in error message. Skip when partial watch is allowed
			klog.V(6).Infof("Watch error for partial watch %v. options [%+v]", err, opts)
			continue
		}
		aggWatch.AddWatchInterface(watcher, err)
	}
	return aggWatch, aggWatch.GetErrors()
}

// Create takes the representation of a namespace and creates it.  Returns the server's representation of the namespace, and an error, if there is any.
func (c *namespaces) Create(namespace *v1.Namespace) (result *v1.Namespace, err error) {
	result = &v1.Namespace{}

	objectTenant := namespace.ObjectMeta.Tenant
	if objectTenant == "" {
		objectTenant = c.te
	}

	err = c.client.Post().
		Tenant(objectTenant).
		Resource("namespaces").
		Body(namespace).
		Do().
		Into(result)

	return
}

// Update takes the representation of a namespace and updates it. Returns the server's representation of the namespace, and an error, if there is any.
func (c *namespaces) Update(namespace *v1.Namespace) (result *v1.Namespace, err error) {
	result = &v1.Namespace{}

	objectTenant := namespace.ObjectMeta.Tenant
	if objectTenant == "" {
		objectTenant = c.te
	}

	err = c.client.Put().
		Tenant(objectTenant).
		Resource("namespaces").
		Name(namespace.Name).
		Body(namespace).
		Do().
		Into(result)

	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().

func (c *namespaces) UpdateStatus(namespace *v1.Namespace) (result *v1.Namespace, err error) {
	result = &v1.Namespace{}

	objectTenant := namespace.ObjectMeta.Tenant
	if objectTenant == "" {
		objectTenant = c.te
	}

	err = c.client.Put().
		Tenant(objectTenant).
		Resource("namespaces").
		Name(namespace.Name).
		SubResource("status").
		Body(namespace).
		Do().
		Into(result)

	return
}

// Delete takes name of the namespace and deletes it. Returns an error if one occurs.
func (c *namespaces) Delete(name string, options *metav1.DeleteOptions) error {
	return c.client.Delete().
		Tenant(c.te).
		Resource("namespaces").
		Name(name).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched namespace.
func (c *namespaces) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.Namespace, err error) {
	result = &v1.Namespace{}
	err = c.client.Patch(pt).
		Tenant(c.te).
		Resource("namespaces").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)

	return
}
