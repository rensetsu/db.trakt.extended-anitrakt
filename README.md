# Extended Trakt Database for AniTrakt

This application processes anime data from MyAnimeList (MAL) and retrieves extended information from Trakt.tv, creating enriched JSON files with comprehensive metadata.

## Overview

The application takes JSON files containing anime titles with MAL IDs and Trakt IDs, then fetches additional metadata from Trakt.tv to create extended database files. This is particularly useful for applications that need both MAL and Trakt data for anime shows and movies.

## Input File Schemas

### TV Shows Input (`tv.json`)

```typescript
interface InputShow {
  title: string;        // Anime title from MAL
  mal_id: number;       // MyAnimeList ID
  trakt_id: number;     // Trakt.tv ID
  guessed_slug: string; // Trakt slug (URL identifier)
  season: number;       // Season number for the anime
  type: string;         // Content type ("tv")
}

type InputShowList = InputShow[];
```

### Movies Input (`movies.json`)

```typescript
interface InputMovie {
  title: string;        // Anime movie title from MAL
  mal_id: number;       // MyAnimeList ID
  trakt_id: number;     // Trakt.tv ID
  guessed_slug: string; // Trakt slug (URL identifier)
  type: string;         // Content type ("movies")
}

type InputMovieList = InputMovie[];
```

## Output File Schemas

### Extended TV Shows Output (`tv_ex.json`)

```typescript
interface OutputShow {
  myanimelist: {
    title: string;      // MAL title
    id: number;         // MAL ID
  };
  trakt: {
    title: string;      // Trakt title
    id: number;         // Trakt ID
    slug: string;       // Trakt slug
    type: string;       // "shows"
    is_split_cour: boolean; // Whether anime spans multiple seasons
    release_year: number;   // Year of release
    season?: {          // Season information (if not split cour)
      id: number;       // Season ID on Trakt
      number: number;   // Season number
      externals: {
        tvdb?: number;  // TVDB ID
        tmdb?: number;  // TMDB ID
        tvrage?: number; // TVRage ID
      };
    };
    externals: {
      tvdb?: number;    // TVDB ID for the show
      tmdb?: number;    // TMDB ID for the show
      imdb?: string;    // IMDB ID for the show
      tvrage?: number;  // TVRage ID for the show
    };
  };
}

type OutputShowList = OutputShow[];
```

### Extended Movies Output (`movies_ex.json`)

```typescript
interface OutputMovie {
  myanimelist: {
    title: string;      // MAL title
    id: number;         // MAL ID
  };
  trakt: {
    title: string;      // Trakt title
    id: number;         // Trakt ID
    slug: string;       // Trakt slug
    type: string;       // "movies"
    release_year: number; // Year of release
    externals: {
      tmdb?: number;    // TMDB ID
      imdb?: string;    // IMDB ID
    };
  };
}

type OutputMovieList = OutputMovie[];
```

## Not Found Files Schema

When anime entries cannot be found on Trakt.tv, they are logged in separate files for easy identification:

### Not Found Entries (`not_exist_tv_ex.json`, `not_exist_movies_ex.json`)

```typescript
interface NotFoundEntry {
  mal_id: number;       // MyAnimeList ID of the missing entry
  title: string;        // Title of the anime that wasn't found
}

type NotFoundList = NotFoundEntry[];
```

**Example:**
```json
[
  {
    "mal_id": 50762,
    "title": "Example Anime Title"
  },
  {
    "mal_id": 51234,
    "title": "Another Missing Anime"
  }
]
```

## Override Files Schema

Override files allow manual correction of mappings when the automated process fails:

### Override Structure (`override_tv.json`, `override_movies.json`)

```typescript
interface Override {
  myanimelist: {
    title: string;      // Correct MAL title
    id: number;         // MAL ID
  };
  trakt: {
    title: string;      // Correct Trakt title
    id: number;         // Correct Trakt ID
    type: string;       // "shows" or "movies"
    season?: {          // For TV shows only
      number: number;   // Correct season number
    };
  };
}

type OverrideList = Override[];
```

## Usage

### Command Line Options

```bash
# Process TV shows
./db.trakt.extended-anitrakt -tv tv.json -key YOUR_TRAKT_API_KEY

# Process movies  
./db.trakt.extended-anitrakt -movies movies.json -key YOUR_TRAKT_API_KEY

# Process both with custom output
./db.trakt.extended-anitrakt -tv tv.json -movies movies.json -output custom_output.json

# Verbose mode
./db.trakt.extended-anitrakt -tv tv.json -verbose

# Disable progress bar
./db.trakt.extended-anitrakt -tv tv.json -no-progress
```

### Flags

- `-tv`: Input TV shows JSON file
- `-movies`: Input movies JSON file  
- `-output`: Custom output file name (optional)
- `-key`: Trakt.tv API key
- `-verbose`: Enable verbose logging
- `-no-progress`: Disable progress bar

### Environment Variables

You can set the Trakt API key via environment variable:

```bash
export TRAKT_API_KEY="your_api_key_here"
```

Or use a `.env` file:

```env
TRAKT_API_KEY=your_api_key_here
```

## Processing Logic

1. **Load Input**: Reads MAL anime data from JSON files
2. **Load Existing**: Checks for existing output to resume processing
3. **Load Not Found**: Loads previously identified missing entries to skip them
4. **Load Overrides**: Applies manual corrections from override files
5. **Fetch from Trakt**: Retrieves metadata from Trakt.tv API
6. **Enrich Data**: Combines MAL and Trakt information
7. **Save Results**: Outputs enriched data and updates not found lists

## Split Cour Detection

For TV shows, the application detects "split cour" anime (anime that spans multiple seasons):

- If the specified season exists on Trakt: `is_split_cour: false`
- If the season doesn't exist: `is_split_cour: true` and `season: null`

## Error Handling

- **404 Errors**: Entries not found on Trakt are added to not found files
- **Network Errors**: Logged and processing continues
- **Rate Limiting**: Built-in request delays to respect Trakt API limits

## Caching

The application uses temporary file caching to avoid redundant API calls:
- Show data cached in `/tmp/trakt_data/shows/`
- Movie data cached in `/tmp/trakt_data/movies/`
- Season data cached in `/tmp/trakt_data/seasons/`

Cache is automatically cleaned up when the application exits.

## File Naming Convention

- Input: `tv.json`, `movies.json`
- Output: `tv_ex.json`, `movies_ex.json` (or custom name via `-output`)
- Not Found: `not_exist_tv_ex.json`, `not_exist_movies_ex.json`
- Overrides: `override_tv.json`, `override_movies.json`

## Build Requirements

- Go 1.21 or higher
- Internet connection for Trakt.tv API access

## Dependencies

- `github.com/joho/godotenv` - Environment variable loading
- `github.com/schollz/progressbar/v3` - Progress indication
- `golang.org/x/term` - Terminal input handling