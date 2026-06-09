package internal

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// AnimeAPI TSV loader
// ---------------------------------------------------------------------------

// LoadAnimeAPITSV parses animeapi.tsv (or fetches it from animeapi.my.id if
// path is empty / the file does not exist) and returns:
//
//   - anidbToMAL  : AniDB ID → MAL ID  (for cross-referencing with Fribb)
//   - malToRow    : MAL ID  → full row  (for title lookup)
func LoadAnimeAPITSV(path string) (anidbToMAL map[int]int, malToRow map[int]AnimeAPIRow, err error) {
	anidbToMAL = make(map[int]int)
	malToRow = make(map[int]AnimeAPIRow)

	var r io.Reader
	if path != "" {
		f, openErr := os.Open(path)
		if openErr != nil {
			return nil, nil, fmt.Errorf("open animeapi tsv %s: %w", path, openErr)
		}
		defer f.Close()
		r = f
	} else {
		// Fall back to remote — lowercase path is required by the Vercel function
		fmt.Println("Fetching AnimeAPI TSV from animeapi.my.id …")
		resp, httpErr := http.Get("https://animeapi.my.id/animeapi.tsv")
		if httpErr != nil {
			return nil, nil, fmt.Errorf("fetch animeapi tsv: %w", httpErr)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return nil, nil, fmt.Errorf("fetch animeapi tsv: HTTP %d (expected 200)", resp.StatusCode)
		}
		r = resp.Body
	}

	scanner := bufio.NewScanner(r)
	// The TSV can have very long lines; bump the buffer.
	scanner.Buffer(make([]byte, 0, 1024*1024), 4*1024*1024)

	var cols animeAPIColumns
	lineNum := 0
	for scanner.Scan() {
		line := scanner.Text()
		lineNum++
		fields := strings.Split(line, "\t")

		if lineNum == 1 {
			cols = parseAnimeAPIColumns(fields)
			if cols.anidb < 0 || cols.myanimelist < 0 {
				return nil, nil, fmt.Errorf("animeapi tsv: could not find required columns (anidb, myanimelist) in header")
			}
			continue
		}

		if len(fields) <= cols.myanimelist || len(fields) <= cols.anidb {
			continue
		}

		anidbID := parseInt(fields[cols.anidb])
		malID := parseInt(fields[cols.myanimelist])
		if anidbID == 0 || malID == 0 {
			continue
		}

		row := AnimeAPIRow{
			AniDB:       anidbID,
			MyAnimeList: malID,
		}
		if cols.title >= 0 && cols.title < len(fields) {
			row.Title = strings.TrimSpace(fields[cols.title])
		}
		if cols.trakt >= 0 && cols.trakt < len(fields) {
			row.TraktID = parseInt(fields[cols.trakt])
		}
		if cols.traktType >= 0 && cols.traktType < len(fields) {
			row.TraktType = strings.TrimSpace(fields[cols.traktType])
		}
		if cols.traktSeason >= 0 && cols.traktSeason < len(fields) {
			row.TraktSeason = parseInt(fields[cols.traktSeason])
		}
		if cols.themoviedb >= 0 && cols.themoviedb < len(fields) {
			row.TMDB = parseInt(fields[cols.themoviedb])
		}
		if cols.themoviedbType >= 0 && cols.themoviedbType < len(fields) {
			row.TMDBType = strings.TrimSpace(fields[cols.themoviedbType])
		}
		if cols.themoviedbSeasonID >= 0 && cols.themoviedbSeasonID < len(fields) {
			row.TMDBSeasonID = parseInt(fields[cols.themoviedbSeasonID])
		}

		anidbToMAL[anidbID] = malID
		malToRow[malID] = row
	}
	if scanErr := scanner.Err(); scanErr != nil {
		return nil, nil, fmt.Errorf("scan animeapi tsv: %w", scanErr)
	}

	return anidbToMAL, malToRow, nil
}

// ---------------------------------------------------------------------------
// Fribb JSON loader
// ---------------------------------------------------------------------------

// LoadFribbJSON loads Fribb's anime-lists-reduced.json from a local file or
// fetches it from GitHub if path is empty / file doesn't exist.
func LoadFribbJSON(path string) ([]FribbEntry, error) {
	var r io.Reader

	if path != "" {
		f, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("open fribb json %s: %w", path, err)
		}
		defer f.Close()
		r = f
	} else {
		fmt.Println("Fetching Fribb anime-lists-reduced.json from GitHub …")
		resp, err := http.Get("https://raw.githubusercontent.com/Fribb/anime-lists/master/anime-lists-reduced.json")
		if err != nil {
			return nil, fmt.Errorf("fetch fribb json: %w", err)
		}
		defer resp.Body.Close()
		r = resp.Body
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read fribb json: %w", err)
	}

	var entries []FribbEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parse fribb json: %w", err)
	}

	return entries, nil
}

