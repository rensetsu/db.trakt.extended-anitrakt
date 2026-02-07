package internal

import (
	"encoding/json"
	"path/filepath"
)

// LoadOverrides loads override entries from file
func LoadOverrides(mediaType string) map[int]*Override {
	overridesFile := filepath.Join("json/overrides", mediaType+"_overrides.json")
	var overrides []Override
	LoadJSONOptional(overridesFile, &overrides)

	overridesMap := make(map[int]*Override)
	for i := range overrides {
		overridesMap[overrides[i].MalID] = &overrides[i]
	}
	return overridesMap
}

// ApplyShowOverride applies override data to a show
func ApplyShowOverride(show *OutputShow, override *Override) {
	if override.TraktShow != nil {
		var traktOverride struct {
			Title *string `json:"title"`
			ID    *int    `json:"id"`
			Slug  *string `json:"slug"`
			Type  *string `json:"type"`
		}
		if err := json.Unmarshal(*override.TraktShow, &traktOverride); err == nil {
			if traktOverride.Title != nil {
				show.Trakt.Title = *traktOverride.Title
			}
			if traktOverride.ID != nil {
				show.Trakt.ID = *traktOverride.ID
			}
			if traktOverride.Slug != nil {
				show.Trakt.Slug = *traktOverride.Slug
			}
			if traktOverride.Type != nil {
				show.Trakt.Type = *traktOverride.Type
			}
		}
	}

	if override.Externals != nil {
		var extOverride TraktExternalsShow
		if err := json.Unmarshal(*override.Externals, &extOverride); err == nil {
			if extOverride.TVDB != nil {
				show.Externals.TVDB = extOverride.TVDB
			}
			if extOverride.TMDB != nil {
				show.Externals.TMDB = extOverride.TMDB
			}
			if extOverride.IMDB != nil {
				show.Externals.IMDB = extOverride.IMDB
			}
			if extOverride.TVRage != nil {
				show.Externals.TVRage = extOverride.TVRage
			}
		}
	}
}

// ApplyMovieOverride applies override data to a movie
func ApplyMovieOverride(movie *OutputMovie, override *Override) {
	if override.TraktMovie != nil {
		var traktOverride struct {
			Title *string `json:"title"`
			ID    *int    `json:"id"`
			Slug  *string `json:"slug"`
			Type  *string `json:"type"`
		}
		if err := json.Unmarshal(*override.TraktMovie, &traktOverride); err == nil {
			if traktOverride.Title != nil {
				movie.Trakt.Title = *traktOverride.Title
			}
			if traktOverride.ID != nil {
				movie.Trakt.ID = *traktOverride.ID
			}
			if traktOverride.Slug != nil {
				movie.Trakt.Slug = *traktOverride.Slug
			}
			if traktOverride.Type != nil {
				movie.Trakt.Type = *traktOverride.Type
			}
		}
	}

	if override.Externals != nil {
		var extOverride TraktExternalsMovie
		if err := json.Unmarshal(*override.Externals, &extOverride); err == nil {
			if extOverride.TMDB != nil {
				movie.Externals.TMDB = extOverride.TMDB
			}
			if extOverride.IMDB != nil {
				movie.Externals.IMDB = extOverride.IMDB
			}
			if extOverride.Letterboxd != nil {
				movie.Externals.Letterboxd = extOverride.Letterboxd
			}
		}
	}
}
