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

// File modified by cherrypick from kubernetes on 02/22/2021
package generic

import (
	"context"
	"fmt"
	"io"

	admissionv1 "k8s.io/api/admission/v1"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	"k8s.io/api/admissionregistration/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	genericadmissioninit "k8s.io/apiserver/pkg/admission/initializer"
	"k8s.io/apiserver/pkg/admission/plugin/webhook"
	"k8s.io/apiserver/pkg/admission/plugin/webhook/config"
	"k8s.io/apiserver/pkg/admission/plugin/webhook/namespace"
	"k8s.io/apiserver/pkg/admission/plugin/webhook/object"
	"k8s.io/apiserver/pkg/admission/plugin/webhook/rules"
	webhookutil "k8s.io/apiserver/pkg/util/webhook"
	"k8s.io/client-go/informers"
	clientset "k8s.io/client-go/kubernetes"
)

// Webhook is an abstract admission plugin with all the infrastructure to define Admit or Validate on-top.
type Webhook struct {
	*admission.Handler

	sourceFactory sourceFactory

	hookSource       Source
	clientManager    *webhookutil.ClientManager
	namespaceMatcher *namespace.Matcher
	objectMatcher    *object.Matcher
	dispatcher       Dispatcher
}

var (
	_ genericadmissioninit.WantsExternalKubeClientSet = &Webhook{}
	_ admission.Interface                             = &Webhook{}
)

type sourceFactory func(f informers.SharedInformerFactory) Source
type dispatcherFactory func(cm *webhookutil.ClientManager) Dispatcher

// NewWebhook creates a new generic admission webhook.
func NewWebhook(handler *admission.Handler, configFile io.Reader, sourceFactory sourceFactory, dispatcherFactory dispatcherFactory) (*Webhook, error) {
	kubeconfigFile, err := config.LoadConfig(configFile)
	if err != nil {
		return nil, err
	}

	cm, err := webhookutil.NewClientManager(
		[]schema.GroupVersion{
			admissionv1beta1.SchemeGroupVersion,
			admissionv1.SchemeGroupVersion,
		},
		admissionv1beta1.AddToScheme,
		admissionv1.AddToScheme,
	)
	if err != nil {
		return nil, err
	}
	authInfoResolver, err := webhookutil.NewDefaultAuthenticationInfoResolver(kubeconfigFile)
	if err != nil {
		return nil, err
	}
	// Set defaults which may be overridden later.
	cm.SetAuthenticationInfoResolver(authInfoResolver)
	cm.SetServiceResolver(webhookutil.NewDefaultServiceResolver())

	return &Webhook{
		Handler:          handler,
		sourceFactory:    sourceFactory,
		clientManager:    &cm,
		namespaceMatcher: &namespace.Matcher{},
		objectMatcher:    &object.Matcher{},
		dispatcher:       dispatcherFactory(&cm),
	}, nil
}

// SetAuthenticationInfoResolverWrapper sets the
// AuthenticationInfoResolverWrapper.
// TODO find a better way wire this, but keep this pull small for now.
func (a *Webhook) SetAuthenticationInfoResolverWrapper(wrapper webhookutil.AuthenticationInfoResolverWrapper) {
	a.clientManager.SetAuthenticationInfoResolverWrapper(wrapper)
}

// SetServiceResolver sets a service resolver for the webhook admission plugin.
// Passing a nil resolver does not have an effect, instead a default one will be used.
func (a *Webhook) SetServiceResolver(sr webhookutil.ServiceResolver) {
	a.clientManager.SetServiceResolver(sr)
}

// SetExternalKubeClientSet implements the WantsExternalKubeInformerFactory interface.
// It sets external ClientSet for admission plugins that need it
func (a *Webhook) SetExternalKubeClientSet(client clientset.Interface) {
	a.namespaceMatcher.Client = client
}

