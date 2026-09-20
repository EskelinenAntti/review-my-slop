package ui

import "testing"

func TestKeyDecoderRecognizesCommandsAndSequences(t *testing.T) {
	decoder := keyDecoder{}
	tests := []struct {
		keys []string
		want command
	}{
		{keys: []string{"j"}, want: commandMoveForward},
		{keys: []string{"g", "g"}, want: commandFirstLine},
		{keys: []string{"z", "t"}, want: commandAlignTop},
		{keys: []string{"]", "f"}, want: commandJumpFileForward},
		{keys: []string{"ctrl+w", "l"}, want: commandSwitchRight},
	}
	for _, test := range tests {
		t.Run(test.keys[len(test.keys)-1], func(t *testing.T) {
			decoder = keyDecoder{}
			var got command
			for _, key := range test.keys {
				got = decoder.Decode(key)
			}
			if got != test.want {
				t.Fatalf("Decode(%q) = %v, want %v", test.keys, got, test.want)
			}
		})
	}
}

func TestKeyDecoderConsumesInvalidSequence(t *testing.T) {
	decoder := keyDecoder{}
	if got := decoder.Decode("]"); got != commandNone {
		t.Fatalf("prefix = %v", got)
	}
	if got := decoder.Decode("j"); got != commandNone {
		t.Fatalf("invalid sequence = %v", got)
	}
	if decoder.pending != prefixNone {
		t.Fatalf("pending = %v", decoder.pending)
	}
}
