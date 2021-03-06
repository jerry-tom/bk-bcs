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

package strategy

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

// CreateAction creates a strategy object.
type CreateAction struct {
	viper    *viper.Viper
	buSvrCli pbbusinessserver.BusinessClient

	req  *pb.CreateStrategyReq
	resp *pb.CreateStrategyResp
}

// NewCreateAction creates new CreateAction.
func NewCreateAction(viper *viper.Viper, buSvrCli pbbusinessserver.BusinessClient,
	req *pb.CreateStrategyReq, resp *pb.CreateStrategyResp) *CreateAction {
	action := &CreateAction{viper: viper, buSvrCli: buSvrCli, req: req, resp: resp}

	action.resp.Seq = req.Seq
	action.resp.ErrCode = pbcommon.ErrCode_E_OK
	action.resp.ErrMsg = "OK"

	return action
}

// Err setup error code message in response and return the error.
func (act *CreateAction) Err(errCode pbcommon.ErrCode, errMsg string) error {
	act.resp.ErrCode = errCode
	act.resp.ErrMsg = errMsg
	return errors.New(errMsg)
}

// Input handles the input messages.
func (act *CreateAction) Input() error {
	if err := act.verify(); err != nil {
		return act.Err(pbcommon.ErrCode_E_AS_PARAMS_INVALID, err.Error())
	}
	return nil
}

// Output handles the output messages.
func (act *CreateAction) Output() error {
	// do nothing.
	return nil
}

func (act *CreateAction) verify() error {
	length := len(act.req.Bid)
	if length == 0 {
		return errors.New("invalid params, bid missing")
	}
	if length > database.BSCPIDLENLIMIT {
		return errors.New("invalid params, bid too long")
	}

	length = len(act.req.Appid)
	if length == 0 {
		return errors.New("invalid params, appid missing")
	}
	if length > database.BSCPIDLENLIMIT {
		return errors.New("invalid params, appid too long")
	}

	length = len(act.req.Name)
	if length == 0 {
		return errors.New("invalid params, name missing")
	}
	if length > database.BSCPNAMELENLIMIT {
		return errors.New("invalid params, name too long")
	}

	if act.req.Clusterids == nil {
		act.req.Clusterids = []string{}
	}
	if len(act.req.Clusterids) > database.BSCPBATCHLIMIT {
		return errors.New("invalid params, clusterids list too long")
	}

	if act.req.Zoneids == nil {
		act.req.Zoneids = []string{}
	}
	if len(act.req.Zoneids) > database.BSCPBATCHLIMIT {
		return errors.New("invalid params, zoneids list too long")
	}

	if act.req.Dcs == nil {
		act.req.Dcs = []string{}
	}
	if len(act.req.Dcs) > database.BSCPBATCHLIMIT {
		return errors.New("invalid params, dcs list too long")
	}

	if act.req.IPs == nil {
		act.req.IPs = []string{}
	}
	if len(act.req.IPs) > database.BSCPBATCHLIMIT {
		return errors.New("invalid params, ips list too long")
	}

	if act.req.Labels == nil {
		act.req.Labels = make(map[string]string)
	}
	if len(act.req.Labels) > database.BSCPBATCHLIMIT {
		return errors.New("invalid params, labels set too large")
	}

	length = len(act.req.Creator)
	if length == 0 {
		return errors.New("invalid params, creator missing")
	}
	if length > database.BSCPNAMELENLIMIT {
		return errors.New("invalid params, creator too long")
	}

	if len(act.req.Memo) > database.BSCPLONGSTRLENLIMIT {
		return errors.New("invalid params, memo too long")
	}
	return nil
}

func (act *CreateAction) create() (pbcommon.ErrCode, string) {
	r := &pbbusinessserver.CreateStrategyReq{
		Seq:        act.req.Seq,
		Bid:        act.req.Bid,
		Appid:      act.req.Appid,
		Name:       act.req.Name,
		Clusterids: act.req.Clusterids,
		Zoneids:    act.req.Zoneids,
		Dcs:        act.req.Dcs,
		IPs:        act.req.IPs,
		Labels:     act.req.Labels,
		Memo:       act.req.Memo,
		Creator:    act.req.Creator,
	}

	ctx, cancel := context.WithTimeout(context.Background(), act.viper.GetDuration("businessserver.calltimeout"))
	defer cancel()

	logger.V(2).Infof("CreateStrategy[%d]| request to businessserver CreateStrategy, %+v", act.req.Seq, r)

	resp, err := act.buSvrCli.CreateStrategy(ctx, r)
	if err != nil {
		return pbcommon.ErrCode_E_AS_SYSTEM_UNKONW, fmt.Sprintf("request to businessserver CreateStrategy, %+v", err)
	}
	act.resp.Strategyid = resp.Strategyid

	return resp.ErrCode, resp.ErrMsg
}

// Do makes the workflows of this action base on input messages.
func (act *CreateAction) Do() error {
	// create strategy.
	if errCode, errMsg := act.create(); errCode != pbcommon.ErrCode_E_OK {
		return act.Err(errCode, errMsg)
	}
	return nil
}
