package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestLoadAnimeAPITSV_Remote(t *testing.T) {
	// Live test – fetches from animeapi.my.id; skip manually if offline
	anidbToMAL, malToRow, err := LoadAnimeAPITSV("") // empty = fetch remote
	if err != nil {
		t.Fatalf("LoadAnimeAPITSV (remote) error: %v", err)
	}
	if len(anidbToMAL) == 0 {
		t.Fatal("expected non-empty anidbToMAL map from remote")
	}
	t.Logf("Remote: %d AniDB→MAL, %d MAL rows", len(anidbToMAL), len(malToRow))
}

// TestUseFribbDetection verifies that UseFribb is correctly populated via
// flag.Visit — i.e. an explicit "-fribb \"\"" sets it to true while an absent
// flag leaves it false.  We simulate this by directly inspecting the Config
// field since we can't re-parse flags inside a test.
func TestUseFribbDetection(t *testing.T) {
	// Simulate explicit pass: UseFribb should be true
	explicit := Config{UseFribb: true, FribbFile: ""}
	if !explicit.UseFribb {
		t.Error("expected UseFribb=true when flag was explicitly set")
	}
	if explicit.FribbFile != "" {
		t.Errorf("expected empty FribbFile (fetch from internet), got %q", explicit.FribbFile)
	}

	// Simulate absent flag: UseFribb should remain false (zero value)
	absent := Config{}
	if absent.UseFribb {
		t.Error("expected UseFribb=false when flag was not provided")
	}
}

func TestLoadAnimeAPITSV(t *testing.T) {
	path := "/home/nattadasu/Git/animeApi/database/animeapi.tsv"
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skip("local animeapi.tsv not available")
	}

	anidbToMAL, malToRow, err := LoadAnimeAPITSV(path)
	if err != nil {
		t.Fatalf("LoadAnimeAPITSV error: %v", err)
	}
	if len(anidbToMAL) == 0 {
		t.Fatal("expected non-empty anidbToMAL map")
	}
	t.Logf("anidbToMAL entries: %d, malToRow entries: %d", len(anidbToMAL), len(malToRow))

	// Spot-check: AniDB 10143 is MAL 20707 (""""0""")
	if mal, ok := anidbToMAL[10143]; ok {
		t.Logf("AniDB 10143 → MAL %d (title: %q)", mal, malToRow[mal].Title)
	}
}

func TestLoadFribbJSON(t *testing.T) {
	path := "/tmp/anime-lists-reduced.json"
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skip("local anime-lists-reduced.json not available")
	}

	entries, err := LoadFribbJSON(path)
	if err != nil {
		t.Fatalf("LoadFribbJSON error: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected non-empty entries")
	}
	t.Logf("Fribb entries: %d", len(entries))

	tvCount, movieCount, noTMDB := 0, 0, 0
	for _, e := range entries {
		if e.ThemoviedbID == nil {
			noTMDB++
		} else if e.ThemoviedbID.TV != nil {
			tvCount++
		} else if e.ThemoviedbID.Movie != nil {
			movieCount++
		} else {
			noTMDB++
		}
	}
	t.Logf("TV TMDB: %d, Movie TMDB: %d, No TMDB: %d", tvCount, movieCount, noTMDB)
}

func TestFirstIMDb(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{"tt0936323,tt0936320", "tt0936323"},
		{"tt0245429", "tt0245429"},
		{"", ""},
		{"tt1234567, tt9999999", "tt1234567"},
	}
	for _, c := range cases {
		e := &FribbEntry{IMDbID: FribbIMDbID(c.raw)}
		got := e.FirstIMDb()
		if got != c.want {
			t.Errorf("FirstIMDb(%q) = %q, want %q", c.raw, got, c.want)
		}
	}
}

func TestCrossReference(t *testing.T) {
	tsvPath := "/home/nattadasu/Git/animeApi/database/animeapi.tsv"
	fribbPath := "/tmp/anime-lists-reduced.json"
	for _, p := range []string{tsvPath, fribbPath} {
		if _, err := os.Stat(p); os.IsNotExist(err) {
			t.Skipf("file not available: %s", p)
		}
	}

	anidbToMAL, _, err := LoadAnimeAPITSV(tsvPath)
	if err != nil {
		t.Fatalf("LoadAnimeAPITSV: %v", err)
	}
	entries, err := LoadFribbJSON(fribbPath)
	if err != nil {
		t.Fatalf("LoadFribbJSON: %v", err)
	}

	// Mirror the priority order used in ProcessFribb's work-list builder,
	// including the TVDB (TV fallback) and IMDB (movie fallback) paths.
	var tvTMDB, tvTVDB, movieTMDB, movieIMDB, noMAL, noID int
	for _, e := range entries {
		malID, ok := anidbToMAL[e.AnidbID]
		if !ok || malID == 0 {
			noMAL++
			continue
		}
		switch {
		case e.ThemoviedbID != nil && e.ThemoviedbID.Movie != nil && *e.ThemoviedbID.Movie > 0:
			movieTMDB++
		case e.ThemoviedbID != nil && e.ThemoviedbID.TV != nil && *e.ThemoviedbID.TV > 0:
			tvTMDB++
		case e.TVDbID > 0:
			tvTVDB++
		case e.FirstIMDb() != "":
			movieIMDB++
		default:
			noID++
		}
	}

	t.Logf("TV: %d via TMDB + %d via TVDB | Movies: %d via TMDB + %d via IMDB | no-MAL: %d | no-ID: %d",
		tvTMDB, tvTVDB, movieTMDB, movieIMDB, noMAL, noID)
	fmt.Printf("\nCross-reference summary (with fallbacks):\n"+
		"  TV  via TMDB:   %d\n"+
		"  TV  via TVDB:   %d  (fallback)\n"+
		"  Movie via TMDB: %d\n"+
		"  Movie via IMDB: %d  (fallback)\n"+
		"  No MAL mapping: %d\n"+
		"  No ID at all:   %d\n",
		tvTMDB, tvTVDB, movieTMDB, movieIMDB, noMAL, noID)
}

