package internal

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"path/filepath"
)

// LoadJSON loads JSON from a file, fatal on error
func LoadJSON(filename string, v interface{}) {
	file, err := os.Open(filename)
	if err != nil {
		log.Fatalf("Failed to open file %s: %v", filename, err)
	}
	defer file.Close()

	bytes, err := io.ReadAll(file)
	if err != nil {
		log.Fatalf("Failed to read file %s: %v", filename, err)
	}

	if err := json.Unmarshal(bytes, v); err != nil {
		log.Fatalf("Failed to unmarshal JSON from %s: %v", filename, err)
	}
}

// LoadJSONOptional loads JSON from a file, silent on error
func LoadJSONOptional(filename string, v interface{}) {
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return
	}

	file, err := os.Open(filename)
	if err != nil {
		log.Printf("Warning: Failed to open optional file %s: %v", filename, err)
		return
	}
	defer file.Close()

	bytes, err := io.ReadAll(file)
	if err != nil {
		log.Printf("Warning: Failed to read optional file %s: %v", filename, err)
		return
	}

	if err := json.Unmarshal(bytes, v); err != nil {
		log.Printf("Warning: Failed to unmarshal JSON from optional file %s: %v", filename, err)
	}
}

// SaveJSON saves data to a JSON file
func SaveJSON(filename string, v interface{}) {
	bytes, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal data for %s: %v", filename, err)
	}

	if err := os.WriteFile(filename, bytes, 0644); err != nil {
		log.Fatalf("Failed to write to file %s: %v", filename, err)
	}
}

// LoadNotFound loads the not found entries for an output file
func LoadNotFound(outputFile string) map[int]bool {
	notExistFile := filepath.Join("json/not_found", "not_exist_"+filepath.Base(outputFile))
	var notExist []NotFoundEntry
	LoadJSONOptional(notExistFile, &notExist)
	notExistMap := make(map[int]bool)
	for _, entry := range notExist {
		notExistMap[entry.MalID] = true
	}
	return notExistMap
}

// SaveNotFound saves not found entries
func SaveNotFound(outputFile string, newNotExist []NotFoundEntry, notExistMap map[int]bool) {
	if len(newNotExist) > 0 {
		notExistFile := filepath.Join("json/not_found", "not_exist_"+filepath.Base(outputFile))
		var existingNotExist []NotFoundEntry
		LoadJSONOptional(notExistFile, &existingNotExist)
		for _, entry := range newNotExist {
			if !notExistMap[entry.MalID] {
				existingNotExist = append(existingNotExist, entry)
			}
		}
		SaveJSON(notExistFile, existingNotExist)
	}
}

// SaveResults saves show results to file
func SaveResults(outputFile string, resultsMap map[int]OutputShow) {
	results := make([]OutputShow, 0, len(resultsMap))
	for _, show := range resultsMap {
		results = append(results, show)
	}
	// Sort by MAL ID
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[i].MyAnimeList.ID > results[j].MyAnimeList.ID {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
	SaveJSON(outputFile, results)
}

// SaveMovieResults saves movie results to file
func SaveMovieResults(outputFile string, resultsMap map[int]OutputMovie) {
	results := make([]OutputMovie, 0, len(resultsMap))
	for _, movie := range resultsMap {
		results = append(results, movie)
	}
	// Sort by MAL ID
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[i].MyAnimeList.ID > results[j].MyAnimeList.ID {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
	SaveJSON(outputFile, results)
}
