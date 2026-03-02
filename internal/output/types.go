package output

import "time"

type Page struct {
	ID             string
	Title          string
	URL            string
	CreatedTime    time.Time
	LastEditedTime time.Time
	ParentType     string
	ParentID       string
	Archived       bool
	Icon           string
	Content        string
}

type Database struct {
	ID             string
	Title          string
	URL            string
	CreatedTime    time.Time
	LastEditedTime time.Time
	Description    string
	Icon           string
}

type View struct {
	ID     string         `json:"id"`
	Name   string         `json:"name"`
	Layout string         `json:"layout"`
	URL    string         `json:"url"`
	Format map[string]any `json:"format,omitempty"`
}

type SearchResult struct {
	ID         string
	Type       string
	Title      string
	URL        string
	ParentType string
	ParentID   string
}

type Comment struct {
	ID             string
	DiscussionID   string
	CreatedTime    time.Time
	LastEditedTime time.Time
	CreatedBy      string
	Content        string
}
