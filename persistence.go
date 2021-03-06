package rest

import (
	"database/sql"
	"fmt"

	"github.com/gofrs/uuid"
	"github.com/pkg/errors"
)

type Identifier interface {
	fmt.Stringer
}

type Entity interface{}

type DAO interface {
	GetAllEntities(Pagination) ([]Entity, error)
	GetAllIDs(Pagination) ([]Identifier, error)
	TotalNumberOfEntities() (int64, error)
	Get(Identifier) (Entity, error)
	Set(entity IdentifiableEntity) (Identifier, error)
	Delete(Identifier) error
}

type IdentifiableEntity interface {
	Entity
	ID() Identifier
	WithID(Identifier) (IdentifiableEntity, error)
}

type Mapper interface {
	ToEntities(*sql.Rows) ([]Entity, error)
	ToIdentifiers(*sql.Rows) ([]Identifier, error)
	ToSlice(Entity) ([]interface{}, error)
}

type IdentifierGenerator interface {
	Generate(entity Entity) (Identifier, error)
}

type UUIDIdentifierGenerator struct {
}

func (g UUIDIdentifierGenerator) Generate(_ Entity) (Identifier, error) {
	return uuid.NewV4()
}

type Queries interface {
	GetAllEntities() string
	GetAllIDs() string
	TotalNumberOfEntities() string
	Get() string
	Insert() string
	Update() string
	Delete() string
}

type queryKey string

const (
	getAllEntities = queryKey("getAllEntities")
	getAllIDs      = queryKey("getAllIds")
	size           = queryKey("size")
	get            = queryKey("get")
	insert         = queryKey("insert")
	update         = queryKey("update")
	delete         = queryKey("delete")
)

type DatabaseDAO struct {
	idGenerator IdentifierGenerator
	mapper      Mapper
	queries     map[queryKey]*sql.Stmt
}

func NewDatabaseDAO(db *sql.DB, mapper Mapper, queries Queries, idGenerator IdentifierGenerator) (*DatabaseDAO, error) {
	var err error
	preparedQueries := make(map[queryKey]*sql.Stmt)
	preparedQueries[getAllEntities], err = db.Prepare(queries.GetAllEntities())
	if err != nil {
		return nil, errors.Wrapf(err, "Error when preparing %s", queries.GetAllEntities())
	}
	preparedQueries[getAllIDs], err = db.Prepare(queries.GetAllIDs())
	if err != nil {
		return nil, errors.Wrapf(err, "Error when preparing %s", queries.GetAllIDs())
	}
	preparedQueries[size], err = db.Prepare(queries.TotalNumberOfEntities())
	if err != nil {
		return nil, errors.Wrapf(err, "Error when preparing %s", queries.TotalNumberOfEntities())
	}
	preparedQueries[get], err = db.Prepare(queries.Get())
	if err != nil {
		return nil, errors.Wrapf(err, "Error when preparing %s", queries.Get())
	}
	preparedQueries[insert], err = db.Prepare(queries.Insert())
	if err != nil {
		return nil, errors.Wrapf(err, "Error when preparing %s", queries.Insert())
	}
	preparedQueries[update], err = db.Prepare(queries.Update())
	if err != nil {
		return nil, errors.Wrapf(err, "Error when preparing %s", queries.Update())
	}
	preparedQueries[delete], err = db.Prepare(queries.Delete())
	if err != nil {
		return nil, errors.Wrapf(err, "Error when preparing %s", queries.Delete())
	}

	return &DatabaseDAO{
		mapper:      mapper,
		queries:     preparedQueries,
		idGenerator: idGenerator,
	}, nil
}

func (d *DatabaseDAO) Close() {
	for _, query := range d.queries {
		query.Close()
	}
}

func (d *DatabaseDAO) GetAllEntities(p Pagination) ([]Entity, error) {
	rows, err := d.queries[getAllEntities].Query(p.Offset(), p.Limit())
	if err != nil {
		return nil, errors.Wrapf(err, "Retrieving entities from database")
	}
	defer rows.Close()
	entities, err := d.mapper.ToEntities(rows)
	if err != nil {
		return nil, errors.Wrapf(err, "Map result set to entity")
	}
	if err = rows.Err(); nil != err {
		return nil, errors.Wrapf(err, "Error while looping through entity rows")
	}
	return entities, nil
}

func (d *DatabaseDAO) GetAllIDs(p Pagination) ([]Identifier, error) {
	queries := d.queries
	getAllQuery := queries[getAllIDs]
	rows, err := getAllQuery.Query(p.Offset(), p.Limit())
	if err != nil {
		return nil, errors.Wrapf(err, "Retrieving entities from database")
	}
	defer rows.Close()
	identifiers, err := d.mapper.ToIdentifiers(rows)
	if err != nil {
		return nil, errors.Wrapf(err, "Map result set to identifiers")
	}
	if err = rows.Err(); nil != err {
		return nil, errors.Wrapf(err, "Error while looping through identifiers rows")
	}
	return identifiers, nil
}

func (d *DatabaseDAO) TotalNumberOfEntities() (int64, error) {
	row := d.queries[size].QueryRow()
	var nbItems int64
	err := row.Scan(&nbItems)
	if err != nil {
		return 0, errors.Wrapf(err, "Counting number of entities in database")
	}
	return nbItems, nil
}

func (d *DatabaseDAO) Get(id Identifier) (Entity, error) {
	rows, err := d.queries[get].Query(id)
	if err != nil {
		return nil, errors.Wrapf(err, "Retrieving entities from database")
	}
	defer rows.Close()
	entities, err := d.mapper.ToEntities(rows)
	if err != nil {
		return nil, errors.Wrapf(err, "Map result set to entity")
	}
	if err = rows.Err(); nil != err {
		return nil, errors.Wrapf(err, "Error while looping through entity rows")
	}
	nbEntities := len(entities)
	if nbEntities > 1 {
		return nil, fmt.Errorf("Expected only one entity identified by '%s' but got %d", id, nbEntities)
	}
	if nbEntities == 0 {
		return nil, nil
	}
	return entities[0], nil
}

func (d *DatabaseDAO) Set(entity IdentifiableEntity) (Identifier, error) {
	id := entity.ID()
	shouldInsert := false
	if nil == id {
		generator := d.idGenerator
		id, err := generator.Generate(entity)
		if err != nil {
			return nil, errors.Wrapf(err, "Generating Identifier")
		}
		entity, err = entity.WithID(id)
		if err != nil {
			return nil, errors.Wrapf(err, "Setting ID")
		}
		shouldInsert = true
	} else {
		found, err := d.Get(id)
		if err != nil {
			return nil, errors.Wrapf(err, "Checking if entity exist")
		}
		shouldInsert = nil == found
	}

	s, err := d.mapper.ToSlice(entity)
	if err != nil {
		return nil, errors.Wrapf(err, "Turn an entity into a slice of fields")
	}
	if shouldInsert {
		_, err := d.queries[insert].Exec(s...)
		if err != nil {
			return nil, errors.Wrapf(err, "Inserting '%+v'", entity)
		}
	} else {
		_, err := d.queries[update].Exec(s...)
		if err != nil {
			return nil, errors.Wrapf(err, "Updating '%+v'", entity)
		}
	}

	return entity.ID(), nil
}

func (d *DatabaseDAO) Delete(id Identifier) error {
	_, err := d.queries[delete].Exec(id)
	if err != nil {
		return errors.Wrapf(err, "Deleting '%s'", id.String())
	}
	return nil
}
