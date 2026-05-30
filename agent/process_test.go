package agent

import (
	"testing"

	boxenprofile "github.com/carlmontanari/boxen/profile"
)

func TestPromptCallbackName(t *testing.T) {
	cases := []struct {
		name     string
		idx      int
		prompt   boxenprofile.Prompt
		expected string
	}{
		{
			name: "named prompt",
			idx:  3,
			prompt: boxenprofile.Prompt{
				Name: "login prompt",
			},
			expected: `prompts step name "login prompt", idx 3`,
		},
		{
			name:     "unnamed prompt",
			idx:      4,
			expected: "prompts step idx 4",
		},
		{
			name: "blank prompt name",
			idx:  5,
			prompt: boxenprofile.Prompt{
				Name: " \t\n ",
			},
			expected: "prompts step idx 5",
		},
	}

	for _, testCase := range cases {
		t.Run(
			testCase.name,
			func(t *testing.T) {
				actual := promptCallbackName(testCase.idx, testCase.prompt)
				if actual != testCase.expected {
					t.Fatalf("prompt callback name incorrect, got %q, want %q", actual, testCase.expected)
				}
			},
		)
	}
}
