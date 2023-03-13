// Copyright 2022 The imkuqin-zw Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package polaris

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	config2 "github.com/imkuqin-zw/yggdrasil/pkg/config"
	"github.com/imkuqin-zw/yggdrasil/pkg/logger"
	resolver2 "github.com/imkuqin-zw/yggdrasil/pkg/resolver"
	"github.com/imkuqin-zw/yggdrasil/pkg/utils/xgo"
	"github.com/polarismesh/polaris-go/api"
	"github.com/polarismesh/polaris-go/pkg/model"
)

func init() {
	resolver2.RegisterBuilder(name, newResolver)
}

type resolver struct {
	wg         sync.WaitGroup
	sourceInfo *model.ServiceInfo
	mu         sync.RWMutex
	closed     bool
	watcher    map[string]*watcherInstance
}

func newResolver(_ string) (resolver2.Resolver, error) {
	rs := &resolver{watcher: map[string]*watcherInstance{}}
	rs.sourceInfo = buildSourceInfo()
	return rs, nil
}

func (pr *resolver) AddWatch(serviceName string) (err error) {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	if pr.closed {
		return errors.New("resolver closed")
	}
	if _, ok := pr.watcher[serviceName]; ok {
		return nil
	}
	watcher := &watcherInstance{
		pr: pr,
	}
	watcher.ctx, watcher.cancel = context.WithCancel(context.Background())
	defer func() {
		if err != nil {
			watcher.cancel()
		}
	}()
	watcher.info = getDstServiceInfo(serviceName)
	sdkCtx, err := Context()
	if nil != err {
		return err
	}
	watcher.consumer = api.NewConsumerAPIByContext(sdkCtx)
	pr.watcher[serviceName] = watcher
	xgo.Go(func() {
		watcher.watch()
	}, nil)
	return nil
}

func (pr *resolver) DelWatch(serviceName string) error {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	if pr.closed {
		return nil
	}
	w, ok := pr.watcher[serviceName]
	if !ok {
		return nil
	}
	w.stop()
	return nil
}

func (pr *resolver) Close() error {
	pr.mu.Lock()
	if pr.closed {
		return nil
	}
	pr.closed = true
	pr.mu.Unlock()
	for _, item := range pr.watcher {
		item.stop()
	}
	pr.wg.Wait()
	return nil
}

func (pr *resolver) Name() string {
	return name
}

type watcherInstance struct {
	info     *DstServiceInfo
	pr       *resolver
	consumer api.ConsumerAPI
	ctx      context.Context
	cancel   context.CancelFunc
}

func (w *watcherInstance) lookup() {
	info := w.info
	instancesRequest := &api.GetInstancesRequest{}
	instancesRequest.Namespace = info.Namespace
	instancesRequest.Service = info.ServiceName
	instancesRequest.SourceService = w.pr.sourceInfo
	instancesRequest.SkipRouteFilter = true
	if len(info.DstMetadata) > 0 {
		instancesRequest.Metadata = info.DstMetadata
	}
	resp, err := w.consumer.GetInstances(instancesRequest)
	if nil != err {
		logger.ErrorFiled("fault to get instances", logger.String("serviceName", w.info.ServiceName), logger.Err(err))
		return
	}
	var endpoints = make([]interface{}, 0, len(resp.Instances))
	for _, instance := range resp.Instances {
		endpoint := map[string]interface{}{
			config2.KeySingleAddress:  fmt.Sprintf("%s:%d", instance.GetHost(), instance.GetPort()),
			config2.KeySingleProtocol: instance.GetProtocol(),
			config2.KeySingleMetadata: instance.GetMetadata(),
		}
		endpoints = append(endpoints, endpoint)
	}
	_ = config2.SetMulti([]string{fmt.Sprintf(config2.KeyClientEndpoints, info.ServiceName)}, []interface{}{endpoints})
	return
}

func (w *watcherInstance) doWatch() <-chan model.SubScribeEvent {
	watchRequest := &api.WatchServiceRequest{}
	watchRequest.Key = model.ServiceKey{
		Namespace: w.info.Namespace,
		Service:   w.info.ServiceName,
	}
	resp, err := w.consumer.WatchService(watchRequest)
	if nil != err {
		logger.ErrorFiled("fail to do watch", logger.String("serviceName", w.info.ServiceName), logger.Err(err))
		return nil
	}
	return resp.EventChannel
}

func (w *watcherInstance) watch() {
	eventChan := w.doWatch()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	w.pr.wg.Add(1)
	defer w.pr.wg.Done()
	for {
		select {
		case <-w.ctx.Done():
			return
		case <-eventChan:
		case <-ticker.C:
		}
		w.lookup()
		eventChan = w.doWatch()
	}
}

func (w *watcherInstance) stop() {
	w.cancel()
	w.consumer.Destroy()
}
