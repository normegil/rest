package rest

import (
	"math"
	"net/url"
	"strconv"

	urlFormats "github.com/normegil/formats/url"
	"github.com/pkg/errors"
)

type CollectionResponse struct {
	Current            urlFormats.URL
	First              urlFormats.URL
	Last               urlFormats.URL
	Previous           urlFormats.URL
	Next               urlFormats.URL
	Offset             int64
	Limit              int64
	TotalNumberOfItems int64
	Items              []interface{}
}

type CollectionResponseBuilder struct {
	BaseURL     url.URL
	MaxNBItems  int64
	QueryParams url.Values
	Items       []interface{}
}

func (b *CollectionResponseBuilder) WithBaseURI(baseURL url.URL) *CollectionResponseBuilder {
	b.BaseURL = baseURL
	return b
}

func (b *CollectionResponseBuilder) WithQueryParams(params url.Values) *CollectionResponseBuilder {
	b.QueryParams = params
	return b
}

func (b *CollectionResponseBuilder) WithMaxNumberOfItems(maxNbItems int64) *CollectionResponseBuilder {
	b.MaxNBItems = maxNbItems
	return b
}

func (b *CollectionResponseBuilder) WithItems(items []interface{}) *CollectionResponseBuilder {
	b.Items = items
	return b
}

func (b *CollectionResponseBuilder) Build() (*CollectionResponse, error) {
	pagination, err := loadPaginationInfo(b.QueryParams)
	if err != nil {
		return nil, errors.Wrapf(err, "Loading pagination info from url parameters")
	}
	current, err := collectionURL(b.BaseURL, pagination.Offset(), pagination.Limit(), b.QueryParams)
	if err != nil {
		return nil, err
	}
	first, err := collectionURL(b.BaseURL, 0, pagination.Limit(), b.QueryParams)
	if err != nil {
		return nil, err
	}
	last, err := generateLastURL(b.BaseURL, pagination.Offset(), pagination.Limit(), b.MaxNBItems, b.QueryParams)
	if err != nil {
		return nil, err
	}
	previous, err := generatePreviousURL(b.BaseURL, pagination.Offset(), pagination.Limit(), b.QueryParams)
	if err != nil {
		return nil, err
	}
	next, err := generateNextURL(b.BaseURL, pagination.Offset(), pagination.Limit(), b.MaxNBItems, b.QueryParams)
	if err != nil {
		return nil, err
	}
	return &CollectionResponse{
		Current:            urlFormats.URL{current},
		First:              urlFormats.URL{first},
		Last:               urlFormats.URL{last},
		Previous:           urlFormats.URL{previous},
		Next:               urlFormats.URL{next},
		Offset:             pagination.Offset(),
		Limit:              pagination.Limit(),
		TotalNumberOfItems: b.MaxNBItems,
		Items:              b.Items,
	}, nil
}

func generateLastURL(baseURL url.URL, offset int64, limit int64, maxNbItems int64, queryParams url.Values) (*url.URL, error) {
	if limit <= 0 {
		return &baseURL, nil
	}
	nbPagesMinusOne := maxNbItems / int64(limit)
	if maxNbItems%int64(limit) == 0 {
		nbPagesMinusOne -= 1
	}
	lastOffset := nbPagesMinusOne * int64(limit)
	return collectionURL(baseURL, lastOffset, limit, queryParams)
}

func generatePreviousURL(baseURL url.URL, offset int64, limit int64, queryParams url.Values) (*url.URL, error) {
	if limit <= 0 {
		return &baseURL, nil
	}
	if offset > 0 {
		var previousOffset int64
		if offset-int64(limit) < 0 {
			previousOffset = 0
		} else {
			previousOffset = offset - int64(limit)
		}
		return collectionURL(baseURL, previousOffset, limit, queryParams)
	}
	return &url.URL{}, nil
}

func generateNextURL(baseURL url.URL, offset int64, limit int64, maxNBItems int64, queryParams url.Values) (*url.URL, error) {
	if limit <= 0 {
		return &baseURL, nil
	}
	if int64(limit)+offset < maxNBItems {
		return collectionURL(baseURL, offset+int64(limit), limit, queryParams)
	}
	return &url.URL{}, nil
}

func collectionURL(baseURL url.URL, offset int64, limit int64, queryParams url.Values) (*url.URL, error) {
	collectionURL := baseURL.String()
	queryParams.Del("offset")
	queryParams.Del("limit")
	paramsStr := queryParams.Encode()
	if limit > 0 || paramsStr != "" {
		collectionURL += "?"
		if limit > 0 && limit != math.MaxInt64 {
			collectionURL += "offset=" + strconv.FormatInt(offset, 10) + "&limit=" + strconv.FormatInt(limit, 10)
		}
		if paramsStr != "" {
			collectionURL += paramsStr
		}
	}
	return url.Parse(collectionURL)
}

type Pagination struct {
	offset int64
	limit  int64
}

func (p Pagination) Limit() int64 {
	limit := p.limit
	if limit <= 0 {
		limit = math.MaxInt64
	}
	return limit
}

func (p *Pagination) SetLimit(limit int64) {
	p.limit = limit
}

func (p Pagination) Offset() int64 {
	return p.offset
}

func (p *Pagination) SetOffset(offset int64) {
	p.offset = offset
}

func loadPaginationInfo(params url.Values) (Pagination, error) {
	var err error
	offsetStr := params.Get("offset")
	var offset int64
	if "" != offsetStr {
		offset, err = strconv.ParseInt(offsetStr, 10, 64)
		if err != nil {
			return Pagination{}, errors.Wrapf(err, "Parsing offset '%s' into int64", offsetStr)
		}
	}

	limitStr := params.Get("limit")
	var limit int64
	if "" != limitStr {
		limit, err = strconv.ParseInt(limitStr, 10, 64)
		if err != nil {
			return Pagination{}, errors.Wrapf(err, "Parsing limit '%s' into int", limitStr)
		}
	}

	p := Pagination{}
	p.SetLimit(limit)
	p.SetOffset(offset)
	return p, nil
}
