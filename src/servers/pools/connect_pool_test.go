package pools

import (
	"fmt"
	"testing"
)

func TestReplaceURLAuthPlaceholders(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		auth map[string]string
		want string
	}{
		{
			name: "replace multiple params",
			raw:  "http://localhost:10001/api/mcp?$(key_a)=$(VALUE)&$(key_b)=$(VALUE)",
			auth: map[string]string{"key_a": "v1", "key_b": "v2"},
			want: "http://localhost:10001/api/mcp?key_a=v1&key_b=v2",
		},
		{
			name: "missing key keeps original",
			raw:  "http://localhost:10001/api/mcp?$(key1)=$(VALUE)&$(key2)=$(VALUE)",
			auth: map[string]string{"key1": "v1"},
			want: "http://localhost:10001/api/mcp?key1=v1&$(key2)=$(VALUE)",
		},
		{
			name: "value not placeholder does not replace",
			raw:  "http://localhost:10001/api/mcp?$(key1)=123&$(key2)=$(VALUE)",
			auth: map[string]string{"key1": "v1", "key2": "v2"},
			want: "http://localhost:10001/api/mcp?$(key1)=123&key2=v2",
		},
		{
			name: "encoded placeholders can be replaced",
			raw:  "http://localhost:10001/api/mcp?%24%28key1%29=%24%28VALUE%29&%24%28key2%29=%24%28VALUE%29",
			auth: map[string]string{"key1": "v1", "key2": "v2"},
			want: "http://localhost:10001/api/mcp?key1=v1&key2=v2",
		},
		{
			name: "no placeholders -> unchanged",
			raw:  "http://localhost:10001/api/mcp?key1=v1&key2=v2",
			auth: map[string]string{"key1": "x"},
			want: "http://localhost:10001/api/mcp?key1=v1&key2=v2",
		},
		{
			name: "empty auth -> unchanged",
			raw:  "http://localhost:10001/api/mcp?$(key1)=$(VALUE)",
			auth: map[string]string{},
			want: "http://localhost:10001/api/mcp?$(key1)=$(VALUE)",
		},
		{
			name: "invalid url -> unchanged",
			raw:  "http://[::1]:namedport?$(key1)=$(VALUE)",
			auth: map[string]string{"key1": "v1"},
			want: "http://[::1]:namedport?$(key1)=$(VALUE)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := replaceURLAuthPlaceholders(tt.raw, tt.auth)
			if got != tt.want {
				t.Fatalf("replaceURLAuthPlaceholders() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestReplaceHeaderAuthPlaceholders(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string]string
		auth    map[string]string
		want    map[string]string
	}{
		{
			name:    "placeholder key and VALUE -> real key and auth value",
			headers: map[string]string{"$(key_a)": "$(VALUE)"},
			auth:    map[string]string{"key_a": "v1"},
			want:    map[string]string{"key_a": "v1"},
		},
		{
			name:    "fixed key with Bearer VALUE template",
			headers: map[string]string{"Authorization": "Bearer $(VALUE)"},
			auth:    map[string]string{"Authorization": "token123"},
			want:    map[string]string{"Authorization": "Bearer token123"},
		},
		{
			name:    "missing auth keeps original",
			headers: map[string]string{"$(key_a)": "$(VALUE)"},
			auth:    map[string]string{"other": "x"},
			want:    map[string]string{"$(key_a)": "$(VALUE)"},
		},
		{
			name:    "mix of normal header and placeholder header",
			headers: map[string]string{"Content-Type": "application/json", "$(X-Api-Key)": "$(VALUE)"},
			auth:    map[string]string{"X-Api-Key": "k123"},
			want:    map[string]string{"Content-Type": "application/json", "X-Api-Key": "k123"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := replaceHeaderAuthPlaceholders(tt.headers, tt.auth)
			if len(got) != len(tt.want) {
				t.Fatalf("len(got)=%d, len(want)=%d, got=%v, want=%v", len(got), len(tt.want), got, tt.want)
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Fatalf("got[%q]=%q, want %q (got=%v)", k, got[k], v, got)
				}
			}
		})
	}
}

func TestReplaceEnvAuthPlaceholders(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]interface{}
		auth map[string]string
		want map[string]interface{}
	}{
		{
			name: "placeholder key and VALUE -> real key and auth value",
			env:  map[string]interface{}{"$(API_KEY)": "$(VALUE)"},
			auth: map[string]string{"API_KEY": "k123"},
			want: map[string]interface{}{"API_KEY": "k123"},
		},
		{
			name: "fixed key with VALUE template",
			env:  map[string]interface{}{"AUTH": "Bearer $(VALUE)"},
			auth: map[string]string{"AUTH": "token123"},
			want: map[string]interface{}{"AUTH": "Bearer token123"},
		},
		{
			name: "missing auth keeps original",
			env:  map[string]interface{}{"$(API_KEY)": "$(VALUE)"},
			auth: map[string]string{"OTHER": "x"},
			want: map[string]interface{}{"$(API_KEY)": "$(VALUE)"},
		},
		{
			name: "mix of normal env and placeholder env keeps normal value",
			env:  map[string]interface{}{"PORT": 8080, "$(API_KEY)": "$(VALUE)"},
			auth: map[string]string{"API_KEY": "k123"},
			want: map[string]interface{}{"PORT": 8080, "API_KEY": "k123"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := replaceEnvAuthPlaceholders(tt.env, tt.auth)
			if len(got) != len(tt.want) {
				t.Fatalf("len(got)=%d, len(want)=%d, got=%v, want=%v", len(got), len(tt.want), got, tt.want)
			}
			for k, wv := range tt.want {
				gv, ok := got[k]
				if !ok {
					t.Fatalf("missing key %q in got=%v", k, got)
				}
				if fmt.Sprint(gv) != fmt.Sprint(wv) {
					t.Fatalf("got[%q]=%q, want %q (got=%v)", k, fmt.Sprint(gv), fmt.Sprint(wv), got)
				}
			}
		})
	}
}

