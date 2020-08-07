/*
 * Tencent is pleased to support the open source community by making Blueking Container Service available.
 * Copyright (C) 2019 THL A29 Limited, a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 * http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing, software distributed under
 * the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

// Code generated by client-gen. DO NOT EDIT.

package v1beta1

import (
	v1beta1 "github.com/Tencent/bk-bcs/bcs-k8s/bcs-k8s-watch/pkg/kubefed/apis/types/v1beta1"
	scheme "github.com/Tencent/bk-bcs/bcs-k8s/bcs-k8s-watch/pkg/kubefed/client/clientset/versioned/scheme"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// FederatedReplicationControllersGetter has a method to return a FederatedReplicationControllerInterface.
// A group's client should implement this interface.
type FederatedReplicationControllersGetter interface {
	FederatedReplicationControllers(namespace string) FederatedReplicationControllerInterface
}

// FederatedReplicationControllerInterface has methods to work with FederatedReplicationController resources.
type FederatedReplicationControllerInterface interface {
	Create(*v1beta1.FederatedReplicationController) (*v1beta1.FederatedReplicationController, error)
	Update(*v1beta1.FederatedReplicationController) (*v1beta1.FederatedReplicationController, error)
	UpdateStatus(*v1beta1.FederatedReplicationController) (*v1beta1.FederatedReplicationController, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*v1beta1.FederatedReplicationController, error)
	List(opts v1.ListOptions) (*v1beta1.FederatedReplicationControllerList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1beta1.FederatedReplicationController, err error)
	FederatedReplicationControllerExpansion
}

// federatedReplicationControllers implements FederatedReplicationControllerInterface
type federatedReplicationControllers struct {
	client rest.Interface
	ns     string
}

// newFederatedReplicationControllers returns a FederatedReplicationControllers
func newFederatedReplicationControllers(c *TypesV1beta1Client, namespace string) *federatedReplicationControllers {
	return &federatedReplicationControllers{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the federatedReplicationController, and returns the corresponding federatedReplicationController object, and an error if there is any.
func (c *federatedReplicationControllers) Get(name string, options v1.GetOptions) (result *v1beta1.FederatedReplicationController, err error) {
	result = &v1beta1.FederatedReplicationController{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("federatedreplicationcontrollers").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of FederatedReplicationControllers that match those selectors.
func (c *federatedReplicationControllers) List(opts v1.ListOptions) (result *v1beta1.FederatedReplicationControllerList, err error) {
	result = &v1beta1.FederatedReplicationControllerList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("federatedreplicationcontrollers").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested federatedReplicationControllers.
func (c *federatedReplicationControllers) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("federatedreplicationcontrollers").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a federatedReplicationController and creates it.  Returns the server's representation of the federatedReplicationController, and an error, if there is any.
func (c *federatedReplicationControllers) Create(federatedReplicationController *v1beta1.FederatedReplicationController) (result *v1beta1.FederatedReplicationController, err error) {
	result = &v1beta1.FederatedReplicationController{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("federatedreplicationcontrollers").
		Body(federatedReplicationController).
		Do().
		Into(result)
	return
}

// Update takes the representation of a federatedReplicationController and updates it. Returns the server's representation of the federatedReplicationController, and an error, if there is any.
func (c *federatedReplicationControllers) Update(federatedReplicationController *v1beta1.FederatedReplicationController) (result *v1beta1.FederatedReplicationController, err error) {
	result = &v1beta1.FederatedReplicationController{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("federatedreplicationcontrollers").
		Name(federatedReplicationController.Name).
		Body(federatedReplicationController).
		Do().
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().

func (c *federatedReplicationControllers) UpdateStatus(federatedReplicationController *v1beta1.FederatedReplicationController) (result *v1beta1.FederatedReplicationController, err error) {
	result = &v1beta1.FederatedReplicationController{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("federatedreplicationcontrollers").
		Name(federatedReplicationController.Name).
		SubResource("status").
		Body(federatedReplicationController).
		Do().
		Into(result)
	return
}

// Delete takes name of the federatedReplicationController and deletes it. Returns an error if one occurs.
func (c *federatedReplicationControllers) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("federatedreplicationcontrollers").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *federatedReplicationControllers) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("federatedreplicationcontrollers").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched federatedReplicationController.
func (c *federatedReplicationControllers) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1beta1.FederatedReplicationController, err error) {
	result = &v1beta1.FederatedReplicationController{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("federatedreplicationcontrollers").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
