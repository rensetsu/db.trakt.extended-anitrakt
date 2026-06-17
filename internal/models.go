package internal

import (
	"encoding/json"
	"strconv"
	"strings"
)

// InputShow structure for input shows
type InputShow struct {
	Title       string `json:"title"`
	MalID       int    `json:"mal_id"`
	TraktID     int    `json:"trakt_id"`
	GuessedSlug string `json:"guessed_slug"`
	Season      int    `json:"season"`
	Type        string `json:"type"`
}

// InputMovie structure for input movies
type InputMovie struct {
	Title       string `json:"title"`
	MalID       int    `json:"mal_id"`
	TraktID     int    `json:"trakt_id"`
	GuessedSlug string `json:"guessed_slug"`
	Type        string `json:"type"`
}

// NotFoundEntry structure for items not found on Trakt
type NotFoundEntry struct {
	MalID int    `json:"mal_id"`
	Title string `json:"title"`
}

// LetterboxdResponse structure for JSON response
type LetterboxdResponse struct {
	ID   int    `json:"id"`
	LID  string `json:"lid"`
	Slug string `json:"slug"`
}

// Letterboxd structure for output
type Letterboxd struct {
	Slug *string `json:"slug"`
	UID  *int    `json:"uid"`
	LID  *string `json:"lid"`
}

// Trakt API structures
type TraktExternals struct {
	TVDB   *int    `json:"tvdb,omitempty"`
	TMDB   *int    `json:"tmdb,omitempty"`
	IMDB   *string `json:"imdb,omitempty"`
	TVRage *int    `json:"tvrage,omitempty"`
}

type TraktShow struct {
	Title string `json:"title"`
	IDs   struct {
		Trakt int     `json:"trakt"`
		Slug  string  `json:"slug"`
		TVDB  *int    `json:"tvdb,omitempty"`
		IMDB  *string `json:"imdb,omitempty"`
		TMDB  *int    `json:"tmdb,omitempty"`
	} `json:"ids"`
	Year int `json:"year"`
}

type TraktMovie struct {
	Title string `json:"title"`
	IDs   struct {
		Trakt int     `json:"trakt"`
		Slug  string  `json:"slug"`
		IMDB  *string `json:"imdb,omitempty"`
		TMDB  *int    `json:"tmdb,omitempty"`
	} `json:"ids"`
	Year int `json:"year"`
}

type TraktSeason struct {
	Number int `json:"number"`
	IDs    struct {
		Trakt  int  `json:"trakt"`
		TVDB   *int `json:"tvdb,omitempty"`
		TMDB   *int `json:"tmdb,omitempty"`
		TVRage *int `json:"tvrage,omitempty"`
	} `json:"ids"`
}

type TraktExternalsShow struct {
	TVDB   *int    `json:"tvdb"`
	TMDB   *int    `json:"tmdb"`
	IMDB   *string `json:"imdb"`
	TVRage *int    `json:"tvrage"`
}

type TraktExternalsSeason struct {
	TVDB   *int `json:"tvdb"`
	TMDB   *int `json:"tmdb"`
	TVRage *int `json:"tvrage"`
}

type TraktExternalsMovie struct {
	TMDB       *int        `json:"tmdb"`
	IMDB       *string     `json:"imdb"`
	Letterboxd *Letterboxd `json:"letterboxd"`
}

// OutputShow structure
type OutputShow struct {
	MyAnimeList struct {
		Title string `json:"title"`
		ID    int    `json:"id"`
	} `json:"myanimelist"`
	Trakt struct {
		Title  string `json:"title"`
		ID     int    `json:"id"`
		Slug   string `json:"slug"`
		Type   string `json:"type"`
		Season *struct {
			ID        int                   `json:"id"`
			Number    int                   `json:"number"`
			Externals *TraktExternalsSeason `json:"externals"`
		} `json:"season"`
		IsSplitCour bool `json:"is_split_cour"`
	} `json:"trakt"`
	ReleaseYear int                 `json:"release_year"`
	Externals   *TraktExternalsShow `json:"externals"`
}

// OutputMovie structure
type OutputMovie struct {
	MyAnimeList struct {
		Title string `json:"title"`
		ID    int    `json:"id"`
	} `json:"myanimelist"`
	Trakt struct {
		Title string `json:"title"`
		ID    int    `json:"id"`
		Slug  string `json:"slug"`
		Type  string `json:"type"`
	} `json:"trakt"`
	ReleaseYear int                  `json:"release_year"`
	Externals   *TraktExternalsMovie `json:"externals"`
}