// ---------------------------------------------------------------------------
// Main Fribb processor
// ---------------------------------------------------------------------------

// isMovieTitle checks if a title contains movie-related keywords in English or Japanese.
// Matches: movie, film, theatrical, cinema, feature, eiga (映画), gekijouban (劇場版),
// eigakan (映画館 - cinema/movie theater), etc.
func isMovieTitle(title string) bool {
	// Pattern matches word boundaries followed by movie-related terms
	moviePattern := regexp.MustCompile(`(?i)\b(movie|film|theatrical|cinema|feature|eiga|eigakan|gekijouban|geki.*ban|shinema)\b`)
	return moviePattern.MatchString(title)
}

// ProcessFribb is the top-level entry point for the Fribb-based ingestion
// pipeline. It:
//
//  1. Loads Fribb's anime-lists-reduced.json
//  2. Loads AnimeAPI TSV to get AniDB → MAL ID mappings
//  3. Cross-references every Fribb entry, resolving Trakt via:
//     - TV shows  : TMDB TV ID (primary) → TVDB ID (fallback)
//     - Movies    : TMDB movie ID (primary) → IMDB ID (fallback)
//  4. Drops entries whose MAL ID is already present in the existing output files
//  5. For each remaining entry, searches Trakt by the resolved external ID
//  6. Merges new results into the existing output files
func ProcessFribb(config Config) {
	// --- 1. Load Fribb data --------------------------------------------------
	fribbEntries, err := LoadFribbJSON(config.FribbFile)
	if err != nil {
		log.Fatalf("Failed to load Fribb data: %v", err)
	}
	fmt.Printf("Loaded %d entries from Fribb anime-lists\n", len(fribbEntries))

	// --- 2. Load AnimeAPI TSV ------------------------------------------------
	anidbToMAL, malToRow, err := LoadAnimeAPITSV(config.AnimeAPIFile)
	if err != nil {
		log.Fatalf("Failed to load AnimeAPI TSV: %v", err)
	}
	fmt.Printf("Loaded %d AniDB→MAL mappings from AnimeAPI\n", len(anidbToMAL))

	// --- 3. Load existing output files ---------------------------------------
	tvOutputFile := filepath.Join("json/output", "tv_ex.json")
	movieOutputFile := filepath.Join("json/output", "movies_ex.json")

	var existingShows []OutputShow
	var existingMovies []OutputMovie
	LoadJSONOptional(tvOutputFile, &existingShows)
	LoadJSONOptional(movieOutputFile, &existingMovies)

	existingShowMAL := make(map[int]OutputShow)
	existingMovieMAL := make(map[int]OutputMovie)
	for _, s := range existingShows {
		existingShowMAL[s.MyAnimeList.ID] = s
	}
	for _, m := range existingMovies {
		existingMovieMAL[m.MyAnimeList.ID] = m
	}

	showNotExistMap := LoadNotFound(tvOutputFile)
	movieNotExistMap := LoadNotFound(movieOutputFile)
	showOverrides := LoadOverrides("tv")
	movieOverrides := LoadOverrides("movies")

	// --- 4. Build work list --------------------------------------------------
	// workItem carries everything needed to process one entry.
	// lookupType is "tmdb", "tvdb", or "imdb".
	// lookupID   is the string form of the external ID (e.g. "tt1234567" for IMDB).
	// season     is the best-available season number (TMDB or TVDB) for TV shows.
	type workItem struct {
		fribb      FribbEntry
		malID      int
		title      string
		isMovie    bool
		lookupType string // "tmdb" | "tvdb" | "imdb"
		lookupID   string
		season     int // season number for TV shows
	}

	var tvWork []workItem
	var movieWork []workItem
	skippedExisting := 0
	skippedNoMAL := 0
	skippedNoID := 0 // no usable external ID at all
	skippedSeason0 := 0
	skippedTypeMismatch := 0

	for _, entry := range fribbEntries {
		// Skip entries with season 0
		if entry.Season != nil && ((entry.Season.TMDB != nil && *entry.Season.TMDB == 0) || (entry.Season.TVDB != nil && *entry.Season.TVDB == 0)) {
			skippedSeason0++
			continue
		}

		malID, ok := anidbToMAL[entry.AnidbID]
		if !ok || malID == 0 {
			skippedNoMAL++
			continue
		}

		row := malToRow[malID]
		title := row.Title
		if title == "" {
			title = fmt.Sprintf("AniDB:%d / MAL:%d", entry.AnidbID, malID)
		}

		// Sanity check: if this will be processed as TV, verify AnimeAPI title doesn't contain movie-related terms
		isTVFromFribb := (entry.ThemoviedbID != nil && entry.ThemoviedbID.TV != nil && *entry.ThemoviedbID.TV > 0) ||
			(entry.ThemoviedbID == nil && entry.TVDbID > 0)

		if isTVFromFribb && isMovieTitle(row.Title) {
			skippedTypeMismatch++
			continue
		}

		// Helper: check if a MAL ID should be skipped for TV
		skipTV := func() bool {
			if existingShowMAL[malID].MyAnimeList.ID != 0 {
				skippedExisting++
				return true
			}
			if showNotExistMap[malID] {
				skippedExisting++
				return true
			}
			return false
		}

		// Helper: check if a MAL ID should be skipped for movies
		skipMovie := func() bool {
			if existingMovieMAL[malID].MyAnimeList.ID != 0 {
				skippedExisting++
				return true
			}
			if movieNotExistMap[malID] {
				skippedExisting++
				return true
			}
			return false
		}

		// ----- Primary paths (TMDB) -----------------------------------------

		// Movie via TMDB
		if entry.ThemoviedbID != nil && entry.ThemoviedbID.Movie != nil && *entry.ThemoviedbID.Movie > 0 {
			if skipMovie() {
				continue
			}
			movieWork = append(movieWork, workItem{
				fribb:      entry,
				malID:      malID,
				title:      title,
				isMovie:    true,
				lookupType: "tmdb",
				lookupID:   fmt.Sprintf("%d", *entry.ThemoviedbID.Movie),
			})
			continue
		}

		// TV via TMDB
		if entry.ThemoviedbID != nil && entry.ThemoviedbID.TV != nil && *entry.ThemoviedbID.TV > 0 {
			if skipTV() {
				continue
			}
			seasonNum := 1
			if entry.Season != nil && entry.Season.TMDB != nil && *entry.Season.TMDB > 0 {
				seasonNum = *entry.Season.TMDB
			}
			tvWork = append(tvWork, workItem{
				fribb:      entry,
				malID:      malID,
				title:      title,
				isMovie:    false,
				lookupType: "tmdb",
				lookupID:   fmt.Sprintf("%d", *entry.ThemoviedbID.TV),
				season:     seasonNum,
			})
			continue
		}

		// ----- Fallback paths (TVDB for TV, IMDB for movies) ----------------

		// TV via TVDB (no TMDB TV ID available)
		if entry.TVDbID > 0 {
			if skipTV() {
				continue
			}
			// Use TVDB season number as best approximation
			seasonNum := 1
			if entry.Season != nil && entry.Season.TVDB != nil && *entry.Season.TVDB > 0 {
				seasonNum = *entry.Season.TVDB
			}
			tvWork = append(tvWork, workItem{
				fribb:      entry,
				malID:      malID,
				title:      title,
				isMovie:    false,
				lookupType: "tvdb",
				lookupID:   fmt.Sprintf("%d", entry.TVDbID),
				season:     seasonNum,
			})
			continue
		}

		// Movie via IMDB (no TMDB movie ID and no TVDB ID)
		if imdb := entry.FirstIMDb(); imdb != "" {
			if skipMovie() {
				continue
			}
			movieWork = append(movieWork, workItem{
				fribb:      entry,
				malID:      malID,
				title:      title,
				isMovie:    true,
				lookupType: "imdb",
				lookupID:   imdb,
			})
			continue
		}

		skippedNoID++
	}

	fmt.Printf("Work list: %d TV shows, %d movies  (skipped: %d existing, %d no-MAL, %d no-ID, %d season-0, %d type-mismatch)\n",
		len(tvWork), len(movieWork), skippedExisting, skippedNoMAL, skippedNoID, skippedSeason0, skippedTypeMismatch)

	// Ensure the search cache dir exists
	os.MkdirAll(filepath.Join(config.TempDir, "search"), 0755)

	client := &http.Client{Timeout: 30 * time.Second}

	// -------------------------------------------------------------------------
	// 5a. Process TV shows
	// -------------------------------------------------------------------------
	tvStats := ProcessingStats{
		MediaType:       "tv (fribb)",
		TotalBefore:     len(existingShowMAL),
		CreatedDetails:  []ChangeDetail{},
		UpdatedDetails:  []ChangeDetail{},
		ModifiedDetails: []ChangeDetail{},
		NotFoundDetails: []ChangeDetail{},
	}
	var tvNewNotExist []NotFoundEntry
	tvBar := setupProgressBar(len(tvWork), "Processing Fribb TV shows", config.NoProgress)

	for _, item := range tvWork {
		tvBar.Add(1)

		if override, exists := showOverrides[item.malID]; exists && override.Ignore {
			if config.Verbose {
				fmt.Printf("\nSkipping ignored show: %s (MAL ID: %d)", item.title, item.malID)
			}
			continue
		}

		results, err := FetchTraktByExternalID(client, config, item.lookupType, item.lookupID, "show")
		if err != nil {
			if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "no results") {
				tvNewNotExist = append(tvNewNotExist, NotFoundEntry{MalID: item.malID, Title: item.title})
				tvStats.NotFoundDetails = append(tvStats.NotFoundDetails, ChangeDetail{
					MalID:  item.malID,
					Title:  item.title,
					Reason: fmt.Sprintf("Not found on Trakt via %s ID %s", item.lookupType, item.lookupID),
				})
			} else {
				log.Printf("Error searching Trakt (%s %s) for MAL %d: %v",
					item.lookupType, item.lookupID, item.malID, err)
			}
			continue
		}

		// Pick the first show result
		var traktShow *TraktShow
		for i := range results {
			if results[i].Type == "show" && results[i].Show != nil {
				traktShow = results[i].Show
				break
			}
		}
		if traktShow == nil {
			tvNewNotExist = append(tvNewNotExist, NotFoundEntry{MalID: item.malID, Title: item.title})
			tvStats.NotFoundDetails = append(tvStats.NotFoundDetails, ChangeDetail{
				MalID:  item.malID,
				Title:  item.title,
				Reason: fmt.Sprintf("Trakt returned no show for %s ID %s", item.lookupType, item.lookupID),
			})
			continue
		}

		outputShow := &OutputShow{
			MyAnimeList: struct {
				Title string `json:"title"`
				ID    int    `json:"id"`
			}{Title: item.title, ID: item.malID},
			Trakt: struct {
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
			}{
				Title: traktShow.Title,
				ID:    traktShow.IDs.Trakt,
				Slug:  traktShow.IDs.Slug,
				Type:  "shows",
			},
			ReleaseYear: traktShow.Year,
			Externals:   &TraktExternalsShow{TVDB: traktShow.IDs.TVDB, TMDB: traktShow.IDs.TMDB, IMDB: traktShow.IDs.IMDB},
		}

		updateSeasonInfo(client, config, outputShow, traktShow.IDs.Trakt, item.season)

		if override, exists := showOverrides[item.malID]; exists && !override.Ignore {
			ApplyShowOverride(outputShow, override)
			tvStats.ModifiedDetails = append(tvStats.ModifiedDetails, ChangeDetail{
				MalID:  item.malID,
				Title:  item.title,
				Reason: override.Description,
			})
		}

		existingShowMAL[item.malID] = *outputShow
		tvStats.CreatedDetails = append(tvStats.CreatedDetails, ChangeDetail{
			MalID:  item.malID,
			Title:  item.title,
			Reason: fmt.Sprintf("Added via Fribb: %s ID %s", item.lookupType, item.lookupID),
		})

		if config.Verbose {
			fmt.Printf("\n  ✓ %s (MAL %d) → Trakt %d (%s) [via %s]",
				item.title, item.malID, traktShow.IDs.Trakt, traktShow.IDs.Slug, item.lookupType)
		}
	}

	tvStats.TotalAfter = len(existingShowMAL)
	tvStats.Created = len(tvStats.CreatedDetails)
	tvStats.NotFound = len(tvStats.NotFoundDetails)
	tvStats.Modified = len(tvStats.ModifiedDetails)

	SaveResults(tvOutputFile, existingShowMAL)
	SaveNotFound(tvOutputFile, tvNewNotExist, showNotExistMap)
	OutputStats("tv (fribb)", tvStats)

	// -------------------------------------------------------------------------
	// 5b. Process movies
	// -------------------------------------------------------------------------
	movieStats := ProcessingStats{
		MediaType:                 "movies (fribb)",
		TotalBefore:               len(existingMovieMAL),
		CreatedDetails:            []ChangeDetail{},
		UpdatedDetails:            []ChangeDetail{},
		ModifiedDetails:           []ChangeDetail{},
		NotFoundDetails:           []ChangeDetail{},
		LetterboxdNotFoundDetails: []ChangeDetail{},
	}
	var movieNewNotExist []NotFoundEntry
	movieBar := setupProgressBar(len(movieWork), "Processing Fribb movies", config.NoProgress)

	for _, item := range movieWork {
		movieBar.Add(1)

		if override, exists := movieOverrides[item.malID]; exists && override.Ignore {
			if config.Verbose {
				fmt.Printf("\nSkipping ignored movie: %s (MAL ID: %d)", item.title, item.malID)
			}
			continue
		}

		results, err := FetchTraktByExternalID(client, config, item.lookupType, item.lookupID, "movie")
		if err != nil {
			if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "no results") {
				movieNewNotExist = append(movieNewNotExist, NotFoundEntry{MalID: item.malID, Title: item.title})
				movieStats.NotFoundDetails = append(movieStats.NotFoundDetails, ChangeDetail{
					MalID:  item.malID,
					Title:  item.title,
					Reason: fmt.Sprintf("Not found on Trakt via %s ID %s", item.lookupType, item.lookupID),
				})
			} else {
				log.Printf("Error searching Trakt (%s %s) for MAL %d: %v",
					item.lookupType, item.lookupID, item.malID, err)
			}
			continue
		}

		var traktMovie *TraktMovie
		for i := range results {
			if results[i].Type == "movie" && results[i].Movie != nil {
				traktMovie = results[i].Movie
				break
			}
		}
		if traktMovie == nil {
			movieNewNotExist = append(movieNewNotExist, NotFoundEntry{MalID: item.malID, Title: item.title})
			movieStats.NotFoundDetails = append(movieStats.NotFoundDetails, ChangeDetail{
				MalID:  item.malID,
				Title:  item.title,
				Reason: fmt.Sprintf("Trakt returned no movie for %s ID %s", item.lookupType, item.lookupID),
			})
			continue
		}

		outputMovie := &OutputMovie{
			MyAnimeList: struct {
				Title string `json:"title"`
				ID    int    `json:"id"`
			}{Title: item.title, ID: item.malID},
			Trakt: struct {
				Title string `json:"title"`
				ID    int    `json:"id"`
				Slug  string `json:"slug"`
				Type  string `json:"type"`
			}{
				Title: traktMovie.Title,
				ID:    traktMovie.IDs.Trakt,
				Slug:  traktMovie.IDs.Slug,
				Type:  "movies",
			},
			ReleaseYear: traktMovie.Year,
			Externals: &TraktExternalsMovie{
				TMDB: traktMovie.IDs.TMDB,
				IMDB: traktMovie.IDs.IMDB,
			},
		}

		// Try to enrich with Letterboxd data
		letterboxdNotFound := updateLetterboxdInfo(client, config, outputMovie, nil)
		if letterboxdNotFound != nil {
			movieStats.LetterboxdNotFoundDetails = append(movieStats.LetterboxdNotFoundDetails, *letterboxdNotFound)
		}

		if override, exists := movieOverrides[item.malID]; exists && !override.Ignore {
			ApplyMovieOverride(outputMovie, override)
			movieStats.ModifiedDetails = append(movieStats.ModifiedDetails, ChangeDetail{
				MalID:  item.malID,
				Title:  item.title,
				Reason: override.Description,
			})
		}

		existingMovieMAL[item.malID] = *outputMovie
		movieStats.CreatedDetails = append(movieStats.CreatedDetails, ChangeDetail{
			MalID:  item.malID,
			Title:  item.title,
			Reason: fmt.Sprintf("Added via Fribb: %s ID %s", item.lookupType, item.lookupID),
		})

		if config.Verbose {
			fmt.Printf("\n  ✓ %s (MAL %d) → Trakt %d (%s) [via %s]",
				item.title, item.malID, traktMovie.IDs.Trakt, traktMovie.IDs.Slug, item.lookupType)
		}
	}

	movieStats.TotalAfter = len(existingMovieMAL)
	movieStats.Created = len(movieStats.CreatedDetails)
	movieStats.NotFound = len(movieStats.NotFoundDetails)
	movieStats.Modified = len(movieStats.ModifiedDetails)

	SaveMovieResults(movieOutputFile, existingMovieMAL)
	SaveNotFound(movieOutputFile, movieNewNotExist, movieNotExistMap)
	OutputStats("movies (fribb)", movieStats)

	fmt.Printf("\nFribb processing complete: %d shows, %d movies added.\n",
		tvStats.Created, movieStats.Created)
}
