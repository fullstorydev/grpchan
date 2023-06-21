/*
 *
 * Copyright 2015 gRPC authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

// Package main implements a server for Greeter service.
package main

import (
	"flag"
	"fmt"
	"log"
	"net"

	"github.com/fullstorydev/grpchan/grpchantesting"
	"google.golang.org/grpc"
)

var (
	port = flag.Int("port", 50051, "The server port")
)

func main() {

	svc := &grpchantesting.TestServer{}
	// svr := httpgrpc.NewServer(httpgrpc.WithBasePath("/foo/"), httpgrpc.ErrorRenderer(errFunc))

	flag.Parse()
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	svr := grpc.NewServer()
	grpchantesting.RegisterTestServiceServer(svr, svc)

	log.Printf("server listening at %v", lis.Addr())
	if err := svr.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}

	defer svr.Stop()

}
