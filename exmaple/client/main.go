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
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/imkuqin-zw/yggdrasil"
	_ "github.com/imkuqin-zw/yggdrasil-polaris"
	"github.com/imkuqin-zw/yggdrasil-polaris/exmaple/common/proto"
	"github.com/imkuqin-zw/yggdrasil/pkg/config"
	"github.com/imkuqin-zw/yggdrasil/pkg/config/source/file"
	_ "github.com/imkuqin-zw/yggdrasil/pkg/interceptor/logger"
	"github.com/imkuqin-zw/yggdrasil/pkg/logger"
	"github.com/imkuqin-zw/yggdrasil/pkg/metadata"
	_ "github.com/imkuqin-zw/yggdrasil/pkg/remote/protocol/grpc"
)

const (
	listenPort   = 6001
	defaultCount = 20
)

func main() {
	if err := config.LoadSource(file.NewSource("./config.yaml", false)); err != nil {
		logger.Fatal(err)
	}
	yggdrasil.Init("github.com.imkuqin_zw.yggdrasil_polaris.example.client")
	echoClient := helloword.NewGreeterClient(yggdrasil.NewClient("github.com.imkuqin_zw.yggdrasil_polaris.example.server"))
	go yggdrasil.Serve()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	address := fmt.Sprintf("0.0.0.0:%d", listenPort)
	listen, err := net.Listen("tcp", address)
	if err != nil {
		logger.Fatalf("Failed to listen addr %s: %v", address, err)
	}
	listenAddr := listen.Addr().String()
	fmt.Printf("listen address is %s\n", listenAddr)

	echoHandler := &EchoHandler{
		echoClient: echoClient,
		ctx:        ctx,
	}

	if err := http.Serve(listen, echoHandler); nil != err {
		log.Fatal(err)
	}
}

// EchoHandler is a http.Handler that implements the echo service.
type EchoHandler struct {
	echoClient helloword.GreeterClient

	ctx context.Context
}

func (s *EchoHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		return
	}
	//err := r.ParseForm()
	//if nil != err {
	//	log.Printf("fail to parse request form: %v\n", err)
	//	w.WriteHeader(500)
	//	_, _ = w.Write([]byte(err.Error()))
	//	return
	//}
	defer func() {
		if rec := recover(); rec != nil {
			fmt.Printf("%+v", rec)
		}
	}()
	fmt.Println(r.URL.RawQuery)
	params, _ := url.ParseQuery(r.URL.RawQuery)

	value := params.Get("value")
	log.Printf("receive value is %s\n", value)
	tagVal := params.Get("tag")
	log.Printf("receive tag is %s\n", tagVal)
	//var value string
	//if len(values) > 0 {
	//	value = values[0]
	//}

	counts := params.Get("count")
	log.Printf("receive count is %s\n", counts)
	count := defaultCount
	if len(counts) > 0 {
		v, err := strconv.Atoi(counts)
		if nil != err {
			log.Printf("parse count value %s into int fail, err: %s", counts, err)
		}
		if v > 0 {
			count = v
		}
	}
	builder := strings.Builder{}
	for i := 0; i < count; i++ {
		ctx, cancel := context.WithTimeout(s.ctx, 3*time.Second)
		ctx = metadata.WithOutContext(ctx, metadata.New(map[string]string{"tag": tagVal, "dd": "1"}))
		resp, err := s.echoClient.SayHello(ctx, &helloword.HelloRequest{Name: value})
		cancel()
		if nil != err {
			builder.Write([]byte(err.Error()))
			builder.WriteByte('\n')
			continue
		}
		builder.Write([]byte(resp.GetMessage()))
		builder.WriteByte('\n')
	}
	w.WriteHeader(200)
	_, _ = w.Write([]byte(builder.String()))
	time.Sleep(100 * time.Millisecond)
}
