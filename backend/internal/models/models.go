package models

type AuthResponse struct {
	Token string      `json:"token"`
	User  UserSummary `json:"user"`
}

type UserSummary struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	IsAdmin  bool   `json:"is_admin"`
}

type UserPreferences struct {
	Language         string `json:"language"`
	Theme            string `json:"theme"`
	FontMode         string `json:"font_mode"`
	CustomFontName   string `json:"custom_font_name"`
	CustomFontFamily string `json:"custom_font_family"`
	CustomFontURL    string `json:"custom_font_url,omitempty"`
}

type DictionarySummary struct {
	ID          int      `json:"id"`
	Name        string   `json:"name"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	EntryCount  int      `json:"entry_count"`
	Enabled     bool     `json:"enabled"`
	Public      bool     `json:"public"`
	FileStatus  string   `json:"file_status"`
	MissingFiles []string `json:"missing_files"`
	MdxPath     string   `json:"mdx_path"`
	MddPaths    []string `json:"mdd_paths"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
	OwnerID     int      `json:"owner_id"`
	OwnerName   string   `json:"owner_name,omitempty"`
}

type SearchResult struct {
	DictionaryID   int    `json:"dictionary_id"`
	DictionaryName string `json:"dictionary_name"`
	Visibility     string `json:"visibility"`
	Word           string `json:"word"`
	HTML           string `json:"html"`
	Score          float64 `json:"score"`
	Source         string `json:"source"`
}

type MaintenanceItemReport struct {
	DictionaryID int                `json:"dictionary_id,omitempty"`
	Name         string             `json:"name"`
	Action       string             `json:"action"`
	Status       string             `json:"status"`
	Message      string             `json:"message"`
	Dictionary   *DictionarySummary `json:"dictionary,omitempty"`
}

type MaintenanceReport struct {
	Summary    string                  `json:"summary"`
	Discovered int                     `json:"discovered"`
	Updated    int                     `json:"updated"`
	Skipped    int                     `json:"skipped"`
	Failed     int                     `json:"failed"`
	Items      []MaintenanceItemReport `json:"items"`
}

type SearchSuggestion struct {
	Word           string `json:"word"`
	DictionaryID   int    `json:"dictionary_id"`
	DictionaryName string `json:"dictionary_name"`
	Visibility     string `json:"visibility"`
	Source         string `json:"source"`
}
