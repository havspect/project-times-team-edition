package config

import "testing"

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		env     map[string]string
		want    Config
		wantErr bool
	}{
		{
			name: "defaults",
			want: Config{Addr: ":8080", DB: "./tstamps.db"},
		},
		{
			name: "env only",
			env:  map[string]string{"TSTAMPS_ADDR": "127.0.0.1:9000", "TSTAMPS_DB": "/srv/tstamps.db"},
			want: Config{Addr: "127.0.0.1:9000", DB: "/srv/tstamps.db"},
		},
		{
			name: "flags only",
			args: []string{"--addr", ":7000", "--db", "flags.db"},
			want: Config{Addr: ":7000", DB: "flags.db"},
		},
		{
			name: "flags beat env",
			args: []string{"--db", "flags.db"},
			env:  map[string]string{"TSTAMPS_ADDR": "127.0.0.1:9000", "TSTAMPS_DB": "/srv/tstamps.db"},
			want: Config{Addr: "127.0.0.1:9000", DB: "flags.db"},
		},
		{
			name:    "unknown flag",
			args:    []string{"--nope"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Load(tt.args, func(key string) string { return tt.env[key] })
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Load(%q) = %+v, want error", tt.args, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load(%q): %v", tt.args, err)
			}
			if got != tt.want {
				t.Errorf("Load(%q) = %+v, want %+v", tt.args, got, tt.want)
			}
		})
	}
}
