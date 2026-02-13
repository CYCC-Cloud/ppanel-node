package grpcclient

import (
	"context"
	"encoding/json"
	"time"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// debugLoggingInterceptor creates a unary interceptor that logs gRPC calls in DEBUG mode
func debugLoggingInterceptor() grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply interface{},
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		start := time.Now()

		// Log request
		if log.IsLevelEnabled(log.DebugLevel) {
			requestJSON := marshalToJSON(req)
			log.WithFields(log.Fields{
				"method":  method,
				"request": requestJSON,
			}).Debug("gRPC request sent")
		}

		// Invoke the actual RPC
		err := invoker(ctx, method, req, reply, cc, opts...)

		// Log response
		if log.IsLevelEnabled(log.DebugLevel) {
			duration := time.Since(start)
			fields := log.Fields{
				"method":      method,
				"duration_ms": duration.Milliseconds(),
			}

			if err != nil {
				fields["error"] = err.Error()
				log.WithFields(fields).Debug("gRPC call failed")
			} else {
				responseJSON := marshalToJSON(reply)
				fields["response"] = responseJSON
				log.WithFields(fields).Debug("gRPC call completed")
			}
		}

		return err
	}
}

// marshalToJSON converts a protobuf message to JSON string
func marshalToJSON(v interface{}) string {
	if v == nil {
		return "null"
	}

	// Try to marshal as protobuf message first
	if msg, ok := v.(proto.Message); ok {
		marshaler := protojson.MarshalOptions{
			EmitUnpopulated: false,
			UseProtoNames:   true,
		}
		if data, err := marshaler.Marshal(msg); err == nil {
			return string(data)
		}
	}

	// Fallback to standard JSON
	if data, err := json.Marshal(v); err == nil {
		return string(data)
	}

	// If all else fails, return string representation
	return "<unable to marshal>"
}
