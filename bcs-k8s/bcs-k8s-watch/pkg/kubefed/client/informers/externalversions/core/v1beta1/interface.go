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

// Code generated by informer-gen. DO NOT EDIT.

package v1beta1

import (
	internalinterfaces "bk-bcs/bcs-k8s/bcs-k8s-watch/pkg/kubefed/client/informers/externalversions/internalinterfaces"
)

// Interface provides access to all the informers in this group version.
type Interface interface {
	// FederatedTypeConfigs returns a FederatedTypeConfigInformer.
	FederatedTypeConfigs() FederatedTypeConfigInformer
	// KubeFedClusters returns a KubeFedClusterInformer.
	KubeFedClusters() KubeFedClusterInformer
	// KubeFedConfigs returns a KubeFedConfigInformer.
	KubeFedConfigs() KubeFedConfigInformer
}

type version struct {
	factory          internalinterfaces.SharedInformerFactory
	namespace        string
	tweakListOptions internalinterfaces.TweakListOptionsFunc
}

// New returns a new Interface.
func New(f internalinterfaces.SharedInformerFactory, namespace string, tweakListOptions internalinterfaces.TweakListOptionsFunc) Interface {
	return &version{factory: f, namespace: namespace, tweakListOptions: tweakListOptions}
}

// FederatedTypeConfigs returns a FederatedTypeConfigInformer.
func (v *version) FederatedTypeConfigs() FederatedTypeConfigInformer {
	return &federatedTypeConfigInformer{factory: v.factory, namespace: v.namespace, tweakListOptions: v.tweakListOptions}
}

// KubeFedClusters returns a KubeFedClusterInformer.
func (v *version) KubeFedClusters() KubeFedClusterInformer {
	return &kubeFedClusterInformer{factory: v.factory, namespace: v.namespace, tweakListOptions: v.tweakListOptions}
}

// KubeFedConfigs returns a KubeFedConfigInformer.
func (v *version) KubeFedConfigs() KubeFedConfigInformer {
	return &kubeFedConfigInformer{factory: v.factory, namespace: v.namespace, tweakListOptions: v.tweakListOptions}
}
