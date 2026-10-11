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

The root `grpchan` package was created to provide its `Channel` and `ServiceRegistry`
interfaces; these were the core interfaces to implement when building a new transport.
The package also includes some helpers related to these interfaces as well as a simple
map-backed registry.

These two interfaces were needed prior to grpc-go v1.32.0, as was the use of a
[proto plugin](https://github.com/fullstorydev/grpchan/blob/master/cmd/protoc-gen-grpchan/protoc-gen-grpchan.go)
that would generate additional functions alongside generated gRPC code, allowing
the use of these abstractions. But as of v1.32.0 of the grpc-go module, there now
exist `grpc.ClientConnInterface` and `grpc.ServiceRegistrar`, which are what a new
transport must implement (`grpchan`'s interfaces are now deprecated aliases of these
two). As long as you are using `protoc-gen-go-grpc` v1.0.1 or newer for code
generation, gRPC generated code works out-of-the-box with `grpchan`. (Note that the
old `protoc-gen-go` plugin, provided by the `github.com/golang/protobuf` repo, could
also generate gRPC code, but its output does not work with `grpchan` without the
proto plugin mentioned above.)
