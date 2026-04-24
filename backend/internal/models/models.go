package models

type AuthResponse struct {
	Token string      `json:"token"`
	User  UserSummary `json:"user"`
}

type UserSummary struct {
	ID          int    `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url,omitempty"`
	IsAdmin     bool   `json:"is_admin"`
}

type SharedFont struct {
	Name   string `json:"name"`
	Family string `json:"family"`
	URL    string `json:"url"`
}

type SystemSettings struct {
	AllowRegister bool   `json:"allow_register"`
	FooterExtra   string `json:"footer_extra"`
	Copyright     string `json:"copyright"`
}

type MCPTokenStatus struct {
	Configured bool   `json:"configured"`
	Hint       string `json:"hint"`
	Token      string `json:"token,omitempty"`
}

type UserPreferences struct {
	Language         string       `json:"language"`
	Theme            string       `json:"theme"`
	FontMode         string       `json:"font_mode"`
	DisplayName      string       `json:"display_name"`
	AvatarURL        string       `json:"avatar_url,omitempty"`
	CustomFontName   string       `json:"custom_font_name"`
	CustomFontFamily string       `json:"custom_font_family"`
	CustomFontURL    string       `json:"custom_font_url,omitempty"`
	AvailableFonts   []SharedFont `json:"available_fonts,omitempty"`
}

type DictionarySummary struct {
	ID           int      `json:"id"`
	Name         string   `json:"name"`
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	EntryCount   int      `json:"entry_count"`
	Enabled      bool     `json:"enabled"`
	Public       bool     `json:"public"`
	FileStatus   string   `json:"file_status"`
	MissingFiles []string `json:"missing_files"`
	MdxPath      string   `json:"mdx_path"`
	MddPaths     []string `json:"mdd_paths"`
	CreatedAt    string   `json:"created_at"`
	UpdatedAt    string   `json:"updated_at"`
	OwnerID      int      `json:"owner_id"`
	OwnerName    string   `json:"owner_name,omitempty"`
}

type SearchResult struct {
	DictionaryID   int     `json:"dictionary_id"`
	DictionaryName string  `json:"dictionary_name"`
	Visibility     string  `json:"visibility"`
	Word           string  `json:"word"`
	HTML           string  `json:"html"`
	Score          float64 `json:"score"`
	Source         string  `json:"source"`
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

type SearchSuggestionSource struct {
	DictionaryID   int    `json:"dictionary_id"`
	DictionaryName string `json:"dictionary_name"`
	Visibility     string `json:"visibility"`
	Source         string `json:"source"`
}

type SearchSuggestion struct {
	Word    string                   `json:"word"`
	Sources []SearchSuggestionSource `json:"sources"`
}

type SearchBackendDictionary struct {
	DictionaryID   int    `json:"dictionary_id"`
	DictionaryName string `json:"dictionary_name"`
	Visibility     string `json:"visibility"`
	FuzzyBackend   string `json:"fuzzy_backend"`
	PrefixBackend  string `json:"prefix_backend"`
	Loaded         bool   `json:"loaded"`
}

type SearchBackendDebug struct {
	RedisConfigured    bool                      `json:"redis_configured"`
	RedisSearchEnabled bool                      `json:"redis_search_enabled"`
	Scope              string                    `json:"scope"`
	Dictionaries       []SearchBackendDictionary `json:"dictionaries"`
}
