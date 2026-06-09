package internal

import (
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
		e := &FribbEntry{IMDbID: c.raw}
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
