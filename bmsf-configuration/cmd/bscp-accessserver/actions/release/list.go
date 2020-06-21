/*
Tencent is pleased to support the open source community by making Blueking Container Service available.
Copyright (C) 2019 THL A29 Limited, a Tencent company. All rights reserved.
Licensed under the MIT License (the "License"); you may not use this file except
in compliance with the License. You may obtain a copy of the License at
http://opensource.org/licenses/MIT
Unless required by applicable law or agreed to in writing, software distributed under
the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
either express or implied. See the License for the specific language governing permissions and
limitations under the License.
*/

package release

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/viper"

	"bk-bscp/internal/database"
	pb "bk-bscp/internal/protocol/accessserver"
	pbbusinessserver "bk-bscp/internal/protocol/businessserver"
	pbcommon "bk-bscp/internal/protocol/common"
	"bk-bscp/pkg/logger"
)

// ListAction query release list.
type ListAction struct {
	viper    *viper.Viper
	buSvrCli pbbusinessserver.BusinessClient

	req  *pb.QueryHistoryReleasesReq
	resp *pb.QueryHistoryReleasesResp

	releases []*pbcommon.Release
}

// NewListAction creates new ListAction.
func NewListAction(viper *viper.Viper, buSvrCli pbbusinessserver.BusinessClient,
	req *pb.QueryHistoryReleasesReq, resp *pb.QueryHistoryReleasesResp) *ListAction {
	action := &ListAction{viper: viper, buSvrCli: buSvrCli, req: req, resp: resp}

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
	act.resp.Releases = act.releases
	return nil
}

func (act *ListAction) verify() error {
	length := len(act.req.Bid)
	if length == 0 {
		return errors.New("invalid params, bid missing")
	}
	if length > database.BSCPIDLENLIMIT {
		return errors.New("invalid params, bid too long")
	}

	length = len(act.req.Cfgsetid)
	if length == 0 {
		return errors.New("invalid params, cfgsetid missing")
	}
	if length > database.BSCPIDLENLIMIT {
		return errors.New("invalid params, cfgsetid too long")
	}

	if act.req.Limit == 0 {
		return errors.New("invalid params, limit missing")
	}
	if act.req.Limit > database.BSCPQUERYLIMIT {
		return errors.New("invalid params, limit too big")
	}

	if len(act.req.Operator) > database.BSCPNAMELENLIMIT {
		return errors.New("invalid params, operator too long")
	}
	return nil
}

func (act *ListAction) list() (pbcommon.ErrCode, string) {
	r := &pbbusinessserver.QueryHistoryReleasesReq{
		Seq:       act.req.Seq,
		Bid:       act.req.Bid,
		Cfgsetid:  act.req.Cfgsetid,
		Operator:  act.req.Operator,
		QueryType: act.req.QueryType,
		Index:     act.req.Index,
		Limit:     act.req.Limit,
		OrderType: act.req.OrderType,
	}

	ctx, cancel := context.WithTimeout(context.Background(), act.viper.GetDuration("businessserver.calltimeout"))
	defer cancel()

	logger.V(2).Infof("QueryHistoryReleases[%d]| request to businessserver QueryHistoryReleases, %+v", act.req.Seq, r)

	resp, err := act.buSvrCli.QueryHistoryReleases(ctx, r)
	if err != nil {
		return pbcommon.ErrCode_E_AS_SYSTEM_UNKONW, fmt.Sprintf("request to businessserver QueryHistoryReleases, %+v", err)
	}
	act.releases = resp.Releases

	return resp.ErrCode, resp.ErrMsg
}

// Do makes the workflows of this action base on input messages.
func (act *ListAction) Do() error {
	// query release list.
	if errCode, errMsg := act.list(); errCode != pbcommon.ErrCode_E_OK {
		return act.Err(errCode, errMsg)
	}
	return nil
}