func TestHeadersContainAuthPlaceholders(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string]string
		want    bool
	}{
		{
			name:    "nil/empty -> false",
			headers: nil,
			want:    false,
		},
		{
			name:    "key contains placeholder -> true",
			headers: map[string]string{"$(X-Api-Key)": "abc"},
			want:    true,
		},
		{
			name:    "value contains VALUE placeholder -> true",
			headers: map[string]string{"Authorization": "Bearer $(VALUE)"},
			want:    true,
		},
		{
			name:    "encoded placeholder -> true",
			headers: map[string]string{"X": "%24%28VALUE%29"},
			want:    true,
		},
		{
			name:    "no placeholders -> false",
			headers: map[string]string{"Content-Type": "application/json"},
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := headersContainAuthPlaceholders(tt.headers)
			if got != tt.want {
				t.Fatalf("headersContainAuthPlaceholders()=%v, want %v (headers=%v)", got, tt.want, tt.headers)
			}
		})
	}
}

func TestEnvContainAuthPlaceholders(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]interface{}
		want bool
	}{
		{
			name: "nil/empty -> false",
			env:  nil,
			want: false,
		},
		{
			name: "key contains placeholder -> true",
			env:  map[string]interface{}{"$(API_KEY)": "abc"},
			want: true,
		},
		{
			name: "value contains VALUE placeholder -> true",
			env:  map[string]interface{}{"AUTH": "Bearer $(VALUE)"},
			want: true,
		},
		{
			name: "encoded placeholder -> true",
			env:  map[string]interface{}{"X": "%24%28VALUE%29"},
			want: true,
		},
		{
			name: "no placeholders -> false",
			env:  map[string]interface{}{"PORT": 8080},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := envContainAuthPlaceholders(tt.env)
			if got != tt.want {
				t.Fatalf("envContainAuthPlaceholders()=%v, want %v (env=%v)", got, tt.want, tt.env)
			}
		})
	}
}