// Config structure
type Config struct {
	APIKey                string
	TvFile                string
	MovieFile             string
	OutputFile            string
	Verbose               bool
	NoProgress            bool
	TempDir               string
	Force                 bool
	RateLimiter           *RateLimiter
	LetterboxdRateLimiter *RateLimiter
	// Fribb-based ingestion
	FribbFile    string // path to anime-lists-reduced.json (empty = fetch from GitHub)
	AnimeAPIFile string // path to animeapi.tsv (empty = fetch from animeapi.my.id)
	UseFribb     bool   // true when -fribb or -animeapi was explicitly passed
}

// ChangeDetail structure for tracking changes
type ChangeDetail struct {
	MalID  int    `json:"mal_id"`
	Title  string `json:"title"`
	Reason string `json:"reason"`
}

// ProcessingStats structure for tracking statistics
type ProcessingStats struct {
	MediaType                 string         `json:"media_type"`
	TotalBefore               int            `json:"total_before"`
	TotalAfter                int            `json:"total_after"`
	Created                   int            `json:"created"`
	Updated                   int            `json:"updated"`
	Modified                  int            `json:"modified"`
	NotFound                  int            `json:"not_found"`
	CreatedDetails            []ChangeDetail `json:"created_details"`
	UpdatedDetails            []ChangeDetail `json:"updated_details"`
	ModifiedDetails           []ChangeDetail `json:"modified_details"`
	NotFoundDetails           []ChangeDetail `json:"not_found_details"`
	DuplicateDetails          []ChangeDetail `json:"duplicate_details"`
	LetterboxdNotFoundDetails []ChangeDetail `json:"letterboxd_not_found_details"`
}

// Override structure
type Override struct {
	MalID       int              `json:"mal_id"`
	Description string           `json:"description"`
	Trakt       *json.RawMessage `json:"trakt,omitempty"`
	Externals   *json.RawMessage `json:"externals,omitempty"`
	Ignore      bool             `json:"ignore,omitempty"`
}

// ---------------------------------------------------------------------------
// Fribb anime-lists models
// ---------------------------------------------------------------------------

// FribbThemoviedbID holds the TMDB ID under either a "tv" or "movie" key.
// Only one of them will be populated per entry.
type FribbThemoviedbID struct {
	TV    *int `json:"tv,omitempty"`
	Movie *int `json:"movie,omitempty"`
}

// parseSingleOrListInt is a helper to parse single int, list of ints, string, or list of strings.
func parseSingleOrListInt(d []byte) (*int, error) {
	if len(d) == 0 || string(d) == "null" {
		return nil, nil
	}
	// Array
	if d[0] == '[' {
		var rawList []json.RawMessage
		if err := json.Unmarshal(d, &rawList); err != nil {
			return nil, err
		}
		if len(rawList) == 0 {
			return nil, nil
		}
		// Check recursively on the first element of the list
		var val *int
		var recursiveErr error
		if len(rawList) > 0 {
			val, recursiveErr = parseSingleOrListInt(rawList[0])
		}
		return val, recursiveErr
	}
	// String
	if d[0] == '"' {
		var s string
		if err := json.Unmarshal(d, &s); err != nil {
			return nil, err
		}
		s = strings.TrimSpace(s)
		if s == "" {
			return nil, nil
		}
		parts := strings.Split(s, ",")
		first := strings.TrimSpace(parts[0])
		val, err := strconv.Atoi(first)
		if err != nil {
			return nil, err
		}
		return &val, nil
	}
	// Integer/Number
	var val int
	if err := json.Unmarshal(d, &val); err != nil {
		return nil, err
	}
	return &val, nil
}

// UnmarshalJSON custom unmarshaler for FribbThemoviedbID to handle dictionaries,
// lists, nested types, and flat values.
func (ft *FribbThemoviedbID) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}

	// 1. If it's a JSON object/dictionary: {"movie": [434326]} or {"tv": 12345}
	if data[0] == '{' {
		var rawMap map[string]json.RawMessage
		if err := json.Unmarshal(data, &rawMap); err != nil {
			return err
		}
		if movieData, exists := rawMap["movie"]; exists {
			val, err := parseSingleOrListInt(movieData)
			if err != nil {
				return err
			}
			ft.Movie = val
		}
		if tvData, exists := rawMap["tv"]; exists {
			val, err := parseSingleOrListInt(tvData)
			if err != nil {
				return err
			}
			ft.TV = val
		}
		return nil
	}

	// 2. If it's a flat value (int, string, or list), populate both TV and Movie to be safe
	val, err := parseSingleOrListInt(data)
	if err != nil {
		return err
	}
	ft.TV = val
	ft.Movie = val
	return nil
}

