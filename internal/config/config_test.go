package config

import "testing"

func TestLoad(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "")
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

func TestCORSOrigins(t *testing.T) {
	t.Setenv("PORT", "8080")
	for _, tt := range []struct {
		value   string
		count   int
		invalid bool
	}{
		{"", 0, false},
		{"http://localhost:5173", 1, false},
		{"https://frontend.example, http://localhost:3000", 2, false},
		{"*", 0, true}, {"null", 0, true}, {"file:///tmp/index.html", 0, true},
		{"https://example.com/path", 0, true}, {"https://example.com/", 0, true},
		{"https://user:password@example.com", 0, true}, {"https://*.example.com", 0, true},
		{"https://example.com?x=1", 0, true}, {"http://localhost:5173,", 0, true},
	} {
		t.Run(tt.value, func(t *testing.T) {
			t.Setenv("CORS_ALLOWED_ORIGINS", tt.value)
			cfg, err := Load()
			if (err != nil) != tt.invalid {
				t.Fatalf("Load error=%v invalid=%v", err, tt.invalid)
			}
			if !tt.invalid && len(cfg.AllowedOrigins) != tt.count {
				t.Errorf("origins=%v want count=%d", cfg.AllowedOrigins, tt.count)
			}
		})
	}
}
