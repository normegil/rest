package rest

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"

	"github.com/julienschmidt/httprouter"
	"github.com/normegil/resterrors"
	"github.com/pkg/errors"
)

type Controller interface {
	Routes() []Route
}

type DefaultController struct {
	DAO           DAO
	DefinedRoutes []Route
	EmptyInstance IdentifiableEntity
	ErrorHandler  resterrors.Handler
	Logger        Logger
}

const keyIdentifier = "id"

func NewController(basePath string, dao DAO, errorHandler resterrors.Handler, emptyEntity IdentifiableEntity) *DefaultController {
	c := &DefaultController{
		DAO:           dao,
		EmptyInstance: emptyEntity,
		ErrorHandler:  errorHandler,
	}
	routes := []Route{
		NewRoute("GET", "/"+basePath, c.GetAll),
		NewRoute("GET", "/"+basePath+"/:"+keyIdentifier, c.Get),
		NewRoute("PUT", "/"+basePath, c.Update),
		NewRoute("DELETE", "/"+basePath+"/:"+keyIdentifier, c.Delete),
	}
	c.DefinedRoutes = routes
	return c
}

func (c *DefaultController) Routes() []Route {
	return c.DefinedRoutes
}

func (c *DefaultController) GetAll(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	params := r.URL.Query()

	pagination, err := loadPaginationInfo(params)
	if err != nil {
		c.Handle(w, errors.Wrapf(err, "Load Pagination informations from query params"))
		return
	}

	var expand bool
	expandStr := params.Get("expand")
	if "" != expandStr {
		expand, err = strconv.ParseBool(expandStr)
		if err != nil {
			c.Handle(w, errors.Wrapf(err, "Parsing expand flag from '%s'", expandStr))
			return
		}
	}

	baseURL, err := getBaseURL(r)
	if err != nil {
		c.Handle(w, errors.Wrapf(err, "Constructing base url from request"))
		return
	}
	var items []interface{}
	if expand {
		entities, err := c.DAO.GetAllEntities(pagination)
		if err != nil {
			c.Handle(w, errors.Wrapf(err, "Get all entities {offset:%+v;limit:%+v}", pagination.Offset, pagination.Limit))
			return
		}
		for _, entity := range entities {
			items = append(items, entity)
		}
	} else {
		ids, err := c.DAO.GetAllIDs(pagination)
		if err != nil {
			c.Handle(w, errors.Wrapf(err, "Get all ids {offset:%+v;limit:%+v}", pagination.Offset, pagination.Limit))
		}
		for _, id := range ids {
			items = append(items, baseURL.String()+"/"+id.String())
		}
	}

	nbEntity, err := c.DAO.TotalNumberOfEntities()
	if err != nil {
		c.Handle(w, errors.Wrapf(err, "Get total number of entities"))
		return
	}

	respBuilder := &CollectionResponseBuilder{}
	response, err := respBuilder.WithBaseURI(*baseURL).WithItems(items).WithMaxNumberOfItems(nbEntity).WithQueryParams(params).Build()
	if err != nil {
		c.Handle(w, errors.Wrapf(err, "Building collection response"))
		return
	}
	responseBytes, err := json.Marshal(response)
	if err != nil {
		c.Handle(w, errors.Wrapf(err, "Encoding response '%+v'", response))
		return
	}
	fmt.Fprintf(w, string(responseBytes))
}

func (c *DefaultController) Get(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	id := params.ByName("id")
	entity, err := c.DAO.Get(StringIdentifier(id))
	if err != nil {
		c.Handle(w, errors.Wrapf(err, "Get entity with id '%+v'", id))
		return
	}
	jsonEntity, err := json.Marshal(entity)
	if err != nil {
		c.Handle(w, errors.Wrapf(err, "Encoding entity into json '%+v'", entity))
		return
	}
	_, err = fmt.Fprint(w, jsonEntity)
	if err != nil {
		c.Handle(w, errors.Wrapf(err, "Writing entity as response '%s'", string(jsonEntity)))
		return
	}
	return
}

func (c *DefaultController) Update(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	// Copy EmptyInstance to not modify it and only use it as reference
	instance := c.EmptyInstance
	bodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		c.Handle(w, errors.Wrapf(err, "Reading body"))
		return
	}
	err = json.Unmarshal(bodyBytes, &instance)
	if err != nil {
		c.Handle(w, errors.Wrapf(err, "Unmarshal %s", string(bodyBytes)))
		return
	}
	id, err := c.DAO.Set(instance)
	if err != nil {
		c.Handle(w, errors.Wrapf(err, "Update '%+v'", instance))
		return
	}
	baseURL, err := getBaseURL(r)
	if err != nil {
		c.Handle(w, errors.Wrapf(err, "Get base request url from request"))
		return
	}
	fmt.Fprintf(w, baseURL.String()+"/%s", id.String())
	return
}

func (c *DefaultController) Delete(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	id := params.ByName("id")
	err := c.DAO.Delete(StringIdentifier(id))
	if err != nil {
		c.Handle(w, errors.Wrapf(err, "Deleting %s", id))
		return
	}
	return
}

func (c *DefaultController) Handle(w http.ResponseWriter, err error) {
	e := c.ErrorHandler.Handle(w, err)
	if nil != e {
		newErr := errors.Wrapf(e, "Error while writing error response -{ %s }-", err.Error())
		c.log(newErr.Error())
		fmt.Fprintf(w, newErr.Error())
	} else {
		c.log(err.Error())
	}
}

func (c *DefaultController) log(msg string, objects ...interface{}) {
	if nil != c.Logger {
		c.Logger.Printf(msg, objects...)
	}
}

func getBaseURL(r *http.Request) (*url.URL, error) {
	return url.Parse(r.Host + r.URL.Path)
}

type StringIdentifier string

func (s StringIdentifier) String() string {
	return string(s)
}
