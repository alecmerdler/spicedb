package nats

import "github.com/authzed/spicedb/pkg/datastore"

func init() {
	datastore.Engines = append(datastore.Engines, Engine)
}

const (
	Engine = "nats"
)

type natsDatastore struct{}

var _ datastore.Datastore = &natsDatastore{}
