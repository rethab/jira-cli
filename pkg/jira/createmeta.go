package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
)

// CreateMetaRequest struct holds request data for createmeta request.
type CreateMetaRequest struct {
	Projects       string
	IssueTypeNames string
	Expand         string
}

// CreateMetaResponse struct holds response from GET /issue/createmeta endpoint.
type CreateMetaResponse struct {
	Projects []struct {
		Key        string                 `json:"key"`
		Name       string                 `json:"name"`
		IssueTypes []*CreateMetaIssueType `json:"issuetypes"`
	} `json:"projects"`
}

// CreateMetaResponseJiraServerV9 struct holds response from GET /issue/createmeta endpoint
// for jira server 9 and above.
type CreateMetaResponseJiraServerV9 struct {
	Values []struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Subtask bool   `json:"subtask"`
	} `json:"values"`
}

// CreateMetaIssueType struct holds issue types from GET /issue/createmeta endpoint.
type CreateMetaIssueType struct {
	IssueType
	Fields map[string]IssueTypeField `json:"fields"`
}

// GetCreateMeta gets create metadata using GET /issue/createmeta endpoint.
func (c *Client) GetCreateMeta(req *CreateMetaRequest) (*CreateMetaResponse, error) {
	path := fmt.Sprintf(
		"/issue/createmeta?projectKeys=%s&expand=%s",
		req.Projects, req.Expand,
	)
	if req.IssueTypeNames != "" {
		path += fmt.Sprintf("&issuetypeNames=%s", req.IssueTypeNames)
	}

	res, err := c.GetV2(context.Background(), path, nil)
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, ErrEmptyResponse
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		return nil, formatUnexpectedResponse(res)
	}

	var out CreateMetaResponse

	err = json.NewDecoder(res.Body).Decode(&out)

	return &out, err
}

// GetCreateMetaForJiraServerV9 gets create metadata using GET /issue/createmeta endpoint for jira server 9 and above.
func (c *Client) GetCreateMetaForJiraServerV9(req *CreateMetaRequest) (*CreateMetaResponseJiraServerV9, error) {
	path := fmt.Sprintf(
		"/issue/createmeta/%s/issuetypes?expand=%s",
		req.Projects, req.Expand,
	)
	if req.IssueTypeNames != "" {
		path += fmt.Sprintf("&issuetypeNames=%s", req.IssueTypeNames)
	}

	res, err := c.GetV2(context.Background(), path, nil)
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, ErrEmptyResponse
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		return nil, formatUnexpectedResponse(res)
	}

	var out CreateMetaResponseJiraServerV9

	err = json.NewDecoder(res.Body).Decode(&out)

	return &out, err
}

// createMetaPageSize is the largest page Jira Cloud serves for the createmeta endpoints.
const createMetaPageSize = 200

// CreateMetaAllowedValue is one allowed value of a field on the create screen. Options carry
// a value, system fields like priority or components a name, projects a key, and cascading
// selects children.
type CreateMetaAllowedValue struct {
	Value    string                   `json:"value,omitempty"`
	Key      string                   `json:"key,omitempty"`
	Name     string                   `json:"name,omitempty"`
	Children []CreateMetaAllowedValue `json:"children,omitempty"`
}

// CreateMetaField is a field on the create screen of an issue type.
type CreateMetaField struct {
	FieldID  string `json:"fieldId"`
	Key      string `json:"key"`
	Name     string `json:"name"`
	Required bool   `json:"required"`
	Schema   struct {
		DataType string `json:"type"`
		Items    string `json:"items,omitempty"`
	} `json:"schema"`
	AllowedValues []CreateMetaAllowedValue `json:"allowedValues,omitempty"`
}

// ID returns the id of the field, which is what `--custom` accepts as a key.
func (f *CreateMetaField) ID() string {
	if f.FieldID != "" {
		return f.FieldID
	}
	return f.Key
}

// createMetaPage decodes the paginated createmeta responses. Jira Cloud returns the lists under
// "issueTypes" and "fields", Jira Server under "values".
type createMetaPage struct {
	Total      int                `json:"total"`
	IssueTypes []IssueType        `json:"issueTypes"`
	Fields     []*CreateMetaField `json:"fields"`
	Values     json.RawMessage    `json:"values"`
}

// GetCreateMetaIssueTypes gets all issue types of a project using
// GET /issue/createmeta/{project}/issuetypes endpoint.
func (c *Client) GetCreateMetaIssueTypes(project string) ([]IssueType, error) {
	var out []IssueType

	path := "/issue/createmeta/" + url.PathEscape(project) + "/issuetypes"
	err := c.getCreateMetaPages(path, func(p *createMetaPage) (int, error) {
		page := p.IssueTypes
		if page == nil && len(p.Values) > 0 {
			if err := json.Unmarshal(p.Values, &page); err != nil {
				return 0, err
			}
		}
		out = append(out, page...)
		return len(page), nil
	})

	return out, err
}