// FribbIMDbID is a custom type to handle IMDb ID values which can be strings or arrays of strings.
type FribbIMDbID string

// UnmarshalJSON custom unmarshaler for FribbIMDbID.
func (f *FribbIMDbID) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	if data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		*f = FribbIMDbID(s)
		return nil
	}
	if data[0] == '[' {
		var l []string
		if err := json.Unmarshal(data, &l); err != nil {
			return err
		}
		*f = FribbIMDbID(strings.Join(l, ","))
		return nil
	}
	return nil
}

// FribbSeason holds the TVDB / TMDB season numbers used by Fribb.
type FribbSeason struct {
	TVDB *int `json:"tvdb,omitempty"`
	TMDB *int `json:"tmdb,omitempty"`
}

// FribbEntry is one record from anime-lists-reduced.json.
// imdb_id may be a comma-separated list or array of strings; we take only the first value.
type FribbEntry struct {
	AnidbID      int                `json:"anidb_id"`
	IMDbID       FribbIMDbID        `json:"imdb_id,omitempty"`
	ThemoviedbID *FribbThemoviedbID `json:"themoviedb_id,omitempty"`
	TVDbID       int                `json:"tvdb_id,omitempty"`
	Season       *FribbSeason       `json:"season,omitempty"`
}

// FirstIMDb returns the first IMDb ID from a potentially comma-separated list.
func (f *FribbEntry) FirstIMDb() string {
	if f.IMDbID == "" {
		return ""
	}
	parts := strings.SplitN(string(f.IMDbID), ",", 2)
	return strings.TrimSpace(parts[0])
}

// ---------------------------------------------------------------------------
// AnimeAPI TSV row model
// ---------------------------------------------------------------------------

// AnimeAPIRow represents one row of animeapi.tsv.
// Only the fields we need are parsed; the rest are ignored.
type AnimeAPIRow struct {
	Title        string
	AniDB        int
	MyAnimeList  int
	TraktID      int
	TraktType    string // "shows" or "movies"
	TraktSeason  int    // trakt_season
	TMDB         int    // themoviedb
	TMDBType     string // themoviedb_type: "tv" or "movie"
	TMDBSeasonID int    // themoviedb_season_id
}

// animeAPIColumns is a helper to map header names to column indices.
type animeAPIColumns struct {
	title              int
	anidb              int
	myanimelist        int
	themoviedb         int
	themoviedbType     int
	themoviedbSeasonID int
	trakt              int
	traktType          int
	traktSeason        int
}

// parseAnimeAPIColumns maps a header row to column indices.
func parseAnimeAPIColumns(headers []string) animeAPIColumns {
	cols := animeAPIColumns{
		title:              -1,
		anidb:              -1,
		myanimelist:        -1,
		themoviedb:         -1,
		themoviedbType:     -1,
		themoviedbSeasonID: -1,
		trakt:              -1,
		traktType:          -1,
		traktSeason:        -1,
	}
	for i, h := range headers {
		switch strings.TrimSpace(h) {
		case "title":
			cols.title = i
		case "anidb":
			cols.anidb = i
		case "myanimelist":
			cols.myanimelist = i
		case "themoviedb":
			cols.themoviedb = i
		case "themoviedb_type":
			cols.themoviedbType = i
		case "themoviedb_season_id":
			cols.themoviedbSeasonID = i
		case "trakt":
			cols.trakt = i
		case "trakt_type":
			cols.traktType = i
		case "trakt_season":
			cols.traktSeason = i
		}
	}
	return cols
}

// parseInt parses a TSV cell as int, returning 0 on blank/error.
func parseInt(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return v
}

// ---------------------------------------------------------------------------
// Trakt /search/:id_type/:id response models
// ---------------------------------------------------------------------------

// TraktSearchResult is one element of the search-by-ID array response.
type TraktSearchResult struct {
	Type  string      `json:"type"`
	Score float64     `json:"score"`
	Show  *TraktShow  `json:"show,omitempty"`
	Movie *TraktMovie `json:"movie,omitempty"`
}
