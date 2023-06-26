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

package main

import (
	"context"
	flag2 "flag"
	"fmt"

	"github.com/imkuqin-zw/yggdrasil"
	_ "github.com/imkuqin-zw/yggdrasil-polaris"
	"github.com/imkuqin-zw/yggdrasil-polaris/exmaple/common/proto"
	"github.com/imkuqin-zw/yggdrasil/pkg/config"
	"github.com/imkuqin-zw/yggdrasil/pkg/config/source/file"
	"github.com/imkuqin-zw/yggdrasil/pkg/config/source/flag"
	_ "github.com/imkuqin-zw/yggdrasil/pkg/interceptor/logger"
	"github.com/imkuqin-zw/yggdrasil/pkg/logger"
	_ "github.com/imkuqin-zw/yggdrasil/pkg/remote/protocol/grpc"
)

type GreeterCircuitBreakerService struct {
	helloword.UnimplementedGreeterServer
}

func (h *GreeterCircuitBreakerService) SayHello(_ context.Context, request *helloword.HelloRequest) (*helloword.HelloReply, error) {
	return &helloword.HelloReply{Message: request.Name}, nil
	//return nil, status.New(code.Code_INTERNAL, errors.New("error"))
}

var (
	_ = flag2.String("server-name", "0", "server name")
)

func main() {
	if err := config.LoadSource(file.NewSource("./config.yaml", false)); err != nil {
		logger.FatalFiled("fault to load config file", logger.Err(err))
	}
	if err := config.LoadSource(flag.NewSource()); err != nil {
		logger.FatalFiled("fault to load config file", logger.Err(err))
	}
	name := config.Get("server.name").String("0")
	fmt.Println("server_name:", name)
	if err := yggdrasil.Run("github.com.imkuqin_zw.yggdrasil_polaris.example.server."+name,
		yggdrasil.WithServiceDesc(&helloword.GreeterServiceDesc, &GreeterCircuitBreakerService{}),
	); err != nil {
		logger.FatalFiled("the application was ended forcefully ", logger.Err(err))
	}
}
