package fastly

import (
	"bytes"
	"fmt"
	"io"
	"net/url"
	"reflect"
	"strconv"
	"time"

	"github.com/google/jsonapi"
)

type SAUser struct {
	ID string `jsonapi:"primary,user"`
}

type SAService struct {
	ID string `jsonapi:"primary,service"`
}

type ServiceAuthorization struct {
	ID         string     `jsonapi:"primary,service_authorization"`
	Permission string     `jsonapi:"attr,permission,omitempty"`
	CreatedAt  *time.Time `jsonapi:"attr,created_at,iso8601"`
	UpdatedAt  *time.Time `jsonapi:"attr,updated_at,iso8601"`
	DeletedAt  *time.Time `jsonapi:"attr,deleted_at,iso8601"`
	User       *SAUser    `jsonapi:"relation,user,omitempty"`
	Service    *SAService `jsonapi:"relation,service,omitempty"`
}

// GetServiceAuthorizationInput is used as input to the GetServiceAuthorization function.
type GetServiceAuthorizationInput struct {
	// ID of the service authorization to retrieve.
	ID string
}

// GetServiceAuthorization retrieves an existing service authorization using its ID.
func (c *Client) GetServiceAuthorization(i *GetServiceAuthorizationInput) (*ServiceAuthorization, error) {
	if i.ID == "" {
		return nil, ErrMissingID
	}

	path := fmt.Sprintf("/service-authorizations/%s", i.ID)
	resp, err := c.Get(path, nil)
	if err != nil {
		return nil, err
	}

	var sa ServiceAuthorization
	if err := jsonapi.UnmarshalPayload(resp.Body, &sa); err != nil {
		return nil, err
	}

	return &sa, nil
}

// CreateServiceAuthorizationInput is used as input to the CreateServiceAuthorization function.
type CreateServiceAuthorizationInput struct {
	// ID value is ignored and should not be set, needed to make JSONAPI work correctly.
	ID string `jsonapi:"primary,service_authorization"`

	// Permission is the level of permissions to grant the user to the service. Valid values are "full", "read_only", "purge_select" or "purge_all".
	Permission string `jsonapi:"attr,permission,omitempty"`

	// ServiceID is the ID of the service to grant permissions for.
	Service *SAService `jsonapi:"relation,service,omitempty"`

	// UserID is the ID of the user which should have its permissions set.
	User *SAUser `jsonapi:"relation,user,omitempty"`
}

// CreateServiceAuthorization creates a new service authorization granting granular service and user permissions.
func (c *Client) CreateServiceAuthorization(i *CreateServiceAuthorizationInput) (*ServiceAuthorization, error) {
	if i.Service == nil || i.Service.ID == "" {
		return nil, ErrMissingServiceAuthorizationsService
	}
	if i.User == nil || i.User.ID == "" {
		return nil, ErrMissingServiceAuthorizationsUser
	}

	resp, err := c.PostJSONAPI("/service-authorizations", i, nil)
	if err != nil {
		return nil, err
	}

	var sa ServiceAuthorization
	if err := jsonapi.UnmarshalPayload(resp.Body, &sa); err != nil {
		return nil, err
	}

	return &sa, nil
}

// UpdateServiceAuthorizationInput is used as input to the UpdateServiceAuthorization function.
type UpdateServiceAuthorizationInput struct {
	// ID uniquely identifies the service authorization (service and user pair) to be updated.
	ID string `jsonapi:"primary,service_authorization"`

	// The permission to grant the user to the service referenced by this service authorization.
	Permissions string `jsonapi:"attr,permission,omitempty"`
}

// UpdateServiceAuthorization updates an exisitng service authorization. The ID must be known.
func (c *Client) UpdateServiceAuthorization(i *UpdateServiceAuthorizationInput) (*ServiceAuthorization, error) {
	if i.ID == "" {
		return nil, ErrMissingID
	}

	if i.Permissions == "" {
		return nil, ErrMissingPermissions
	}

	path := fmt.Sprintf("/service-authorizations/%s", i.ID)
	resp, err := c.PatchJSONAPI(path, i, nil)
	if err != nil {
		return nil, err
	}

	var sa ServiceAuthorization
	if err := jsonapi.UnmarshalPayload(resp.Body, &sa); err != nil {
		return nil, err
	}

	return &sa, nil
}

// DeleteServiceAuthorizationInput is used as input to the DeleteServiceAuthorization function.
type DeleteServiceAuthorizationInput struct {
	// ID of the service authorization to delete.
	ID string
}

