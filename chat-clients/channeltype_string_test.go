package main

import "testing"

func TestChannelType_String(t *testing.T) {
	tests := []struct {
		name string
		i    ChannelType
		want string
	}{
		{
			name: "restring_DM",
			i:    DM,
			want: "DM",
		},
		{
			name: "restring_Standard",
			i:    Standard,
			want: "Standard",
		},
		{
			name: "MultiDM",
			i:    MultiDM,
			want: "MultiDM",
		},
		{
			name: "Unknown",
			i:    Unknown,
			want: "Unknown",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.i.String(); got != tt.want {
				t.Errorf("ChannelType.String() = %v, want %v", got, tt.want)
			}
		})
	}
}