// SetExternalKubeInformerFactory implements the WantsExternalKubeInformerFactory interface.
func (a *Webhook) SetExternalKubeInformerFactory(f informers.SharedInformerFactory) {
	namespaceInformer := f.Core().V1().Namespaces()
	a.namespaceMatcher.NamespaceLister = namespaceInformer.Lister()
	a.hookSource = a.sourceFactory(f)
	a.SetReadyFunc(func() bool {
		return namespaceInformer.Informer().HasSynced() && a.hookSource.HasSynced()
	})
}

// ValidateInitialization implements the InitializationValidator interface.
func (a *Webhook) ValidateInitialization() error {
	if a.hookSource == nil {
		return fmt.Errorf("kubernetes client is not properly setup")
	}
	if err := a.namespaceMatcher.Validate(); err != nil {
		return fmt.Errorf("namespaceMatcher is not properly setup: %v", err)
	}
	if err := a.clientManager.Validate(); err != nil {
		return fmt.Errorf("clientManager is not properly setup: %v", err)
	}
	return nil
}

// ShouldCallHook returns invocation details if the webhook should be called, nil if the webhook should not be called,
// or an error if an error was encountered during evaluation.
func (a *Webhook) ShouldCallHook(h webhook.WebhookAccessor, attr admission.Attributes, o admission.ObjectInterfaces) (*WebhookInvocation, *apierrors.StatusError) {
	var err *apierrors.StatusError
	var invocation *WebhookInvocation
	for _, r := range h.GetRules() {
		m := rules.Matcher{Rule: r, Attr: attr}
		if m.Matches() {
			invocation = &WebhookInvocation{
				Webhook:     h,
				Resource:    attr.GetResource(),
				Subresource: attr.GetSubresource(),
				Kind:        attr.GetKind(),
			}
			break
		}
	}
	if invocation == nil && h.GetMatchPolicy() != nil && *h.GetMatchPolicy() == v1beta1.Equivalent {
		attrWithOverride := &attrWithResourceOverride{Attributes: attr}
		equivalents := o.GetEquivalentResourceMapper().EquivalentResourcesFor(attr.GetResource(), attr.GetSubresource())
		// honor earlier rules first
	OuterLoop:
		for _, r := range h.GetRules() {
			// see if the rule matches any of the equivalent resources
			for _, equivalent := range equivalents {
				if equivalent == attr.GetResource() {
					// exclude attr.GetResource(), which we already checked
					continue
				}
				attrWithOverride.resource = equivalent
				m := rules.Matcher{Rule: r, Attr: attrWithOverride}
				if m.Matches() {
					kind := o.GetEquivalentResourceMapper().KindFor(equivalent, attr.GetSubresource())
					if kind.Empty() {
						return nil, apierrors.NewInternalError(fmt.Errorf("unable to convert to %v: unknown kind", equivalent))
					}
					invocation = &WebhookInvocation{
						Webhook:     h,
						Resource:    equivalent,
						Subresource: attr.GetSubresource(),
						Kind:        kind,
					}
					break OuterLoop
				}
			}
		}
	}

	if invocation == nil {
		return nil, nil
	}

	matches, err := a.namespaceMatcher.MatchNamespaceSelector(h, attr)
	if !matches || err != nil {
		return nil, err
	}

	matches, err = a.objectMatcher.MatchObjectSelector(h, attr)
	if !matches || err != nil {
		return nil, err
	}

	return invocation, nil
}

type attrWithResourceOverride struct {
	admission.Attributes
	resource schema.GroupVersionResource
}

func (a *attrWithResourceOverride) GetResource() schema.GroupVersionResource { return a.resource }

// Dispatch is called by the downstream Validate or Admit methods.
func (a *Webhook) Dispatch(attr admission.Attributes, o admission.ObjectInterfaces) error {
	if rules.IsWebhookConfigurationResource(attr) {
		return nil
	}
	if !a.WaitForReady() {
		return admission.NewForbidden(attr, fmt.Errorf("not yet ready to handle request"))
	}
	hooks := a.hookSource.Webhooks()
	// TODO: Figure out if adding one second timeout make sense here.
	ctx := context.TODO()

	return a.dispatcher.Dispatch(ctx, attr, o, hooks)
}
