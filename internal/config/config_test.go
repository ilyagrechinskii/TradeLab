package config

import "testing"

func TestLoad(t *testing.T) {
	for _, tt := range []struct {
		name    string
		value   string
		want    string
		wantErr bool
	}{
		{name: "default", want: "8080"},
		{name: "custom", value: "9000", want: "9000"},
		{name: "minimum", value: "1", want: "1"},
		{name: "maximum", value: "65535", want: "65535"},
		{name: "text", value: "invalid", wantErr: true},
		{name: "zero", value: "0", wantErr: true},
		{name: "negative", value: "-1", wantErr: true},
		{name: "out of range", value: "65536", wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("PORT", tt.value)
			cfg, err := Load()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Load() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && cfg.Port != tt.want {
				t.Errorf("Port = %q, want %q", cfg.Port, tt.want)
			}
		})
	}
}
