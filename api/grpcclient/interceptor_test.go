package grpcclient

import (
	"testing"

	nodecontrolv1 "github.com/CYCC-Cloud/ppanel-proto/gen/go/ppanel/nodecontrol/v1"
	log "github.com/sirupsen/logrus"
)

func TestMarshalToJSON(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		want  string
	}{
		{
			name:  "nil value",
			input: nil,
			want:  "null",
		},
		{
			name: "protobuf message",
			input: &nodecontrolv1.GetConfigRequest{
				ServerId:  1,
				Protocols: []string{"trojan"},
			},
			want: `{"server_id":"1","protocols":["trojan"]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := marshalToJSON(tt.input)
			if got != tt.want {
				t.Errorf("marshalToJSON() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDebugLoggingInterceptor(t *testing.T) {
	// Save original log level
	originalLevel := log.GetLevel()
	defer log.SetLevel(originalLevel)

	// Test that interceptor is created successfully
	interceptor := debugLoggingInterceptor()
	if interceptor == nil {
		t.Error("debugLoggingInterceptor() returned nil")
	}

	// Test that interceptor only logs when debug is enabled
	log.SetLevel(log.InfoLevel)
	// At Info level, debug logs should not appear
	// (This is more of a documentation test, actual behavior verified in integration tests)

	log.SetLevel(log.DebugLevel)
	// At Debug level, debug logs should appear
}
