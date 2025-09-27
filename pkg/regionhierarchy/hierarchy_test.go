package regionhierarchy

import "testing"

func TestParseRegionID(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		level    Level
		province string
		city     string
		district string
		sub      string
		wantErr  bool
	}{
		{"province", "11", LevelProvince, "11", "", "", "", false},
		{"city", "11.05", LevelCity, "11", "11.05", "", "", false},
		{"district", "11.05.03", LevelDistrict, "11", "11.05", "11.05.03", "", false},
		{"subdistrict", "11.05.03.2001", LevelSubdistrict, "11", "11.05", "11.05.03", "11.05.03.2001", false},
		{"invalid pattern", "1105", LevelUnknown, "", "", "", "", true},
		{"empty", "", LevelUnknown, "", "", "", "", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			segs, err := ParseRegionID(tc.id)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if segs.Level() != tc.level {
				t.Fatalf("expected level %v, got %v", tc.level, segs.Level())
			}
			if segs.Province != tc.province {
				t.Fatalf("expected province %q, got %q", tc.province, segs.Province)
			}
			if segs.City != tc.city {
				t.Fatalf("expected city %q, got %q", tc.city, segs.City)
			}
			if segs.District != tc.district {
				t.Fatalf("expected district %q, got %q", tc.district, segs.District)
			}
			if segs.Subdistrict != tc.sub {
				t.Fatalf("expected subdistrict %q, got %q", tc.sub, segs.Subdistrict)
			}
		})
	}
}

func TestSharePrefix(t *testing.T) {
	cases := []struct {
		parent string
		child  string
		want   bool
	}{
		{"11", "11.05", true},
		{"11.05", "11.05.03.2001", true},
		{"11.05", "12.01.02", false},
		{"11.05.01", "11.05", false},
		{"11.05", "", false},
		{"", "11.05.03", false},
		{"11.05", "11.0501", false},
	}
	for _, tc := range cases {
		if got := SharePrefix(tc.parent, tc.child); got != tc.want {
			t.Fatalf("SharePrefix(%q, %q) = %v, want %v", tc.parent, tc.child, got, tc.want)
		}
	}
}

func TestIsConsistentHierarchy(t *testing.T) {
	if !IsConsistentHierarchy("11", "11.05", "11.05.03.2001") {
		t.Fatalf("expected hierarchy to be consistent")
	}
	if IsConsistentHierarchy("11.05", "12.01") {
		t.Fatalf("expected mismatch")
	}
	if !IsConsistentHierarchy("11.05.03", "11.05.03.2001") {
		t.Fatalf("expected consistent partial chain")
	}
}

func TestCodeAtLevel(t *testing.T) {
	segs, err := ParseRegionID("11.02.04.3001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tests := []struct {
		lvl  Level
		want string
	}{
		{LevelProvince, "11"},
		{LevelCity, "11.02"},
		{LevelDistrict, "11.02.04"},
		{LevelSubdistrict, "11.02.04.3001"},
	}
	for _, tc := range tests {
		if got := segs.CodeForLevel(tc.lvl); got != tc.want {
			t.Fatalf("expected %s for level %v, got %s", tc.want, tc.lvl, got)
		}
		got, err := CodeAtLevel("11.02.04.3001", tc.lvl)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != tc.want {
			t.Fatalf("CodeAtLevel mismatch: want %s, got %s", tc.want, got)
		}
	}
}
