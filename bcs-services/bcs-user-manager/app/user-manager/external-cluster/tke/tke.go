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

package tke

import (
	"fmt"

	"github.com/Tencent/bk-bcs/bcs-services/bcs-user-manager/app/user-manager/external-cluster"
	"github.com/Tencent/bk-bcs/bcs-services/bcs-user-manager/app/user-manager/external-cluster/tke/common"
	"github.com/Tencent/bk-bcs/bcs-services/bcs-user-manager/app/user-manager/storages/sqlstore"
	"github.com/Tencent/bk-bcs/bcs-services/bcs-user-manager/config"
)

const (
	TkeSdkToGetCredentials = "DescribeClusterSecurityInfo"
	HttpScheme             = "https://"
	TkeClusterPort         = ":443"
)

type tkeCluster struct {
	ClusterId        string
	TkeClusterId     string
	TkeClusterRegion string
}

type Client struct {
	*common.Client
}

type Response struct {
	Code     int    `json:"code"`
	Message  string `json:"message"`
	CodeDesc string `json:"codeDesc"`
}

type DescribeClusterSecurityInfoArgs struct {
	ClusterId string `qcloud_arg:"clusterId"`
}

type BindMasterVipLoadBalancerArgs struct {
	ClusterId string `qcloud_arg:"clusterId"`
	SubnetId  string `qcloud_arg:"subnetId"`
}

type BindMasterVipLoadBalanceResponse struct {
	Response
	Data interface{}
}

type GetMasterVipArgs struct {
	ClusterId string `qcloud_arg:"clusterId"`
}

type GetMasterVipResponse struct {
	Response
	Data GetMasterVipRespData
}

type GetMasterVipRespData struct {
	Status string `json:"status"`
}

type DescribeClusterSecurityInfoRespData struct {
	UserName                string `json:"userName"`
	Domain                  string `json:"domain"`
	CertificationAuthority  string `json:"certificationAuthority"`
	PgwEndpoint             string `json:"pgwEndpoint"`
	ClusterExternalEndpoint string `json:"clusterExternalEndpoint"`
	Password                string `json:"password"`
}

type DescribeClusterSecurityInfoResponse struct {
	Response
	Data DescribeClusterSecurityInfoRespData `json:"data"`
}

func NewTkeCluster(clusterId, tkeClusterId, tkeClusterRegion string) external_cluster.ExternalCluster {
	return &tkeCluster{
		ClusterId:        clusterId,
		TkeClusterId:     tkeClusterId,
		TkeClusterRegion: tkeClusterRegion,
	}
}

func (t *tkeCluster) SyncClusterCredentials() error {
	tkeClient, err := NewClient(t.TkeClusterRegion, "GET")
	if err != nil {
		return fmt.Errorf("error when creating tke client: %s", err.Error())
	}

	args := DescribeClusterSecurityInfoArgs{
		ClusterId: t.TkeClusterId,
	}
	response := &DescribeClusterSecurityInfoResponse{}
	err = tkeClient.Invoke(TkeSdkToGetCredentials, args, response)
	if err != nil {
		return fmt.Errorf("error when invoking tke api %s: %s", TkeSdkToGetCredentials, err.Error())
	}
	if response.Code != 0 {
		return fmt.Errorf("%s cluster %s failed, codeDesc: %s, message: %s", TkeSdkToGetCredentials, t.TkeClusterId, response.CodeDesc, response.Message)
	}

	if response.Data.PgwEndpoint == "" || response.Data.Domain == "" {
		return fmt.Errorf("BindMasterVipLoadBalancer failed, pgwEndpoint or domain nil")
	}

	serverAddress := HttpScheme + response.Data.PgwEndpoint + TkeClusterPort
	clusterDomainUrl := HttpScheme + response.Data.Domain + "/"
	err = sqlstore.SaveCredentials(t.ClusterId, serverAddress, response.Data.CertificationAuthority, response.Data.Password, clusterDomainUrl)
	if err != nil {
		return fmt.Errorf("error when updating external cluster credentials to db: %s", err.Error())
	}
	return nil
}

func NewClient(tkeClusterRegion, method string) (*Client, error) {
	tkeSecretId := config.Tke.SecretId
	tkeSecretKey := config.Tke.SecretKey
	tkeCcsHost := config.Tke.CcsHost
	tkeCcsPath := config.Tke.CcsPath
	if tkeSecretId == "" || tkeSecretKey == "" || tkeCcsHost == "" || tkeCcsPath == "" {
		return nil, fmt.Errorf("tke conf invalid")
	}
	credential := common.Credential{
		SecretId:  tkeSecretId,
		SecretKey: tkeSecretKey,
	}

	opts := common.Opts{
		Region: tkeClusterRegion,
		Host:   tkeCcsHost,
		Path:   tkeCcsPath,
		Method: method,
	}

	client, err := common.NewClient(credential, opts)
	if err != nil {
		return &Client{}, err
	}
	return &Client{client}, nil
}
