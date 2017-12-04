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
	"strings"
)

type Controller interface {
	Routes() []Route
	BasePath() string
}

type Unmarshaller interface {
	json.Unmarshaler
	Entity() IdentifiableEntity
}

type MiddlewareSetter func(method Method, path Path, handler httprouter.Handle) httprouter.Handle

type DefaultController struct {
	DAO              DAO
	basePath         string
	ErrorHandler     resterrors.Handler
	Logger           Logger
	Unmarshaller     Unmarshaller
	MiddlewareSetter MiddlewareSetter
}

const keyIdentifier = "id"

type StringIdentifier string

func (s StringIdentifier) String() string {
	return string(s)
}

func NewController(basePath string, dao DAO, errorHandler resterrors.Handler, unmarshaller Unmarshaller) *DefaultController {
	return &DefaultController{
		DAO:          dao,
		basePath:     basePath,
		ErrorHandler: errorHandler,
		Unmarshaller: unmarshaller,
	}
}

func (c DefaultController) BasePath() string {
	return c.basePath
}

func (c *DefaultController) Routes() []Route {
	defaultRoutes := []Route{
		NewRoute(GET, Path("/"+c.basePath), c.GetAll),
		NewRoute(GET, Path("/"+c.basePath+"/:"+keyIdentifier), c.Get),
		NewRoute(PUT, Path("/"+c.basePath), c.Update),
		NewRoute(DELETE, Path("/"+c.basePath+"/:"+keyIdentifier), c.Delete),
	}

	if nil == c.MiddlewareSetter {
		return defaultRoutes
	}

	routesWithMiddlewares := make([]Route, 0)
	for _, route := range defaultRoutes {
		routesWithMiddlewares = append(routesWithMiddlewares, NewRoute(route.Method(), route.Path(), c.MiddlewareSetter(route.Method(), route.Path(), route.Handler())))
	}
	return routesWithMiddlewares
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
	w.Header().Set("Content-Type", "application/json")
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
	w.Header().Set("Content-Type", "application/json")
	_, err = fmt.Fprint(w, string(jsonEntity))
	if err != nil {
		c.Handle(w, errors.Wrapf(err, "Writing entity as response '%s'", string(jsonEntity)))
		return
	}
	return
}

func (c *DefaultController) Update(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	// Copy EmptyInstance to not modify it and only use it as reference
	bodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		c.Handle(w, errors.Wrapf(err, "Reading body"))
		return
	}
	err = json.Unmarshal(bodyBytes, c.Unmarshaller)
	if err != nil {
		c.Handle(w, errors.Wrapf(err, "Unmarshal %s", string(bodyBytes)))
		return
	}
	id, err := c.DAO.Set(c.Unmarshaller.Entity())
	if err != nil {
		c.Handle(w, errors.Wrapf(err, "Update '%+v'", c.Unmarshaller.Entity()))
		return
	}
	baseURL, err := getBaseURL(r)
	if err != nil {
		c.Handle(w, errors.Wrapf(err, "Get base request url from request"))
		return
	}
	baseURLStr := baseURL.String()
	idStr := id.String()
	fmt.Fprintf(w, baseURLStr+"/%s", idStr)
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
	return url.Parse("http://" + r.Host + r.URL.Path)
}

type CORSController struct {
	Controller
	MiddlewareSetter MiddlewareSetter
	allowedOrigin    string
}

func NewCORSController(controller Controller, allowedOrigin string) *CORSController {
	return &CORSController{
		Controller:    controller,
		allowedOrigin: allowedOrigin,
	}
}

func (c *CORSController) Routes() []Route {
	routes := c.Controller.Routes()
	optRoute := NewRoute(OPTIONS, Path("/"+c.BasePath()), c.Options)
	if nil != c.MiddlewareSetter {
		optRoute = NewRoute(optRoute.Method(), optRoute.Path(), c.MiddlewareSetter(optRoute.Method(), optRoute.Path(), optRoute.handler))
	}
	return append(routes, optRoute)
}

func (c CORSController) Options(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	routes := c.Routes()
	methods := make([]string, 0)
	for _, route := range routes {
		methods = append(methods, string(route.Method()))
	}
	allMethods := strings.Join(methods, ",")
	w.Header().Add("Allow", allMethods)
	w.Header().Add("Access-Control-Allow-Origin", c.allowedOrigin)
	w.Header().Add("Access-Control-Allow-Methods", allMethods)
	w.Header().Add("Access-Control-Allow-Headers", "*")
}
