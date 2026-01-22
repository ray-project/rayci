package wanda

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseEnvFile(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    map[string]string
		wantErr string
	}{
		{
			name:    "simple key-value",
			content: "FOO=bar",
			want:    map[string]string{"FOO": "bar"},
		},
		{
			name: "multiple key-values",
			content: strings.Join([]string{
				"FOO=bar",
				"BAZ=qux",
			}, "\n"),
			want: map[string]string{"FOO": "bar", "BAZ": "qux"},
		},
		{
			name: "comments ignored",
			content: strings.Join([]string{
				"# comment",
				"FOO=bar",
			}, "\n"),
			want: map[string]string{"FOO": "bar"},
		},
		{
			name: "hash in value preserved",
			content: strings.Join([]string{
				"FOO=bar # not a comment",
				"BAZ=qux",
			}, "\n"),
			want: map[string]string{"FOO": "bar # not a comment", "BAZ": "qux"},
		},
		{
			name: "blank lines ignored",
			content: strings.Join([]string{
				"FOO=bar",
				"",
				"BAZ=qux",
			}, "\n"),
			want: map[string]string{"FOO": "bar", "BAZ": "qux"},
		},
		{
			name:    "dollar sign preserved",
			content: "FOO=$UNDEFINED",
			want:    map[string]string{"FOO": "$UNDEFINED"},
		},
		{
			name:    "double dollar preserved",
			content: "PRICE=$$100",
			want:    map[string]string{"PRICE": "$$100"},
		},
		{
			name:    "value with spaces trimmed",
			content: "FOO = bar ",
			want:    map[string]string{"FOO": "bar"},
		},
		{
			name:    "empty value",
			content: "FOO=",
			want:    map[string]string{"FOO": ""},
		},
		{
			name:    "url value",
			content: "URL=postgresql://user:pass@host/db",
			want:    map[string]string{"URL": "postgresql://user:pass@host/db"},
		},
		{
			name:    "url with anchor hash",
			content: "URL=https://example.com/page#anchor",
			want:    map[string]string{"URL": "https://example.com/page#anchor"},
		},
		{
			name:    "color code with hash",
			content: "COLOR=#FF0000",
			want:    map[string]string{"COLOR": "#FF0000"},
		},
		{
			name:    "password with hash",
			content: "PASSWORD=secret#123",
			want:    map[string]string{"PASSWORD": "secret#123"},
		},
		{
			name:    "value with equals sign",
			content: "FOO=a=b=c",
			want:    map[string]string{"FOO": "a=b=c"},
		},
		{
			name:    "missing equals",
			content: "FOOBAR",
			wantErr: "expected KEY=value",
		},
		{
			name:    "empty key",
			content: "=bar",
			wantErr: "empty key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, ".env")
			if err := os.WriteFile(path, []byte(tt.content), 0644); err != nil {
				t.Fatalf("write temp file: %v", err)
			}

			got, err := ParseEnvFile(path)

			if tt.wantErr != "" {
				if err == nil {
					t.Errorf("error = nil, want containing %q", tt.wantErr)
					return
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if !reflect.DeepEqual(tt.want, got) {
				t.Errorf("ParseEnvFile() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseEnvFileMissing(t *testing.T) {
	_, err := ParseEnvFile("/nonexistent/path/.env")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestParseEnvFileWithExpandVar(t *testing.T) {
	tests := []struct {
		name    string
		content string
		input   string
		want    string
	}{
		{
			name:    "simple expansion",
			content: "FOO=bar",
			input:   "hello $FOO",
			want:    "hello bar",
		},
		{
			name: "multiple vars",
			content: strings.Join([]string{
				"HOST=localhost",
				"PORT=5432",
			}, "\n"),
			input: "postgresql://$HOST:$PORT/db",
			want:  "postgresql://localhost:5432/db",
		},
		{
			name:    "undefined var preserved",
			content: "FOO=bar",
			input:   "$UNDEFINED stays",
			want:    "$UNDEFINED stays",
		},
		{
			name: "envfile overrides fallback",
			content: strings.Join([]string{
				"VERSION=2.0",
			}, "\n"),
			input: "v$VERSION",
			want:  "v2.0",
		},
		{
			name:    "value with dollar sign",
			content: "PRICE=$100",
			input:   "cost: $PRICE",
			want:    "cost: $100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, ".env")
			if err := os.WriteFile(path, []byte(tt.content), 0644); err != nil {
				t.Fatalf("write temp file: %v", err)
			}

			envMap, err := ParseEnvFile(path)
			if err != nil {
				t.Fatalf("ParseEnvFile: %v", err)
			}

			lookup := func(key string) (string, bool) {
				if v, ok := envMap[key]; ok {
					return v, true
				}
				return "", false
			}

			got := expandVar(tt.input, lookup)
			if got != tt.want {
				t.Errorf("expandVar(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
