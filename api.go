package notion

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
)

const (
	notionHost = "https://www.notion.so"
	// modern Chrome
	userAgent  = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_13_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/69.0.3483.0 Safari/537.36"
	acceptLang = "en-US,en;q=0.9"
)

// HTTPInterceptor allows intercepting HTTP request so that a client
// of this library can provide e.g. a caching system for requests
// instead
type HTTPInterceptor interface {
	// OnRequest is called before http request is sent tot he server
	// If it returns non-nil response, it'll be used instead of sending
	// a request to the server
	OnReqeust(*http.Request) *http.Response
	// OnResponse is called after getting a response from the server
	// to allow e.g. caching of responses
	// Only called if the request was sent to the server (i.e. doesn't come
	// from OnRequest)
	OnResponse(*http.Response)
}

var (
	// HTTPIntercept allows intercepting http requests
	// e.g. to implement caching
	HTTPIntercept HTTPInterceptor
)

// PageInfo describes a single Notion page
type PageInfo struct {
	ID   string
	Page *Block
	// Users allows to find users that Page refers to by their ID
	Users *NotionUser
}

func doNotionAPI(apiURL string, requestData interface{}, parseFn func(d []byte) error) error {
	var js []byte
	var err error
	if requestData != nil {
		js, err = json.Marshal(requestData)
		if err != nil {
			return err
		}
	}
	uri := notionHost + apiURL
	body := bytes.NewBuffer(js)
	log("POST %s\n", uri)
	if len(js) > 0 {
		log("%s\n", string(js))
	}

	req, err := http.NewRequest("POST", uri, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept-Language", acceptLang)

	var rsp *http.Response
	if HTTPIntercept != nil {
		rsp = HTTPIntercept.OnReqeust(req)
	}

	realHTTPRequest := false
	if rsp == nil {
		realHTTPRequest = true
		rsp, err = http.DefaultClient.Do(req)
	}

	if err != nil {
		log("http.DefaultClient.Do() failed with %s\n", err)
		return err
	}
	if HTTPIntercept != nil && realHTTPRequest {
		HTTPIntercept.OnResponse(rsp)
	}
	if rsp.StatusCode != 200 {
		log("Error: status code %d\n", rsp.StatusCode)
		return fmt.Errorf("http.Post('%s') returned non-200 status code of %d", uri, rsp.StatusCode)
	}
	defer rsp.Body.Close()
	d, err := ioutil.ReadAll(rsp.Body)
	if err != nil {
		log("Error: ioutil.ReadAll() failed with %s\n", err)
		return err
	}
	logJSON(d)
	err = parseFn(d)
	if err != nil {
		log("Error: json.Unmarshal() failed with %s\n", err)
	}
	return err
}

func apiLoadPageChunk(pageID string, cur *cursor) (*loadPageChunkResponse, error) {
	// emulating notion's website api usage: 50 items on first request,
	// 30 on subsequent requests
	limit := 30
	apiURL := "/api/v3/loadPageChunk"
	if cur == nil {
		cur = &cursor{
			// to mimic browser api which sends empty array for this argment
			Stack: make([][]stack, 0),
		}
		limit = 50
	}
	req := &loadPageChunkRequest{
		PageID:          pageID,
		Limit:           limit,
		Cursor:          *cur,
		VerticalColumns: false,
	}
	var rsp *loadPageChunkResponse
	parse := func(d []byte) error {
		var err error
		rsp, err = parseLoadPageChunk(d)
		return err
	}
	err := doNotionAPI(apiURL, req, parse)
	if err != nil {
		return nil, err
	}
	return rsp, nil
}

func getFirstInline(inline []*InlineBlock) string {
	if len(inline) == 0 {
		return ""
	}
	return inline[0].Text
}

func getFirstInlineBlock(v interface{}) (string, error) {
	inline, err := parseInlineBlocks(v)
	if err != nil {
		return "", err
	}
	return getFirstInline(inline), nil
}

func parseProperties(block *Block) error {
	var err error
	props := block.Properties

	if title, ok := props["title"]; ok {
		if block.Type == TypePage {
			block.Title, err = getFirstInlineBlock(title)
		} else if block.Type == TypeCode {
			block.Code, err = getFirstInlineBlock(title)
		} else {
			block.InlineContent, err = parseInlineBlocks(title)
		}
		if err != nil {
			return err
		}
	}

	if TypeTodo == block.Type {
		if checked, ok := props["checked"]; ok {
			s, _ := getFirstInlineBlock(checked)
			// fmt.Printf("checked: '%s'\n", s)
			block.IsChecked = strings.EqualFold(s, "Yes")
		}
	}

	if description, ok := props["description"]; ok {
		// for TypeBookmark
		block.Description, err = getFirstInlineBlock(description)
		if err != nil {
			return err
		}
	}

	if link, ok := props["link"]; ok {
		// for TypeBookmark
		block.Link, err = getFirstInlineBlock(link)
		if err != nil {
			return err
		}
	}

	if source, ok := props["source"]; ok {
		// for TypeBookmark, TypeImage, TypeGist
		block.Source, err = getFirstInlineBlock(source)
		if err != nil {
			return err
		}

		if block.IsImage() {
			// sometimes image url in "source" is not accessible but can
			// be accessed when proxied via notion server as
			// www.notion.so/image/${source}
			// This also allows resizing via ?width=${n} arguments
			block.ImageURL = "https://www.notion.so/image/" + url.PathEscape(block.Source)
		}
	}

	if language, ok := props["language"]; ok {
		block.CodeLanguage, err = getFirstInlineBlock(language)
		if err != nil {
			return err
		}
	}

	return nil
}

func parseFormat(block *Block) error {
	switch block.Type {
	case TypePage:
		// it's only present when different than defaults
		if len(block.FormatRaw) == 0 {
			block.FormatPage = &FormatPage{}
			return nil
		}
		var format FormatPage
		err := json.Unmarshal(block.FormatRaw, &format)
		if err != nil {
			fmt.Printf("parseFormat: json.Unamrshal() failed with '%s', format: '%s'\n", err, string(block.FormatRaw))
			return err
		}
		block.FormatPage = &format
	}
	return nil
}

// convert 2131b10c-ebf6-4938-a127-7089ff02dbe4 to 2131b10cebf64938a1277089ff02dbe4
func normalizeID(s string) string {
	return strings.Replace(s, "-", "", -1)
}

// TODO: non-recusrive version of revolveBlocks but doesn't work?
func resolveBlocks2(block *Block, idToBlock map[string]*Block) error {
	resolved := map[string]struct{}{}

	toResolve := []*Block{block}

	for len(toResolve) > 0 {
		block := toResolve[0]
		toResolve = toResolve[1:]
		id := normalizeID(block.ID)
		if _, ok := resolved[id]; ok {
			continue
		}
		resolved[id] = struct{}{}

		err := parseProperties(block)
		if err != nil {
			return err
		}

		n := len(block.ContentIDs)
		if n == 0 {
			continue
		}
		block.Content = make([]*Block, n, n)
		for i, id := range block.ContentIDs {
			b := idToBlock[id]
			if b == nil {
				return fmt.Errorf("Couldn't resolve block with id '%s'", id)
			}
			block.Content[i] = b
			skip := false
			switch b.Type {
			case TypePage:
				skip = true
			}
			if !skip {
				toResolve = append(toResolve, b)
			}
		}
	}
	return nil
}

func resolveBlocks(block *Block, idToBlock map[string]*Block) error {
	err := parseProperties(block)
	if err != nil {
		return err
	}
	err = parseFormat(block)
	if err != nil {
		return err
	}

	if block.Content != nil || len(block.ContentIDs) == 0 {
		return nil
	}
	n := len(block.ContentIDs)
	block.Content = make([]*Block, n, n)
	for i, id := range block.ContentIDs {
		resolved := idToBlock[id]
		if resolved == nil {
			return fmt.Errorf("Couldn't resolve block with id '%s'", id)
		}
		block.Content[i] = resolved
		resolveBlocks(resolved, idToBlock)
	}
	return nil
}

func apiGetRecordValues(ids []string) (*getRecordValuesResponse, error) {
	req := &getRecordValuesRequest{}

	for _, id := range ids {
		v := getRecordValuesRequestInner{
			Table: TableBlock,
			ID:    id,
		}
		req.Requests = append(req.Requests, v)
	}

	apiURL := "/api/v3/getRecordValues"
	var rsp *getRecordValuesResponse
	parse1 := func(d []byte) error {
		var err error
		rsp, err = parseGetRecordValues(d)
		return err
	}
	err := doNotionAPI(apiURL, req, parse1)
	if err != nil {
		return nil, err
	}
	return rsp, nil
}

func findMissingBlocks(startIds []string, idToBlock map[string]*Block) []string {
	var missing []string
	seen := map[string]struct{}{}
	toCheck := append([]string{}, startIds...)
	for len(toCheck) > 0 {
		id := toCheck[0]
		toCheck = toCheck[1:]
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		block := idToBlock[id]
		if block == nil {
			missing = append(missing, id)
			continue
		}

		// we don't have the content of this block.
		// get it unless this is a page block becuase this is only
		// a link to a page
		switch block.Type {
		case TypePage:
		// skip those blocks
		default:
			toCheck = append(toCheck, block.ContentIDs...)
		}
	}
	return missing
}

// GetPageInfo returns Noion page data given its id
func GetPageInfo(pageID string) (*PageInfo, error) {
	// TODO: validate pageID?

	var pageInfo PageInfo
	{
		recVals, err := apiGetRecordValues([]string{pageID})
		if err != nil {
			return nil, err
		}
		pageID = recVals.Results[0].Value.ID
		pageInfo.ID = pageID
		pageInfo.Page = recVals.Results[0].Value
	}

	idToBlock := map[string]*Block{}
	var cur *cursor
	for {
		rsp, err := apiLoadPageChunk(pageID, cur)
		if err != nil {
			return nil, err
		}
		for id, blockWithRole := range rsp.RecordMap.Blocks {
			idToBlock[id] = blockWithRole.Value
		}
		cursor := rsp.Cursor
		//dbg("GetPageInfo: len(cursor.Stack)=%d\n", len(cursor.Stack))
		if len(cursor.Stack) == 0 {
			break
		}
		cur = &rsp.Cursor
	}

	// get blocks that are not already loaded
	missing := findMissingBlocks(pageInfo.Page.ContentIDs, idToBlock)
	// the API worked even with 6k items, but I'll split it into many
	// smaller requrests anyway
	maxToGet := 128
	for len(missing) > 0 {
		//dbg("GetPageInfo: there are %d missing blocks\n", len(missing))
		toGet := missing
		if len(toGet) > maxToGet {
			toGet = missing[:maxToGet]
			missing = missing[maxToGet:]
		} else {
			missing = nil
		}

		recVals, err := apiGetRecordValues(toGet)
		if err != nil {
			return nil, err
		}
		for _, blockWithRole := range recVals.Results {
			block := blockWithRole.Value
			id := block.ID
			idToBlock[id] = block
		}
	}

	err := resolveBlocks(pageInfo.Page, idToBlock)
	if err != nil {
		return nil, err
	}
	return &pageInfo, nil
}