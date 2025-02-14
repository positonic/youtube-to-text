package transcription

import (
	"testing"
	"time"
)

func TestParseVTT(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    int
		wantErr bool
	}{
		{
			name: "basic vtt",
			content: `WEBVTT

00:00:01.000 --> 00:00:04.000
Hello, this is the first subtitle

00:00:04.100 --> 00:00:08.000
This is the second subtitle`,
			want:    2,
			wantErr: false,
		},
		{
			name: "multi-line subtitle",
			content: `WEBVTT

00:00:01.000 --> 00:00:04.000
Hello, this is
a multi-line subtitle

00:00:04.100 --> 00:00:08.000
Second entry`,
			want:    2,
			wantErr: false,
		},
		{
			name:    "invalid header",
			content: "NOT A VTT FILE",
			want:    0,
			wantErr: true,
		},
		{
			name: "empty lines between entries",
			content: `WEBVTT


00:00:01.000 --> 00:00:04.000
First entry


00:00:04.100 --> 00:00:08.000
Second entry`,
			want:    2,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries, err := parseVTT(tt.content)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseVTT() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(entries) != tt.want {
				t.Errorf("parseVTT() got %d entries, want %d", len(entries), tt.want)
			}
		})
	}
}

func TestParseVTTTimestamp(t *testing.T) {
	tests := []struct {
		name      string
		timestamp string
		want      time.Duration
		wantErr   bool
	}{
		{
			name:      "zero timestamp",
			timestamp: "00:00:00.000",
			want:      0,
			wantErr:   false,
		},
		{
			name:      "one second",
			timestamp: "00:00:01.000",
			want:      time.Second,
			wantErr:   false,
		},
		{
			name:      "with hours",
			timestamp: "01:00:00.000",
			want:      time.Hour,
			wantErr:   false,
		},
		{
			name:      "with milliseconds",
			timestamp: "00:00:00.500",
			want:      500 * time.Millisecond,
			wantErr:   false,
		},
		{
			name:      "complex time",
			timestamp: "01:23:45.678",
			want:      1*time.Hour + 23*time.Minute + 45*time.Second + 678*time.Millisecond,
			wantErr:   false,
		},
		{
			name:      "invalid format",
			timestamp: "1:23:45.678",
			wantErr:   true,
		},
		{
			name:      "missing milliseconds",
			timestamp: "00:00:01",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseVTTTimestamp(tt.timestamp)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseVTTTimestamp() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("parseVTTTimestamp() = %v, want %v", got, tt.want)
			}
		})
	}
} 