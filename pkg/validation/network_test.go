package validation

import (
	"testing"
)

func TestValidateBrokerAddress(t *testing.T) {
	tests := []struct {
		name        string
		address     string
		wantErr     bool
		errContains string
	}{
		{
			name:    "valid address with hostname",
			address: "broker.example.com:9092",
			wantErr: false,
		},
		{
			name:    "valid address with IP",
			address: "192.168.1.1:9092",
			wantErr: false,
		},
		{
			name:    "valid address with localhost",
			address: "localhost:9092",
			wantErr: false,
		},
		{
			name:        "empty address",
			address:     "",
			wantErr:     true,
			errContains: "cannot be empty",
		},
		{
			name:        "missing port",
			address:     "broker.example.com",
			wantErr:     true,
			errContains: "must include port",
		},
		{
			name:        "invalid port",
			address:     "broker.example.com:99999",
			wantErr:     true,
			errContains: "must be between 1 and 65535",
		},
		{
			name:        "empty host",
			address:     ":9092",
			wantErr:     true,
			errContains: "host cannot be empty",
		},
		{
			name:        "invalid characters in host",
			address:     "broker;example.com:9092",
			wantErr:     true,
			errContains: "invalid characters",
		},
		{
			name:        "hostname too long",
			address:     "verylonghostnamethatexceedsthemaximumallowedlengthforahostnamewhichissupposedtobelessthan253charactersaccordingtotherfcspecificationandthisshouldcauseanerrorverylonghostnamethatexceedsthemaximumallowedlengthforahostnamewhichissupposedtobelessthan253characters:9092",
			wantErr:     true,
			errContains: "hostname too long",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBrokerAddress(tt.address)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateBrokerAddress() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.errContains != "" && !contains(err.Error(), tt.errContains) {
				t.Errorf("ValidateBrokerAddress() error = %v, want error containing %s", err, tt.errContains)
			}
		})
	}
}

func TestValidateMQTTBroker(t *testing.T) {
	tests := []struct {
		name        string
		host        string
		port        int
		wantErr     bool
		errContains string
	}{
		{
			name:    "valid MQTT broker",
			host:    "mqtt.example.com",
			port:    1883,
			wantErr: false,
		},
		{
			name:    "valid MQTT broker with IP",
			host:    "192.168.1.1",
			port:    1883,
			wantErr: false,
		},
		{
			name:        "empty host",
			host:        "",
			port:        1883,
			wantErr:     true,
			errContains: "cannot be empty",
		},
		{
			name:        "invalid port low",
			host:        "mqtt.example.com",
			port:        0,
			wantErr:     true,
			errContains: "must be between 1 and 65535",
		},
		{
			name:        "invalid port high",
			host:        "mqtt.example.com",
			port:        70000,
			wantErr:     true,
			errContains: "must be between 1 and 65535",
		},
		{
			name:        "invalid characters in host",
			host:        "mqtt;example.com",
			port:        1883,
			wantErr:     true,
			errContains: "invalid characters",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMQTTBroker(tt.host, tt.port)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateMQTTBroker() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.errContains != "" && !contains(err.Error(), tt.errContains) {
				t.Errorf("ValidateMQTTBroker() error = %v, want error containing %s", err, tt.errContains)
			}
		})
	}
}