/*
 * Tencent is pleased to support the open source community by making 蓝鲸 available.
 * Copyright (C) 2017-2018 THL A29 Limited, a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 * http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing, software distributed under
 * the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing permissions and
 * limitations under the License.
 */

package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	restful "github.com/emicklei/go-restful"

	"configcenter/src/common/backbone"
	cc "configcenter/src/common/backbone/configcenter"
	"configcenter/src/common/blog"
	"configcenter/src/common/types"
	"configcenter/src/common/version"
	"configcenter/src/scene_server/event_server/app/options"
	"configcenter/src/scene_server/event_server/distribution"
	svc "configcenter/src/scene_server/event_server/service"
	"configcenter/src/storage/dal/mongo"
	"configcenter/src/storage/dal/mongo/local"
	"configcenter/src/storage/dal/redis"
	"configcenter/src/storage/rpc"
)

func Run(ctx context.Context, op *options.ServerOption) error {
	svrInfo, err := newServerInfo(op)
	if err != nil {
		return fmt.Errorf("wrap server info failed, err: %v", err)
	}

	service := svc.NewService(ctx)

	process := new(EventServer)
	input := &backbone.BackboneParameter{
		ConfigUpdate: process.onHostConfigUpdate,
		ConfigPath:   op.ServConf.ExConfig,
		Regdiscv:     op.ServConf.RegDiscover,
		SrvInfo:      svrInfo,
	}
	engine, err := backbone.NewBackbone(ctx, input)
	if err != nil {
		return fmt.Errorf("new backbone failed, err: %v", err)
	}

	service.Engine = engine
	process.Core = engine
	process.Service = service
	errCh := make(chan error, 1)
	for {
		if process.Config == nil {
			time.Sleep(time.Second * 2)
			blog.V(3).Info("config not found, retry 2s later")
			continue
		}
		db, err := local.NewMgo(process.Config.MongoDB.BuildURI(), time.Minute)
		if err != nil {
			return fmt.Errorf("connect mongo server failed %s", err.Error())
		}
		process.Service.SetDB(db)

		cache, err := redis.NewFromConfig(process.Config.Redis)
		if err != nil {
			return fmt.Errorf("connect redis server failed %s", err.Error())
		}
		process.Service.SetCache(cache)

		rpccli, err := rpc.NewClientPool("tcp", engine.ServiceManageInterface.TMServer().GetServers, "/txn/v3/rpc")
		if err != nil {
			return fmt.Errorf("connect rpc server failed %s", err.Error())
		}

		subcli, err := redis.NewFromConfig(process.Config.Redis)
		if err != nil {
			return fmt.Errorf("connect subcli redis server failed %s", err.Error())
		}

		go func() {
			errCh <- distribution.SubscribeChannel(subcli)
		}()

		go func() {
			errCh <- distribution.Start(ctx, cache, db, rpccli)
		}()
		break
	}
	if err := backbone.StartServer(ctx, engine, restful.NewContainer().Add(service.WebService())); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
	case err = <-errCh:
		blog.V(3).Infof("distribution routine stoped %v", err)
	}
	blog.V(3).Infof("process stoped")

	return nil
}

type EventServer struct {
	Core    *backbone.Engine
	Config  *options.Config
	Service *svc.Service
}

var configLock sync.Mutex

func (h *EventServer) onHostConfigUpdate(previous, current cc.ProcessConfig) {
	configLock.Lock()
	defer configLock.Unlock()
	if len(current.ConfigMap) > 0 {
		if h.Config == nil {
			h.Config = new(options.Config)
		}
		out, _ := json.MarshalIndent(current.ConfigMap, "", "  ") //ignore err, cause ConfigMap is map[string]string
		blog.Infof("config updated: \n%s", out)
		mongoConf := mongo.ParseConfigFromKV("mongodb", current.ConfigMap)
		h.Config.MongoDB = mongoConf

		redisConf := redis.ParseConfigFromKV("redis", current.ConfigMap)
		h.Config.Redis = redisConf

		h.Config.RPC.Address = current.ConfigMap["rpc.address"]
	}
}

func newServerInfo(op *options.ServerOption) (*types.ServerInfo, error) {
	ip, err := op.ServConf.GetAddress()
	if err != nil {
		return nil, err
	}

	port, err := op.ServConf.GetPort()
	if err != nil {
		return nil, err
	}

	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	info := &types.ServerInfo{
		IP:       ip,
		Port:     port,
		HostName: hostname,
		Scheme:   "http",
		Version:  version.GetVersion(),
		Pid:      os.Getpid(),
	}
	return info, nil
}
