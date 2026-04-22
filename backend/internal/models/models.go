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

type DictionarySummary struct {
	ID          int      `json:"id"`
	Name        string   `json:"name"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	EntryCount  int      `json:"entry_count"`
	Enabled     bool     `json:"enabled"`
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
	Word           string `json:"word"`
	HTML           string `json:"html"`
	Score          float64 `json:"score"`
	Source         string `json:"source"`
}
