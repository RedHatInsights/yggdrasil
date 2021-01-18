package yggdrasil

import "github.com/hashicorp/go-memdb"

const (
	tableNameWorker  string = "worker"
	tableNameData    string = "data"
	tableNameProcess string = "process"
	indexNameID      string = "id"
	indexNameHandler string = "handler"
)

// NewDatastore creates a new MemDB initialized with the application schema.
func NewDatastore() (*memdb.MemDB, error) {
	schema := &memdb.DBSchema{
		Tables: map[string]*memdb.TableSchema{
			tableNameWorker: {
				Name: tableNameWorker,
				Indexes: map[string]*memdb.IndexSchema{
					indexNameID: {
						Name:    indexNameID,
						Unique:  true,
						Indexer: &memdb.IntFieldIndex{Field: "pid"},
					},
					indexNameHandler: {
						Name:    indexNameHandler,
						Unique:  true,
						Indexer: &memdb.StringFieldIndex{Field: "handler"},
					},
				},
			},
			tableNameData: {
				Name: tableNameData,
				Indexes: map[string]*memdb.IndexSchema{
					indexNameID: {
						Name:    indexNameID,
						Unique:  true,
						Indexer: &memdb.StringFieldIndex{Field: "MessageID"},
					},
					indexNameHandler: {
						Name:    indexNameHandler,
						Unique:  false,
						Indexer: &memdb.StringFieldIndex{Field: "Directive"},
					},
				},
			},
			tableNameProcess: {
				Name: tableNameProcess,
				Indexes: map[string]*memdb.IndexSchema{
					indexNameID: {
						Name:    indexNameID,
						Unique:  true,
						Indexer: &memdb.IntFieldIndex{Field: "pid"},
					},
				},
			},
		},
	}

	db, err := memdb.NewMemDB(schema)
	if err != nil {
		return nil, err
	}

	return db, nil
}
