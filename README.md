# gRPC Channels

[![Build Status](https://circleci.com/gh/fullstorydev/grpchan/tree/master.svg?style=svg)](https://circleci.com/gh/fullstorydev/grpchan/tree/master)
[![Go Report Card](https://goreportcard.com/badge/github.com/fullstorydev/grpchan)](https://goreportcard.com/report/github.com/fullstorydev/grpchan)
[![GoDoc](https://godoc.org/github.com/fullstorydev/grpchan?status.svg)](https://godoc.org/github.com/fullstorydev/grpchan)

This repo provides alternate "channel" implementations for gRPC, that provide
the same interface and semantics (and re-use gRPC generated code) but over a
transport other than the standard HTTP/2-based one provided by the
`google.golang.org/grpc` package.

The ability to use alternate transports, such as HTTP 1.1 or in-process channels,
is often quite useful, for testing but also for providing RPC semantics on top of
different communication primitives. Implementations could even be built that use
things like web sockets or streaming frameworks (like publish/subscribe systems
or message brokers).

This repo contains two alternate transports: an HTTP 1.1 implementation (which
supports all stream kinds other than full-duplex bidi streams) and an in-process
transport (which allows a process to dispatch handlers implemented in the same
program without needing to serialize and de-serialize messages over the loopback
network interface).

The root `grpchan` package provides `grpchan.Channel` and `grpchan.ServiceRegistry`,
which are the core interfaces to implement when building a new transport.
Prior to grpc-go v1.32.0, it was necessary to use a
[proto plugin](https://github.com/fullstorydev/grpchan/blob/master/cmd/protoc-gen-grpchan/protoc-gen-grpchan.go)
that would generate additional hooks in generated gRPC code, allowing use of these
abstractions. But as of v1.32.0, there are now `grpc.ClientConnInterface` and
`grpc.ServiceRegistrar` interfaces that are identical to `grpchan`'s abstractions.
As long as you are using `protoc-gen-go-grpc` v1.0.1 or higher for code generation,
gRPC generated code works out-of-the-box with `grpchan`.
