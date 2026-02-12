package internal

import "encoding/json"

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
}

// ChangeDetail structure for tracking changes
type ChangeDetail struct {
	MalID  int    `json:"mal_id"`
	Title  string `json:"title"`
	Reason string `json:"reason"`
}

// ProcessingStats structure for tracking statistics
type ProcessingStats struct {
	MediaType       string         `json:"media_type"`
	TotalBefore     int            `json:"total_before"`
	TotalAfter      int            `json:"total_after"`
	Created         int            `json:"created"`
	Updated         int            `json:"updated"`
	Modified        int            `json:"modified"`
	NotFound        int            `json:"not_found"`
	CreatedDetails  []ChangeDetail `json:"created_details"`
	UpdatedDetails  []ChangeDetail `json:"updated_details"`
	ModifiedDetails []ChangeDetail `json:"modified_details"`
	NotFoundDetails []ChangeDetail `json:"not_found_details"`
}

// Override structure
type Override struct {
	MalID       int              `json:"mal_id"`
	Description string           `json:"description"`
	TraktShow   *json.RawMessage `json:"trakt,omitempty"`
	TraktMovie  *json.RawMessage `json:"trakt,omitempty"`
	Externals   *json.RawMessage `json:"externals,omitempty"`
	Ignore      bool             `json:"ignore,omitempty"`
}