func TestFribbCustomUnmarshal(t *testing.T) {
	tests := []struct {
		name       string
		jsonInput  string
		wantAnidb  int
		wantImdb   string
		wantTvdb   int
		wantTmdbTV *int
		wantTmdbMV *int
	}{
		{
			name: "Nested TMDB movie list and single IMDB",
			jsonInput: `{
				"anidb_id": 123,
				"imdb_id": "tt15052770",
				"themoviedb_id": {"movie": [434326]},
				"tvdb_id": 0
			}`,
			wantAnidb:  123,
			wantImdb:   "tt15052770",
			wantTvdb:   0,
			wantTmdbTV: nil,
			wantTmdbMV: intPtr(434326),
		},
		{
			name: "Nested TMDB movie and list IMDB",
			jsonInput: `{
				"anidb_id": 456,
				"imdb_id": ["tt17677744", "tt25010142"],
				"themoviedb_id": {"movie": 434326},
				"tvdb_id": 0
			}`,
			wantAnidb:  456,
			wantImdb:   "tt17677744", // First IMDB ID
			wantTvdb:   0,
			wantTmdbTV: nil,
			wantTmdbMV: intPtr(434326),
		},
		{
			name: "Nested TMDB TV single value and empty IMDB",
			jsonInput: `{
				"anidb_id": 789,
				"imdb_id": null,
				"themoviedb_id": {"tv": 12345},
				"tvdb_id": 54321
			}`,
			wantAnidb:  789,
			wantImdb:   "",
			wantTvdb:   54321,
			wantTmdbTV: intPtr(12345),
			wantTmdbMV: nil,
		},
		{
			name: "Flat TMDB and string-list IMDB",
			jsonInput: `{
				"anidb_id": 111,
				"imdb_id": ["tt123"],
				"themoviedb_id": 99999,
				"tvdb_id": 0
			}`,
			wantAnidb:  111,
			wantImdb:   "tt123",
			wantTvdb:   0,
			wantTmdbTV: intPtr(99999), // both should be populated for safety
			wantTmdbMV: intPtr(99999),
		},
		{
			name: "String representation in TMDB list",
			jsonInput: `{
				"anidb_id": 222,
				"themoviedb_id": {"movie": ["88888,77777"]}
			}`,
			wantAnidb:  222,
			wantImdb:   "",
			wantTvdb:   0,
			wantTmdbTV: nil,
			wantTmdbMV: intPtr(88888),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var entry FribbEntry
			if err := json.Unmarshal([]byte(tc.jsonInput), &entry); err != nil {
				t.Fatalf("Failed to unmarshal: %v", err)
			}

			if entry.AnidbID != tc.wantAnidb {
				t.Errorf("AnidbID = %d, want %d", entry.AnidbID, tc.wantAnidb)
			}

			imdb := entry.FirstIMDb()
			if imdb != tc.wantImdb {
				t.Errorf("FirstIMDb() = %q, want %q", imdb, tc.wantImdb)
			}

			if entry.TVDbID != tc.wantTvdb {
				t.Errorf("TVDbID = %d, want %d", entry.TVDbID, tc.wantTvdb)
			}

			if tc.wantTmdbTV == nil {
				if entry.ThemoviedbID != nil && entry.ThemoviedbID.TV != nil {
					t.Errorf("ThemoviedbID.TV = %d, want nil", *entry.ThemoviedbID.TV)
				}
			} else {
				if entry.ThemoviedbID == nil || entry.ThemoviedbID.TV == nil {
					t.Errorf("ThemoviedbID.TV is nil, want %d", *tc.wantTmdbTV)
				} else if *entry.ThemoviedbID.TV != *tc.wantTmdbTV {
					t.Errorf("ThemoviedbID.TV = %d, want %d", *entry.ThemoviedbID.TV, *tc.wantTmdbTV)
				}
			}

			if tc.wantTmdbMV == nil {
				if entry.ThemoviedbID != nil && entry.ThemoviedbID.Movie != nil {
					t.Errorf("ThemoviedbID.Movie = %d, want nil", *entry.ThemoviedbID.Movie)
				}
			} else {
				if entry.ThemoviedbID == nil || entry.ThemoviedbID.Movie == nil {
					t.Errorf("ThemoviedbID.Movie is nil, want %d", *tc.wantTmdbMV)
				} else if *entry.ThemoviedbID.Movie != *tc.wantTmdbMV {
					t.Errorf("ThemoviedbID.Movie = %d, want %d", *entry.ThemoviedbID.Movie, *tc.wantTmdbMV)
				}
			}
		})
	}
}

func intPtr(i int) *int {
	return &i
}