// GetCreateMetaFields gets all fields on the create screen of an issue type using
// GET /issue/createmeta/{project}/issuetypes/{id} endpoint.
func (c *Client) GetCreateMetaFields(project, issueTypeID string) ([]*CreateMetaField, error) {
	var out []*CreateMetaField

	path := "/issue/createmeta/" + url.PathEscape(project) + "/issuetypes/" + url.PathEscape(issueTypeID)
	err := c.getCreateMetaPages(path, func(p *createMetaPage) (int, error) {
		page := p.Fields
		if page == nil && len(p.Values) > 0 {
			if err := json.Unmarshal(p.Values, &page); err != nil {
				return 0, err
			}
		}
		out = append(out, page...)
		return len(page), nil
	})

	return out, err
}

// getCreateMetaPages fetches pages until the reported total is reached. The page offset is
// advanced by what was received rather than by the requested size, since the server may cap it.
func (c *Client) getCreateMetaPages(path string, collect func(*createMetaPage) (int, error)) error {
	startAt := 0

	for {
		res, err := c.GetV2(context.Background(), fmt.Sprintf("%s?startAt=%d&maxResults=%d", path, startAt, createMetaPageSize), nil)
		if err != nil {
			return err
		}
		if res == nil {
			return ErrEmptyResponse
		}

		var page createMetaPage

		if res.StatusCode != http.StatusOK {
			err = formatUnexpectedResponse(res)
		} else {
			err = json.NewDecoder(res.Body).Decode(&page)
		}
		_ = res.Body.Close()

		if err != nil {
			return err
		}

		n, err := collect(&page)
		if err != nil {
			return err
		}

		startAt += n
		if n == 0 || startAt >= page.Total {
			return nil
		}
	}
}

// GetCreateFields gets the fields on the create screen of an issue type, matched by name
// case-insensitively. Jira Server before 8.5 only has the legacy createmeta endpoint, which
// is deprecated on Cloud, so callers choose between them with legacy.
func (c *Client) GetCreateFields(project, issueType string, legacy bool) ([]*CreateMetaField, error) {
	if legacy {
		return c.getCreateFieldsLegacy(project, issueType)
	}

	types, err := c.GetCreateMetaIssueTypes(project)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(types))
	for _, t := range types {
		if strings.EqualFold(t.Name, issueType) {
			return c.GetCreateMetaFields(project, t.ID)
		}
		names = append(names, t.Name)
	}

	return nil, unknownIssueTypeError(project, issueType, names)
}

type legacyCreateMetaResponse struct {
	Projects []struct {
		IssueTypes []struct {
			Name   string                      `json:"name"`
			Fields map[string]*CreateMetaField `json:"fields"`
		} `json:"issuetypes"`
	} `json:"projects"`
}

// getCreateFieldsLegacy fetches every issue type of the project rather than filtering with
// issuetypeNames, so an unknown name can be reported with the valid ones.
func (c *Client) getCreateFieldsLegacy(project, issueType string) ([]*CreateMetaField, error) {
	path := fmt.Sprintf("/issue/createmeta?projectKeys=%s&expand=projects.issuetypes.fields", url.QueryEscape(project))

	res, err := c.GetV2(context.Background(), path, nil)
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, ErrEmptyResponse
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		return nil, formatUnexpectedResponse(res)
	}

	var meta legacyCreateMetaResponse
	if err := json.NewDecoder(res.Body).Decode(&meta); err != nil {
		return nil, err
	}
	if len(meta.Projects) == 0 {
		return nil, fmt.Errorf("project %s not found", project)
	}

	var names []string

	for _, p := range meta.Projects {
		for _, t := range p.IssueTypes {
			if !strings.EqualFold(t.Name, issueType) {
				names = append(names, t.Name)
				continue
			}

			fields := make([]*CreateMetaField, 0, len(t.Fields))
			for id, f := range t.Fields {
				if f.FieldID == "" && f.Key == "" {
					f.Key = id
				}
				fields = append(fields, f)
			}
			// The server returns a map, so the order has to be made deterministic here.
			slices.SortFunc(fields, func(a, b *CreateMetaField) int { return strings.Compare(a.ID(), b.ID()) })

			return fields, nil
		}
	}

	return nil, unknownIssueTypeError(project, issueType, names)
}

func unknownIssueTypeError(project, issueType string, valid []string) error {
	return fmt.Errorf("issue type %q not found in project %s, valid types: %s", issueType, project, strings.Join(valid, ", "))
}
