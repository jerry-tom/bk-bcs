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

package main

import (
	"fmt"
	"github.com/Tencent/bk-bcs/bcs-common/common/blog"
	"github.com/Tencent/bk-bcs/bcs-common/common/conf"
	"github.com/Tencent/bk-bcs/bmsf-mesh/bmsf-mesos-adapter/app"
	"os"
)

func main() {
	cfg := &app.Config{}
	conf.Parse(cfg)
	if err := cfg.Validate(); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	blog.InitLogs(cfg.LogConfig)
	defer blog.CloseLogs()
	if err := app.Run(cfg); err != nil {
		fmt.Printf("bmsf-mesos-adaptor starting failed: %v", err)
		os.Exit(1)
	}
}
