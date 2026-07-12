package list

import "testing"

func TestResolveBoardID(t *testing.T) {
	tests := []struct {
		name         string
		configuredID int
		override     int
		want         int
	}{
		{name: "no override uses configured board", configuredID: 10, override: 0, want: 10},
		{name: "override replaces configured board", configuredID: 10, override: 20, want: 20},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveBoardID(tc.configuredID, tc.override)
			if got != tc.want {
				t.Errorf("resolveBoardID(%d, %d) = %d, want %d", tc.configuredID, tc.override, got, tc.want)
			}
		})
	}
}

func TestResolveBoardName(t *testing.T) {
	tests := []struct {
		name           string
		configuredID   int
		configuredName string
		boardID        int
		want           string
	}{
		{
			name:           "configured board keeps its name",
			configuredID:   10,
			configuredName: "My Board",
			boardID:        10,
			want:           "My Board",
		},
		{
			name:           "overridden board shows its ID instead of the configured name",
			configuredID:   10,
			configuredName: "My Board",
			boardID:        20,
			want:           "20",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveBoardName(tc.configuredID, tc.configuredName, tc.boardID)
			if got != tc.want {
				t.Errorf("resolveBoardName(%d, %q, %d) = %q, want %q", tc.configuredID, tc.configuredName, tc.boardID, got, tc.want)
			}
		})
	}
}