// DeleteServiceAuthorization deletes an existing service authorization using the ID.
func (c *Client) DeleteServiceAuthorization(i *DeleteServiceAuthorizationInput) error {
	if i.ID == "" {
		return ErrMissingID
	}

	path := fmt.Sprintf("/service-authorizations/%s", i.ID)
	_, err := c.Delete(path, nil)

	return err
}

// ListServiceAuthorizationsInput is used as input to the ListServiceAuthorizations function.
type ListServiceAuthorizationsInput struct {
	PerPage int
	Page    int
}

// ListServiceAuthorizations returns the full list of service authorizations visible with the current API key.
func (c *Client) ListServiceAuthorizations(i *ListServiceAuthorizationsInput) ([]*ServiceAuthorization, error) {
	resp, err := c.Get("/service-authorizations", &RequestOptions{
		Headers: map[string]string{
			"Accept": "application/vnd.api+json",
		},
	})
	if err != nil {
		return nil, err
	}

	data, err := jsonapi.UnmarshalManyPayload(resp.Body, reflect.TypeOf(new(ServiceAuthorization)))
	if err != nil {
		return nil, err
	}

	s := make([]*ServiceAuthorization, len(data))
	for i := range data {
		typed, ok := data[i].(*ServiceAuthorization)
		if !ok {
			return nil, fmt.Errorf("unexpected response type: %T", data[i])
		}
		s[i] = typed
	}
	return s, nil
}

type ListServiceAuthorizationsPaginator struct {
	consumed    bool
	CurrentPage int
	NextPage    int
	LastPage    int
	client      *Client
	options     *ListServiceAuthorizationsInput
}

// HasNext returns a boolean indicating whether more pages are available
func (p *ListServiceAuthorizationsPaginator) HasNext() bool {
	return !p.consumed || p.Remaining() != 0
}

// Remaining returns the remaining page count
func (p *ListServiceAuthorizationsPaginator) Remaining() int {
	if p.LastPage == 0 {
		return 0
	}
	return p.LastPage - p.CurrentPage
}

// GetNext retrieves data in the next page
func (p *ListServiceAuthorizationsPaginator) GetNext() ([]*ServiceAuthorization, error) {
	return p.client.listServiceAuthorizationsWithPage(p.options, p)
}

// NewListServiceAuthorizationsPaginator returns a new paginator
func (c *Client) NewListServiceAuthorizationsPaginator(i *ListServiceAuthorizationsInput) PaginatorServiceAuthorizations {
	return &ListServiceAuthorizationsPaginator{
		client:  c,
		options: i,
	}
}

// listServiceAuthorizationsWithPage return a list of service authorizations
func (c *Client) listServiceAuthorizationsWithPage(i *ListServiceAuthorizationsInput, p *ListServiceAuthorizationsPaginator) ([]*ServiceAuthorization, error) {
	var perPage int
	const maxPerPage = 100
	if i.PerPage <= 0 {
		perPage = maxPerPage
	} else {
		perPage = i.PerPage
	}

	// page is not specified, fetch from the beginning
	if i.Page <= 0 && p.CurrentPage == 0 {
		p.CurrentPage = 1
	} else {
		// page is specified, fetch from a given page
		if !p.consumed {
			p.CurrentPage = i.Page
		} else {
			p.CurrentPage = p.CurrentPage + 1
		}
	}

	requestOptions := &RequestOptions{
		Params: map[string]string{
			"page[size]":   strconv.Itoa(perPage),
			"page[number]": strconv.Itoa(p.CurrentPage),
		},
		Headers: map[string]string{
			"Accept": "application/vnd.api+json",
		},
	}

	resp, err := c.Get("/service-authorizations", requestOptions)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	tee := io.TeeReader(resp.Body, &buf)

	info, err := getResponseInfo(tee)
	if err != nil {
		return nil, err
	}

	data, err := jsonapi.UnmarshalManyPayload(bytes.NewReader(buf.Bytes()), reflect.TypeOf(new(ServiceAuthorization)))
	if err != nil {
		return nil, err
	}

	s := make([]*ServiceAuthorization, len(data))
	for i := range data {
		typed, ok := data[i].(*ServiceAuthorization)
		if !ok {
			return nil, fmt.Errorf("unexpected response type: %T", data[i])
		}
		s[i] = typed
	}

	if l := info.Links.Next; l != "" {
		u, _ := url.Parse(l)
		query := u.Query()
		p.NextPage, _ = strconv.Atoi(query["page[number]"][0])
	}
	if l := info.Links.Last; l != "" {
		u, _ := url.Parse(l)
		query := u.Query()
		p.LastPage, _ = strconv.Atoi(query["page[number]"][0])
	}

	p.consumed = true

	return s, nil
}
