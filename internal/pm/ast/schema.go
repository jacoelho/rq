package ast

import (
	"encoding/json"
	"fmt"
	"io"
)

// ErrDecode indicates collection JSON decoding failures.
var ErrDecode = fmt.Errorf("collection decode error")

// Collection is the top-level collection export format.
type Collection struct {
	Info  Info    `json:"info"`
	Event []Event `json:"event"`
	Item  []Item  `json:"item"`
}

// Info carries collection metadata.
type Info struct {
	Name   string `json:"name"`
	Schema string `json:"schema"`
}

// Item is either a folder (with nested item) or a request item.
type Item struct {
	Name    string   `json:"name"`
	Item    []Item   `json:"item"`
	Request *Request `json:"request"`
	Event   []Event  `json:"event"`
}

// Request defines a source HTTP request.
type Request struct {
	Method    string          `json:"method"`
	Header    []Header        `json:"header"`
	URL       URLValue        `json:"url"`
	URLObject *URLObject      `json:"urlObject"`
	Body      *Body           `json:"body"`
	Auth      json.RawMessage `json:"auth"`
}

// EffectiveURL merges all known URL representations.
func (r Request) EffectiveURL() URLObject {
	resolved := URLObject{
		Raw:      r.URL.Raw,
		Protocol: r.URL.Protocol,
		Port:     r.URL.Port,
		Host:     append([]string(nil), r.URL.Host...),
		Path:     append([]string(nil), r.URL.Path...),
		Query:    append([]QueryParam(nil), r.URL.Query...),
	}

	if r.URLObject != nil {
		if resolved.Raw == "" {
			resolved.Raw = r.URLObject.Raw
		}
		if resolved.Protocol == "" {
			resolved.Protocol = r.URLObject.Protocol
		}
		if resolved.Port == "" {
			resolved.Port = r.URLObject.Port
		}
		if len(resolved.Host) == 0 {
			resolved.Host = append([]string(nil), r.URLObject.Host...)
		}
		if len(resolved.Path) == 0 {
			resolved.Path = append([]string(nil), r.URLObject.Path...)
		}
		if len(resolved.Query) == 0 {
			resolved.Query = append([]QueryParam(nil), r.URLObject.Query...)
		}
	}

	return resolved
}

// Header defines a request header.
type Header struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	Disabled bool   `json:"disabled"`
}

// QueryParam defines a URL query parameter.
type QueryParam struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	Disabled bool   `json:"disabled"`
}

// URLObject is the structured URL representation.
type URLObject struct {
	Raw      string       `json:"raw"`
	Protocol string       `json:"protocol"`
	Port     string       `json:"port"`
	Host     []string     `json:"host"`
	Path     []string     `json:"path"`
	Query    []QueryParam `json:"query"`
}

// URLValue supports both string and object URL input forms.
type URLValue struct {
	Raw      string
	Protocol string
	Port     string
	Host     []string
	Path     []string
	Query    []QueryParam
}

func (u *URLValue) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		*u = URLValue{}
		return nil
	}

	if data[0] == '"' {
		var raw string
		if err := json.Unmarshal(data, &raw); err != nil {
			return fmt.Errorf("decode URL string: %w", err)
		}
		*u = URLValue{Raw: raw}
		return nil
	}

	var object URLObject
	if err := json.Unmarshal(data, &object); err != nil {
		return fmt.Errorf("decode URL object: %w", err)
	}

	*u = URLValue(object)

	return nil
}

// Body defines supported request body forms.
type Body struct {
	Mode       string    `json:"mode"`
	Raw        string    `json:"raw"`
	URLEncoded []BodyKV  `json:"urlencoded"`
	FormData   []BodyKV  `json:"formdata"`
	File       *BodyFile `json:"file"`
}

// BodyKV is a key/value entry for form-like body payloads.
type BodyKV struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	Type     string `json:"type"`
	Disabled bool   `json:"disabled"`
}

// BodyFile defines file-mode body input metadata.
type BodyFile struct {
	Src string `json:"src"`
}

// Event represents request scripts/hooks.
type Event struct {
	Listen string `json:"listen"`
	Script Script `json:"script"`
}

// Script holds executable source lines.
type Script struct {
	Exec []string `json:"exec"`
}

// Parse reads collection JSON into the schema model.
func Parse(r io.Reader) (Collection, error) {
	decoder := json.NewDecoder(r)

	var collection Collection
	if err := decoder.Decode(&collection); err != nil {
		return Collection{}, fmt.Errorf("%w: %v", ErrDecode, err)
	}

	return collection, nil
}
