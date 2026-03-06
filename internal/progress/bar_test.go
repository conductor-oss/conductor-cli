package progress

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{26214400, "25.0 MB"},
		{1073741824, "1.0 GB"},
	}

	for _, tt := range tests {
		result := FormatBytes(tt.input)
		if result != tt.expected {
			t.Errorf("FormatBytes(%d) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestProgressBarKnownTotal(t *testing.T) {
	var buf bytes.Buffer
	bar := NewProgressBar(100, "Test")
	bar.w = &buf
	bar.lastRender = bar.lastRender // ensure zero time triggers render

	bar.Add(50)
	bar.Finish()

	output := buf.String()
	if !strings.Contains(output, "Test") {
		t.Errorf("output should contain description, got: %q", output)
	}
	if !strings.Contains(output, "100%") {
		// Finish should render final state
		if !strings.Contains(output, "50") {
			t.Errorf("output should contain progress info, got: %q", output)
		}
	}
}

func TestProgressBarUnknownTotal(t *testing.T) {
	var buf bytes.Buffer
	bar := NewProgressBar(0, "Test")
	bar.w = &buf

	bar.Add(1048576)
	bar.Finish()

	output := buf.String()
	if !strings.Contains(output, "downloaded") {
		t.Errorf("unknown total should show 'downloaded', got: %q", output)
	}
}

func TestReaderWrapsRead(t *testing.T) {
	data := "hello world, this is test data for the progress reader"
	src := strings.NewReader(data)

	var buf bytes.Buffer
	pr, bar := NewReader(src, int64(len(data)), "Reading")
	bar.w = &buf // redirect output

	result, err := io.ReadAll(pr)
	bar.Finish()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(result) != data {
		t.Errorf("data mismatch: got %q, want %q", string(result), data)
	}
}

func TestReaderZeroTotal(t *testing.T) {
	data := "some data"
	src := strings.NewReader(data)

	var buf bytes.Buffer
	pr, bar := NewReader(src, 0, "Download")
	bar.w = &buf

	result, err := io.ReadAll(pr)
	bar.Finish()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(result) != data {
		t.Errorf("data mismatch: got %q, want %q", string(result), data)
	}
	if !strings.Contains(buf.String(), "downloaded") {
		t.Errorf("zero total should show 'downloaded', got: %q", buf.String())
	}
}
