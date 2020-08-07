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

// Code generated by lister-gen. DO NOT EDIT.

package v1beta1

import (
	v1beta1 "github.com/Tencent/bk-bcs/bcs-k8s/bcs-k8s-watch/pkg/kubefed/apis/types/v1beta1"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// FederatedEndpointsLister helps list FederatedEndpointses.
type FederatedEndpointsLister interface {
	// List lists all FederatedEndpointses in the indexer.
	List(selector labels.Selector) (ret []*v1beta1.FederatedEndpoints, err error)
	// FederatedEndpointses returns an object that can list and get FederatedEndpointses.
	FederatedEndpointses(namespace string) FederatedEndpointsNamespaceLister
	FederatedEndpointsListerExpansion
}

// federatedEndpointsLister implements the FederatedEndpointsLister interface.
type federatedEndpointsLister struct {
	indexer cache.Indexer
}

// NewFederatedEndpointsLister returns a new FederatedEndpointsLister.
func NewFederatedEndpointsLister(indexer cache.Indexer) FederatedEndpointsLister {
	return &federatedEndpointsLister{indexer: indexer}
}

// List lists all FederatedEndpointses in the indexer.
func (s *federatedEndpointsLister) List(selector labels.Selector) (ret []*v1beta1.FederatedEndpoints, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1beta1.FederatedEndpoints))
	})
	return ret, err
}

// FederatedEndpointses returns an object that can list and get FederatedEndpointses.
func (s *federatedEndpointsLister) FederatedEndpointses(namespace string) FederatedEndpointsNamespaceLister {
	return federatedEndpointsNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// FederatedEndpointsNamespaceLister helps list and get FederatedEndpointses.
type FederatedEndpointsNamespaceLister interface {
	// List lists all FederatedEndpointses in the indexer for a given namespace.
	List(selector labels.Selector) (ret []*v1beta1.FederatedEndpoints, err error)
	// Get retrieves the FederatedEndpoints from the indexer for a given namespace and name.
	Get(name string) (*v1beta1.FederatedEndpoints, error)
	FederatedEndpointsNamespaceListerExpansion
}

// federatedEndpointsNamespaceLister implements the FederatedEndpointsNamespaceLister
// interface.
type federatedEndpointsNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all FederatedEndpointses in the indexer for a given namespace.
func (s federatedEndpointsNamespaceLister) List(selector labels.Selector) (ret []*v1beta1.FederatedEndpoints, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1beta1.FederatedEndpoints))
	})
	return ret, err
}

// Get retrieves the FederatedEndpoints from the indexer for a given namespace and name.
func (s federatedEndpointsNamespaceLister) Get(name string) (*v1beta1.FederatedEndpoints, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1beta1.Resource("federatedendpoints"), name)
	}
	return obj.(*v1beta1.FederatedEndpoints), nil
}
