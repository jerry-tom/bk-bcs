/*
 * Tencent is pleased to support the open source community by making Blueking Container Service available.,
 * Copyright (C) 2019 THL A29 Limited, a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 * http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing, software distributed under,
 * the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing permissions and
 * limitations under the License.
 */

package template

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/viper"

	pb "bk-bscp/internal/protocol/accessserver"
	pbcommon "bk-bscp/internal/protocol/common"
	pbtemplateserver "bk-bscp/internal/protocol/templateserver"
	"bk-bscp/pkg/common"
	"bk-bscp/pkg/logger"
)

// ListAction delete a config template set object
type ListAction struct {
	viper          *viper.Viper
	templateClient pbtemplateserver.TemplateClient
	req            *pb.QueryConfigTemplateSetListReq
	resp           *pb.QueryConfigTemplateSetListResp
}

// NewListAction creates new ListAction
func NewListAction(viper *viper.Viper, templateClient pbtemplateserver.TemplateClient,
	req *pb.QueryConfigTemplateSetListReq, resp *pb.QueryConfigTemplateSetListResp) *ListAction {
	action := &ListAction{viper: viper, templateClient: templateClient, req: req, resp: resp}

	action.resp.Seq = req.Seq
	action.resp.ErrCode = pbcommon.ErrCode_E_OK
	action.resp.ErrMsg = "OK"

	return action
}

// Err setup error code message in response and return the error.
func (act *ListAction) Err(errCode pbcommon.ErrCode, errMsg string) error {
	act.resp.ErrCode = errCode
	act.resp.ErrMsg = errMsg
	return errors.New(errMsg)
}

// Input handles the input messages.
func (act *ListAction) Input() error {
	if err := act.verify(); err != nil {
		return act.Err(pbcommon.ErrCode_E_AS_PARAMS_INVALID, err.Error())
	}
	return nil
}

// Output handles the output messages.
func (act *ListAction) Output() error {
	// do nothing.
	return nil
}

func (act *ListAction) verify() error {
	if err := common.VerifyID(act.req.Bid, "bid"); err != nil {
		return err
	}

	if err := common.VerifyQueryLimit(act.req.Limit); err != nil {
		return nil
	}

	return nil
}

func (act *ListAction) listTemplateSet() (pbcommon.ErrCode, string) {
	req := &pbtemplateserver.QueryConfigTemplateSetListReq{
		Seq:   act.req.Seq,
		Bid:   act.req.Bid,
		Index: act.req.Index,
		Limit: act.req.Limit,
	}

	ctx, cancel := context.WithTimeout(context.Background(), act.viper.GetDuration("templateserver.calltimeout"))
	defer cancel()

	logger.V(2).Infof("QueryConfigTemplateSetList[%d]| request to templateserver QueryConfigTemplateSetList, %+v", req.Seq, req)

	resp, err := act.templateClient.QueryConfigTemplateSetList(ctx, req)
	if err != nil {
		return pbcommon.ErrCode_E_AS_SYSTEM_UNKONW, fmt.Sprintf("request to templateserver QueryConfigTemplateSetList, %+v", err)
	}
	if resp.ErrCode != pbcommon.ErrCode_E_OK {
		return resp.ErrCode, resp.ErrMsg
	}

	act.resp.TemplateSets = resp.TemplateSets

	return pbcommon.ErrCode_E_OK, "OK"
}

// Do do action
func (act *ListAction) Do() error {

	// list config template set
	if errCode, errMsg := act.listTemplateSet(); errCode != pbcommon.ErrCode_E_OK {
		return act.Err(errCode, errMsg)
	}
	return nil
}
