# Extended Trakt Database for db.trakt.anitrakt

An extended metadata layer on top of
[db.trakt.anitrakt](https://github.com/rensetsu/db.trakt.anitrakt) that calls
Trakt.tv for richer metadata and exposes it as structured JSON files.

## Overview

The tool takes JSON files containing anime titles with MAL IDs and Trakt IDs,
fetches additional metadata from Trakt.tv (seasons, external IDs, Letterboxd
links), and writes extended database files. A supplementary **Fribb-based
ingestion pipeline** can discover entries not present in the primary input by
cross-referencing [Fribb/anime-lists](https://github.com/Fribb/anime-lists)
with [AnimeAPI](https://animeapi.my.id).

## Output File Schemas

### Extended TV Shows Output (`tv_ex.json`)

```typescript
interface OutputShow {
  myanimelist: {
    title: string;             // MAL title
    id: number;                // MAL ID
  };
  trakt: {
    title: string;             // Trakt title
    id: number;                // Trakt ID
    slug: string;              // Trakt slug
    type: string;              // "shows"
    is_split_cour: boolean;    // Whether anime spans multiple seasons
    release_year: number;      // Year of release
    season: {                  // Season info (null when is_split_cour = true)
      id: number;              // Season ID on Trakt
      number: number;          // Season number
      externals: {
        tvdb: number | null;   // TVDB season ID
        tmdb: number | null;   // TMDB season ID
        tvrage: number | null; // TVRage season ID (deprecated)
      };
    } | null;
    externals: {
      tvdb: number | null;     // TVDB show ID
      tmdb: number | null;     // TMDB show ID
      imdb: string | null;     // IMDB show ID
      tvrage: number | null;   // TVRage show ID (deprecated)
    };
  };
}

type OutputShowList = OutputShow[];
```

### Extended Movies Output (`movies_ex.json`)

```typescript
interface OutputMovie {
  myanimelist: {
    title: string;           // MAL title
    id: number;              // MAL ID
  };
  trakt: {
    title: string;           // Trakt title
    id: number;              // Trakt ID
    slug: string;            // Trakt slug
    type: string;            // "movies"
    release_year: number;    // Year of release
    externals: {
      tmdb: number | null;   // TMDB movie ID
      imdb: string | null;   // IMDB movie ID
      letterboxd: {
        slug: string | null; // Letterboxd slug (used in URLs)
        lid: string | null;  // Letterboxd LID (documented API)
        uid: number | null;  // Letterboxd internal integer ID
      };
    };
  };
}

type OutputMovieList = OutputMovie[];
```

## Not Found Files Schema

Entries that cannot be found on Trakt.tv are logged separately:

### `not_exist_tv_ex.json` / `not_exist_movies_ex.json`

```typescript
interface NotFoundEntry {
  mal_id: number;   // MyAnimeList ID
  title: string;    // Anime title
}

type NotFoundList = NotFoundEntry[];
```

**Example:**
```json
[
  { "mal_id": 50762, "title": "Example Anime Title" },
  { "mal_id": 51234, "title": "Another Missing Anime" }
]
```

## Overrides

The override system lets you patch specific fields without touching the rest of
an entry. The application deep-merges overrides into existing data.

### Override Files

- `json/overrides/tv_overrides.json` — TV show corrections
- `json/overrides/movies_overrides.json` — Movie corrections

### Override Structure

| Field | Required | Description |
|-------|----------|-------------|
| `mal_id` | ✅ | MAL ID of the entry to modify |
| `description` | ✅ | Human-readable reason for the change |
| `trakt` | optional | Override Trakt title, id, or slug |
| `externals` | optional | Override external IDs (tvdb, tmdb, imdb, letterboxd…) |
| `ignore` | optional | Set `true` to skip this entry entirely |

### When to Use Overrides

**Submit upstream** (`rensetsu/db.trakt.anitrakt`):
- Incorrect Trakt ID mappings in input data
- Actual bugs affecting multiple users

**Use locally** (this repo):
- Mapping to external databases not in upstream
- Application-specific or site-specific tweaks

### Example — TV Overrides

```json
[
  {
    "mal_id": 5114,
    "description": "Custom Trakt mapping for this instance",
    "trakt": { "id": 3572, "slug": "bleach" }
  },
  {
    "mal_id": 11061,
    "description": "Local TVDB mapping override",
    "externals": { "tvdb": 395128 }
  },
  {
    "mal_id": 51234,
    "description": "Skip this entry - local filtering",
    "ignore": true
  }
]
```

### Example — Movie Overrides

```json
[
  {
    "mal_id": 1234,
    "description": "Custom TMDB mapping",
    "externals": { "tmdb": 12345 }
  }
]
```

## Usage

### Building

```bash
go mod tidy
go build
```

### Command Line Options

```bash
# Process TV shows
./db.trakt.extended-anitrakt -tv json/input/tv.json -api-key YOUR_TRAKT_API_KEY

# Process movies
./db.trakt.extended-anitrakt -movies json/input/movies.json -api-key YOUR_TRAKT_API_KEY

# Process both with a custom output path
./db.trakt.extended-anitrakt \
  -tv json/input/tv.json \
  -movies json/input/movies.json \
  -output json/output/custom_output.json

# Verbose mode
./db.trakt.extended-anitrakt -tv json/input/tv.json -verbose

# Force re-fetch everything, ignoring cache
./db.trakt.extended-anitrakt -tv json/input/tv.json -force

# Fribb ingestion — fetch source files from the internet automatically
./db.trakt.extended-anitrakt -fribb "" -api-key YOUR_TRAKT_API_KEY

# Fribb ingestion — use local copies of source files
./db.trakt.extended-anitrakt \
  -fribb /path/to/anime-lists-reduced.json \
  -animeapi /path/to/animeapi.tsv \
  -api-key YOUR_TRAKT_API_KEY

# Combine normal processing + Fribb supplementary ingestion
./db.trakt.extended-anitrakt \
  -tv json/input/tv.json \
  -movies json/input/movies.json \
  -fribb "" \
  -animeapi ""
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-tv` | — | Input TV shows JSON file |
| `-movies` | — | Input movies JSON file |
| `-output` | auto | Custom output file path |
| `-api-key` | — | Trakt.tv Client ID |
| `-verbose` | false | Enable verbose logging |
| `-no-progress` | false | Disable progress bar |
| `-force` | false | Ignore cache; re-fetch everything |
| `-fribb` | — | **Enable Fribb ingestion.** Path to `anime-lists-reduced.json`. Pass `""` to fetch from GitHub automatically. |
| `-animeapi` | — | Path to `animeapi.tsv` for Fribb ingestion. Pass `""` to fetch from `animeapi.my.id` automatically. |

> **Note:** `-fribb` and `-animeapi` must be **explicitly provided** (even as
> empty strings) to trigger Fribb ingestion. Simply omitting the flags will not
> run the Fribb pipeline.

### Environment Variables

```bash
export TRAKT_API_KEY="your_api_key_here"
```

Or use a `.env` file:

```env
TRAKT_API_KEY=your_api_key_here
```

## Processing Logic

### Primary Pipeline (`-tv` / `-movies`)

1. **Load Input** — Read MAL anime data from the specified JSON file
2. **Load Existing** — Read current output to resume interrupted runs
3. **Load Not Found** — Skip entries previously confirmed missing on Trakt
4. **Load Overrides** — Apply manual corrections from override files
5. **Fetch from Trakt** — Retrieve metadata via Trakt.tv API
6. **Enrich Data** — Combine MAL and Trakt data; resolve Letterboxd for movies
7. **Save Results** — Write enriched output and update not-found lists

> The Fribb pipeline always runs **after** `-tv` and `-movies`, so any entries
> added by the primary pipeline are already in the "existing" set and will be
> correctly skipped by Fribb.

## Fribb-based Ingestion Pipeline

A supplementary ingestion mode that discovers anime entries not present in the
primary `db.trakt.anitrakt` input by cross-referencing two community databases.

> [!IMPORTANT]
> **Why this is required:** The upstream project, anitrakt.huere.net, has been
> inactive since around Spring 2026 following the project maintainer's decision
> to discontinue operations. As a result, the primary input files are no longer
> updated, making this Fribb-based supplementary ingestion pipeline necessary
> to keep the database current with new anime releases, while it might be more
> inaccurate due to TVDB and aniDB dependency.

Triggered by passing `-fribb` (or `-animeapi`) on the command line.

### How It Works

1. **Load Fribb data** (`anime-lists-reduced.json`) — AniDB IDs mapped to TMDB
   TV/movie IDs, TVDB IDs, IMDB IDs, and season numbers.
2. **Load AnimeAPI TSV** (`animeapi.tsv`) — Maps AniDB IDs to MyAnimeList IDs.
3. **Cross-reference** — For each Fribb entry, resolve MAL ID via AnimeAPI.
4. **Filter existing** — Drop MAL IDs already in `tv_ex.json` /
   `movies_ex.json` or the not-found lists.
5. **Lookup on Trakt** — Search by the best available external ID:

   | Media | Primary | Fallback |
   |-------|---------|----------|
   | TV shows | TMDB TV ID (`/search/tmdb/:id?type=show`) | TVDB ID (`/search/tvdb/:id?type=show`) |
   | Movies | TMDB movie ID (`/search/tmdb/:id?type=movie`) | IMDB ID (`/search/imdb/:id?type=movie`) |

6. **Season enrichment** — For TV entries, fetch the Trakt season and run
   split-cour detection.
7. **Letterboxd enrichment** — For movies, resolve Letterboxd slug/LID/UID.
8. **Merge and save** — New entries are merged into the existing output files
   and sorted by MAL ID.

### Data Sources

| Source | Endpoint | Purpose |
|--------|----------|---------|
| [Fribb/anime-lists](https://github.com/Fribb/anime-lists) | `raw.githubusercontent.com/…/anime-lists-reduced.json` | AniDB → TMDB / TVDB / IMDB mapping |
| [AnimeAPI](https://animeapi.my.id) | `animeapi.my.id/animeapi.tsv` | AniDB → MAL ID mapping |

### Coverage (approximate, based on local data)

| Lookup type | Count |
|-------------|-------|
| TV via TMDB (primary) | ~6,830 |
| TV via TVDB (fallback) | ~166 |
| Movie via TMDB (primary) | ~618 |
| Movie via IMDB (fallback) | ~729 |

### Quirks and Caveats

- **Multiple IMDB IDs** — Fribb's `imdb_id` is sometimes a comma-separated
  list (e.g. one ID per OVA episode). Only the **first** value is used.
- **TMDB media type** — Determined by Fribb's `themoviedb_id.tv` vs
  `themoviedb_id.movie` sub-key. Entries with an empty `{}` object fall
  through to the TVDB/IMDB fallback.
- **Season 0 filtering** — Entries with `season.tmdb = 0` or `season.tvdb = 0`
  are automatically filtered out during processing to avoid unnecessary lookup
  errors, since season 0 (specials) often don't map cleanly to Trakt's data.
- **Movie-type sanity check** — When Fribb indicates a TV show (via TMDB TV ID or TVDB ID),
  the AnimeAPI title is checked for movie-related keywords in both English and Japanese
  (`movie`, `film`, `eiga`, `gekijouban`, `eigakan`, etc.). If found, the entry is skipped
  to prevent false TV mappings for actual theatrical releases.
- **TVDB season numbers** — When looking up via TVDB, the TVDB season number
  from Fribb is used as a best-effort approximation of the Trakt season number.
  They usually match but may diverge for older titles.
- **No-MAL entries** — AniDB IDs absent from AnimeAPI TSV are silently skipped.

## Split Cour Detection

The `is_split_cour` flag resolves discrepancies between how MAL and Trakt
number anime seasons that have a mid-season broadcast break.

| Value | Meaning |
|-------|---------|
| `false` | Season found on Trakt; `season` object is populated |
| `true` | Season not found — likely a split cour. `season` is `null` |

MAL often lists both halves of a split-cour series as separate seasons, while
TMDB/Trakt treat them as one continuous season when episode numbering doesn't
reset. When `is_split_cour: true`, the episodes for that "season" are likely
included under the **previous** season on Trakt.

### Note on Episode Counts

This flag only detects split cours, not episode-count mismatches. Some series
have different counts on MAL vs TMDB due to minisode grouping:

- *Uchitama?!*: [11 episodes](https://myanimelist.net/anime/39942) on MAL vs
  [28](https://www.themoviedb.org/tv/96660/season/1) on TMDB
- *Saiki K. S1*: [120 (minisodes)](https://myanimelist.net/anime/33255) on MAL
  vs [24](https://www.themoviedb.org/tv/67676/season/1) on TMDB

## Caching

| Cache location | Scope | Notes |
|----------------|-------|-------|
| `/tmp/trakt_data/shows/` | Ephemeral | Cleared after each run |
| `/tmp/trakt_data/movies/` | Ephemeral | Cleared after each run |
| `/tmp/trakt_data/seasons/` | Ephemeral | Cleared after each run |
| `/tmp/trakt_data/search/` | Ephemeral | Fribb external-ID search results |
| `/tmp/trakt_data/letterboxd/` | **Persistent** | Saved across GitHub Actions runs via cache |

Use `-force` to bypass all caches and re-fetch everything from the APIs.

## Error Handling

- **404 / no results** — Entry is added to the not-found file and skipped in
  future runs
- **Network errors** — Logged; processing continues with the next entry
- **Rate limiting** — Built-in request delays and exponential back-off respect
  Trakt API limits

## Change Tracking

Each run reports CRUD operations in a summary table:

| Metric | Description |
|--------|-------------|
| Created | New entries added |
| Updated | Existing entries with changed Trakt metadata |
| Modified (Overridden) | Entries patched by an override file |
| Not Found | Entries not found on Trakt.tv |

In GitHub Actions the summary is automatically written to
`$GITHUB_STEP_SUMMARY` as a markdown table with per-entry detail rows.

## File Structure

```
.
├── main.go
├── internal/
│   ├── api.go          # Trakt / Letterboxd API calls
│   ├── config.go       # CLI flag parsing
│   ├── file.go         # JSON load/save helpers
│   ├── fribb.go        # Fribb-based ingestion pipeline
│   ├── models.go       # Shared structs and Config
│   ├── processor.go    # Primary TV/movie processing
│   ├── ratelimit.go    # Token-bucket rate limiter
│   └── stats.go        # Progress and summary output
├── json/
│   ├── input/
│   │   ├── tv.json
│   │   └── movies.json
│   ├── output/
│   │   ├── tv_ex.json
│   │   └── movies_ex.json
│   ├── overrides/
│   │   ├── tv_overrides.json
│   │   └── movies_overrides.json
│   └── not_found/
│       ├── not_exist_tv_ex.json
│       └── not_exist_movies_ex.json
└── README.md
```

## Build Requirements

- Go 1.21+
- Internet connection for Trakt.tv (and optionally AnimeAPI / GitHub) access

## Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/joho/godotenv` | `.env` file loading |
| `github.com/schollz/progressbar/v3` | Progress bars |
| `golang.org/x/term` | Secure API key prompt |
