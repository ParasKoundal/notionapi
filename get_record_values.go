package notion

import "encoding/json"

// /api/v3/getRecordValues request
type getRecordValuesRequest struct {
	Requests []getRecordValuesRequestInner `json:"requests"`
}

type getRecordValuesRequestInner struct {
	Table string `json:"table"`
	ID    string `json:"id"`
}

// /api/v3/getRecordValues response
// Note: it depends on Table type in request
type getRecordValuesResponse struct {
	Results []*BlockWithRole `json:"results"`
}

// BlockWithRole describes a block info
type BlockWithRole struct {
	Role  string `json:"role"`
	Value *Block `json:"value"`
}

// Block describes a block
type Block struct {
	// values that come from JSON
	// if false, the page is deleted
	Alive bool `json:"alive"`
	// List of block ids for that make up content of this block
	// Use Content to get corresponding block (they are in the same order)
	ContentIDs []string `json:"content,omitempty"`
	// ID of the user who created this block
	CreatedBy   string `json:"created_by"`
	CreatedTime int64  `json:"created_time"`
	// List of block ids with discussion content
	DiscussionIDs []string `json:"discussion,omitempty"`
	// those ids seem to map to storage in s3
	// https://s3-us-west-2.amazonaws.com/secure.notion-static.com/${id}/${name}
	FileIDs   []string        `json:"file_ids,omitempty"`
	FormatRaw json.RawMessage `json:"format,omitempty"`
	// a unique ID of the block
	ID string `json:"id"`
	// ID of the user who last edited this block
	LastEditedBy   string `json:"last_edited_by"`
	LastEditedTime int64  `json:"last_edited_time"`
	// ID of parent Block
	ParentID    string `json:"parent_id"`
	ParentTable string `json:"parent_table"`
	// not always available
	Permissions *[]Permission          `json:"permissions,omitempty"`
	Properties  map[string]interface{} `json:"properties"`
	// type of the block e.g. TypeText, TypePage etc.
	Type string `json:"type"`
	// blocks are versioned
	Version int64 `json:"version"`

	// Values calculated by us
	// maps ContentIDs array
	Content []*Block `json:"content_resolved,omitempty"`
	// this is for some types like TypePage, TypeText, TypeHeader etc.
	InlineContent []*InlineBlock `json:"inline_content,omitempty"`

	// for TypePage
	Title string `json:"title,omitempty"`
	// For TypeTodo, a checked state
	IsChecked bool `json:"is_checked"`

	// for TypeBookmark
	Description string `json:"description,omitempty"`
	Link        string `json:"link,omitempty"`

	// for TypeGist it's the url for the gist
	// for TypeBookmark it's the url of the page
	// fot TypeImage it's url of the image, but use ImageURL instead
	// because Source is sometimes not accessible
	Source string `json:"source,omitempty"`

	// for TypeImage it's
	ImageURL string `json:"image_url,omitempty"`

	// for TypeCode
	Code         string `json:"code,omitempty"`
	CodeLanguage string `json:"code_language,omitempty"`

	FormatPage     *FormatPage     `json:"format_page,omitempty"`
	FormatBookmark *FormatBookmark `json:"format_bookmark,omitempty"`
	FormatImage    *FormatImage    `json:"format_image,omitempty"`
	FormatColumn   *FormatColumn   `json:"format_column,omitempty"`
	FormatText     *FormatText     `json:"format_text,omitempty"`
}

// IsLinkToPage returns true if block element is a link to existing page
// (as opposed to )
func (b *Block) IsLinkToPage() bool {
	if b.Type != TypePage {
		return false
	}
	return b.ParentTable == TableSpace
}

// IsPage returns true if block represents a page (either a sub-page or
// a link to a page)
func (b *Block) IsPage() bool {
	return b.Type == TypePage
}

// IsImage returns true if block represents an image
func (b *Block) IsImage() bool {
	return b.Type == TypeImage
}

// IsCode returns true if block represents a code block
func (b *Block) IsCode() bool {
	return b.Type == TypeCode
}

// FormatPage describes format for TypePage
type FormatPage struct {
	// /images/page-cover/gradients_11.jpg
	PageCoverRelativeURL string `json:"page_cover"`
	// e.g. 0.6
	PageCoverPosition float64 `json:"page_cover_position"`
	PageFullWidth     bool    `json:"page_full_width"`
	// it's url like https://s3-us-west-2.amazonaws.com/secure.notion-static.com/8b3930e3-9dfe-4ba7-a845-a8ff69154f2a/favicon-256.png
	// or emoji like "✉️"
	PageIcon      string `json:"page_icon"`
	PageSmallText bool   `json:"page_small_text"`
}

// FormatBookmark describes format for TypeBookmark
type FormatBookmark struct {
	BookmarkIcon string `json:"bookmark_icon"`
}

// FormatImage describes format for TypeImage
type FormatImage struct {
	BlockAspectRatio   float64 `json:"block_aspect_ratio"`
	BlockFullWidth     bool    `json:"block_full_width"`
	BlockPageWidth     bool    `json:"block_page_width"`
	BlockPreserveScale bool    `json:"block_preserve_scale"`
	BlockWidth         float64 `json:"block_width"`
	DisplaySource      string  `json:"display_source"`
}

// FormatText describes format for TypeText
// TODO: possibly more?
type FormatText struct {
	BlockColor *string `json:"block_color,omitempty"`
}

// FormatColumn describes format for TypeColumn
type FormatColumn struct {
	ColumnRation float64 `json:"column_ratio"` // e.g. 0.5 for half-sized column
}

// Permission describes user permissions
type Permission struct {
	Role   string  `json:"role"`
	Type   string  `json:"type"`
	UserID *string `json:"user_id,omitempty"`
}

func parseGetRecordValues(d []byte) (*getRecordValuesResponse, error) {
	var rec getRecordValuesResponse
	err := json.Unmarshal(d, &rec)
	if err != nil {
		dbg("parseGetRecordValues: json.Unmarshal() failed with '%s'\n", err)
		return nil, err
	}
	return &rec, nil
}
