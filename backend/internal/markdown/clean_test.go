package markdown

import "testing"

func TestWithoutEmbeddedDataImages(t *testing.T) {
	tests := map[string]struct {
		input string
		want  string
	}{
		"markdown image": {
			input: "# Title\n\n![Image](data:image/png;base64,iVBORw0KGgo=)\n\nSearchable text",
			want:  "# Title\n\nSearchable text",
		},
		"truncated markdown image": {
			input: "Before\n\n![Image](data:image/png;base64,iVBORw0KGgo=\nAfter",
			want:  "Before\n\nAfter",
		},
		"html image": {
			input: `Before <img alt="scan" src="data:image/png;base64,iVBORw0KGgo="> After`,
			want:  "Before  After",
		},
		"external image": {
			input: "![Diagram](https://example.com/diagram.png)",
			want:  "![Diagram](https://example.com/diagram.png)",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := WithoutEmbeddedDataImages(tt.input); got != tt.want {
				t.Fatalf("WithoutEmbeddedDataImages() = %q, want %q", got, tt.want)
			}
		})
	}
}
