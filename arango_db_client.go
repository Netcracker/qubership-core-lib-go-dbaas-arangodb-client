package arangodbaas

import (
	"context"
	"fmt"
	"net/http"

	"github.com/arangodb/go-driver/v2/arangodb"
	"github.com/arangodb/go-driver/v2/arangodb/shared"
	"github.com/arangodb/go-driver/v2/connection"
	"github.com/netcracker/qubership-core-lib-go-dbaas-arangodb-client/v4/model"
	dbaasbase "github.com/netcracker/qubership-core-lib-go-dbaas-base-client/v3"
	"github.com/netcracker/qubership-core-lib-go-dbaas-base-client/v3/cache"
	basemodel "github.com/netcracker/qubership-core-lib-go-dbaas-base-client/v3/model"
	"github.com/netcracker/qubership-core-lib-go-dbaas-base-client/v3/model/rest"
	"github.com/netcracker/qubership-core-lib-go/v3/utils"
)

type ArangoDbClient interface {
	GetArangoDatabase(ctx context.Context, dbId string) (arangodb.Database, error)
}

type ArangoDbClientImpl struct {
	httpConfig    *connection.HttpConfiguration
	dbaasClient   dbaasbase.DbaaSClient
	arangodbCache *cache.DbaaSCache
	params        model.DbParams
}

type cachedArangoClient struct {
	arangoClient arangodb.Client
	dbName       string
}

func (a *ArangoDbClientImpl) GetArangoDatabase(ctx context.Context, dbId string) (arangodb.Database, error) {
	classifier := a.params.Classifier(context.WithValue(ctx, dbIdParam, dbId))
	key := cache.NewKey(DbType, classifier)
	rawCachedClient, err := a.arangodbCache.Cache(key, a.createNewArangoClient(ctx, classifier))
	if err != nil {
		return nil, err
	}
	cachedClient := rawCachedClient.(*cachedArangoClient)
	db, err := cachedClient.arangoClient.GetDatabase(ctx, cachedClient.dbName, nil)

	if shared.IsNoLeader(err) {
		logger.ErrorC(ctx, "can't connect to arango database: %s", err)
		return nil, err
	}

	if shared.IsUnauthorized(err) {
		logger.InfoC(ctx, "Trying to get new password due to the authentication error: %+v", err)
		err = a.updateAuthentication(ctx, cachedClient, classifier, a.params.BaseDbParams)
		if err != nil {
			return nil, err
		}
		db, err = cachedClient.arangoClient.GetDatabase(ctx, cachedClient.dbName, nil)
	}

	return db, err
}

func (a *ArangoDbClientImpl) createNewArangoClient(ctx context.Context, classifier map[string]interface{}) func() (interface{}, error) {
	return func() (interface{}, error) {
		logger.DebugC(ctx, "Creating ArangoDB database with classifier %+v", classifier)
		logicalDb, err := a.dbaasClient.GetOrCreateDb(ctx, DbType, classifier, a.params.BaseDbParams)
		if err != nil {
			logger.ErrorC(ctx, "Failed to get database from DBaaS client: %+v", err)
			return nil, err
		}
		httpConfig := buildHttpConfiguration(*a.httpConfig, logicalDb)
		conn := connection.NewHttpConnection(httpConfig)
		client := arangodb.NewClient(conn)
		agroalConnProperties := toArangoConnProperties(logicalDb.ConnectionProperties)
		return &cachedArangoClient{client, agroalConnProperties.DbName}, nil
	}
}

func (a *ArangoDbClientImpl) updateAuthentication(ctx context.Context, c *cachedArangoClient, classifier map[string]interface{}, params rest.BaseDbParams) error {
	connectionProperties, err := a.dbaasClient.GetConnection(ctx, DbType, classifier, params)
	if err != nil {
		logger.ErrorC(ctx, "Cannot get connection properties from DBaaS client")
		return err
	}
	agroalConnProperties := toArangoConnProperties(connectionProperties)
	err = c.arangoClient.Connection().SetAuthentication(connection.NewBasicAuth(agroalConnProperties.Username, agroalConnProperties.Password))
	if err != nil {
		logger.ErrorC(ctx, "Cannot set authentication to connection")
		return err
	}
	return nil
}

func buildHttpConfiguration(httpConfig connection.HttpConfiguration, logicalDb *basemodel.LogicalDb) connection.HttpConfiguration {
	agroalConnProperties := toArangoConnProperties(logicalDb.ConnectionProperties)

	endpoints := []string{fmt.Sprintf("http://%s:%d", agroalConnProperties.Host, agroalConnProperties.Port)}
	transport := httpConfig.Transport

	if tls, ok := logicalDb.ConnectionProperties["tls"].(bool); ok && tls {
		logger.Infof("Connection to arangodb=%s will be secured", logicalDb.Name)

		endpoints = []string{fmt.Sprintf("https://%s:%d", agroalConnProperties.Host, TlsArangodbPort)}

		if transport == nil {
			transport = &http.Transport{}
		}
		transport.(*http.Transport).TLSClientConfig = utils.GetTlsConfig()
	}

	httpConfig.Endpoint = connection.NewRoundRobinEndpoints(endpoints)
	httpConfig.Transport = transport
	httpConfig.Authentication = connection.NewBasicAuth(agroalConnProperties.Username, agroalConnProperties.Password)

	return httpConfig
}
