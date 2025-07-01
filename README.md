[![Go build](https://github.com/Netcracker/qubership-core-lib-go-dbaas-arangodb-client/actions/workflows/go-build.yml/badge.svg)](https://github.com/Netcracker/qubership-core-lib-go-dbaas-arangodb-client/actions/workflows/go-build.yml)
[![Coverage](https://sonarcloud.io/api/project_badges/measure?metric=coverage&project=Netcracker_qubership-core-lib-go-dbaas-arangodb-client)](https://sonarcloud.io/summary/overall?id=Netcracker_qubership-core-lib-go-dbaas-arangodb-client)
[![duplicated_lines_density](https://sonarcloud.io/api/project_badges/measure?metric=duplicated_lines_density&project=Netcracker_qubership-core-lib-go-dbaas-arangodb-client)](https://sonarcloud.io/summary/overall?id=Netcracker_qubership-core-lib-go-dbaas-arangodb-client)
[![vulnerabilities](https://sonarcloud.io/api/project_badges/measure?metric=vulnerabilities&project=Netcracker_qubership-core-lib-go-dbaas-arangodb-client)](https://sonarcloud.io/summary/overall?id=Netcracker_qubership-core-lib-go-dbaas-arangodb-client)
[![bugs](https://sonarcloud.io/api/project_badges/measure?metric=bugs&project=Netcracker_qubership-core-lib-go-dbaas-arangodb-client)](https://sonarcloud.io/summary/overall?id=Netcracker_qubership-core-lib-go-dbaas-arangodb-client)
[![code_smells](https://sonarcloud.io/api/project_badges/measure?metric=code_smells&project=Netcracker_qubership-core-lib-go-dbaas-arangodb-client)](https://sonarcloud.io/summary/overall?id=Netcracker_qubership-core-lib-go-dbaas-arangodb-client)

# ArangoDB DBaaS Go Client

This module provides convenient way of interaction with **ArangoDB** databases provided by dbaas-aggregator.
`ArangoDB DBaaS Go Client` supports _multi-tenancy_ and can work with both _service_ and _tenant_ databases.

- [Install](#install)
- [Usage](#usage)
    * [ArangoDbClient](#arangodbclient)
    * [Get connection for existing database or create new one](#get-connection-for-existing-database-or-create-new-one)
    * [Find connection for existing database](#find-connection-for-existing-database)
    * [Arango multiusers](#arango-multiusers)
    * [Arango Auto Sync Endpoints](#arango-auto-sync-endpoints)
- [Classifier](#classifier)
- [SSL/TLS support](#ssltls-support)
- [Quick example](#quick-example)

## Install
To get ArangoDB DBaaS Go Client do:
```go
    go get github.com/netcracker/qubership-core-lib-go-dbaas-arangodb-client
```

List of all released versions may be found [here](https://github.com/netcracker/qubership-core-lib-go-dbaas-arangodb-client/-/tags)

## Usage

At first, it's necessary to register security implemention - dummy or your own, the followning example shows registration of required services:
```go
import (
	"github.com/netcracker/qubership-core-lib-go/v3/serviceloader"
	"github.com/netcracker/qubership-core-lib-go/v3/security"
)

func init() {
	serviceloader.Register(1, &security.DummyToken{})
}
```

Then the user should create `DbaaSArangoDbClient`. This is a base client, which allows working with tenant and service databases.
To create instance of `DbaaSArangoDbClient` use `NewClient(pool *dbaasbase.DbaaSPool) *DbaaSArangoDbClient`.

Note that client has parameter _pool_. `dbaasbase.DbaaSPool` is a tool which stores all cached connections and
create new ones. To find more info visit [dbaasbase](https://github.com/netcracker/qubership-core-lib-go-dbaas-base-client/blob/main/README.md)

Example of client creation:
```go
pool := dbaasbase.NewDbaasPool()
client := arangodbaas.NewClient(pool)
```

_Note_:By default, `ArangoDB DBaaS Go Client` supports dbaas-aggregator as databases source. But there is a possibility for user to provide another
sources. To do so use [LogcalDbProvider](https://github.com/netcracker/qubership-core-lib-go-dbaas-base-client/blob/main/README.md#logicaldbproviders)
from dbaasbase.

Next step is to create `Database` object. It just an interface which allows creating ArangoDB client.
At this step user may choose which type of database he will work with:  `service` or `tenant`.

* To work with service databases use `ServiceDatabase(params ...model.DbParams) Database`
* To work with tenant databases use `TenantDatabase(params ...model.DbParams) Database`

Each func has `DbParams` as parameter.

DbParams store information for database creation. Note that this parameter is optional, but if user doesn't pass Classifier,
default one will be used. More about classifiers [here](#classifier)

| Name         | Description                                                                                           | type                                                                                                                      |
|--------------|-------------------------------------------------------------------------------------------------------|---------------------------------------------------------------------------------------------------------------------------|
| Classifier   | function which builds classifier from context. Classifier should be unique for each arango db.        | func(ctx context.Context) map[string]interface{}                                                                          |
| BaseDbParams | Specific parameters for operation with database.                                                      | [BaseDbParams](https://github.com/netcracker/qubership-core-lib-go-dbaas-base-client/blob/main#basedbparams)  |

Example how to create an instance of Database.
```go
 dbPool := dbaasbase.NewDbaasPool()
 client := arangodbaas.NewClient(dbPool)
 serviceDB := client.ServiceDatabase() // service Database creation 
 tenantDB := client.TenantDatabase() // tenant Database creation 
```

`Database` allows:   
* get ArangoDbClient, through which you can create Arango DB and get `arangodb.Database` for database operation. 
`serviceDB` and `tenantDB`  instances should be singleton and it's enough to create them only once.

### ArangoDbClient

ArangoDbClient is a special object, which allows getting `arangodb.Database` to establish connection and to operate with a database. 
`ArangoDbClient` is a singleton and should be created only once.

ArangoDbClient has method `GetArangoDatabase(ctx context.Context, dbId string) (arangodb.Database, error)` which will return `arangodb.Database` to work with the database.
We strongly recommend not to store `arangodb.Database` as singleton and get new connection for every block of code.
This is because the password in the database may be changed (by DBaaS or someone else) and then the connection will return an error. Every time the function
`arangoDbClient.GetArangoDatabase(ctx, dbId)` is called, the password lifetime and correctness is checked. If necessary, the password is updated.

_Note_: classifier will be created with context and function from DbParams.

To create arangoDbClient use `GetArangoDbClient(httpConfigs ...*connection.HttpConfiguration) (ArangoDbClient, error)`

Parameters:
* httpConfigs _optional_ - user may pass desired *connection.HttpConfiguration or don't pass anything at all. Note that user **doesn't have to 
set connection parameters** with config, because these parameters will be received from dbaas-aggregator.

```go
    ctx := ctxmanager.InitContext(context.Background(), propagateHeaders()) // preferred way
    // ctx := context.Background() // also possible for service client, but not recommended
    httpConfig := &connection.HttpConfiguration{}
    arangoDbClient, err := database.GetArangoDbClient(httpConfig) // with config
    dbId := "sample-db"
    arangoDb, err := arangoDbClient.GetArangoDatabase(ctx, dbId)

    collection, err := database.Collection(ctx, "collection-name")
    if err != nil { return err }
```

### Get connection for existing database or create new one

Func `GetConnectionProperties(ctx context.Context) (*model.ArangoConnProperties, error)`
at first will check if the desired database with _arangodb_ type and classifier exists. If it exists, function will just return
connection properties in the form of [ArangoConnProperties](model/agroal_conn_properties.go).
If database with _arangodb_ type and classifier doesn't exist, such database will be created and function will return
connection properties for a new created database.

_Parameters:_
* ctx - context, enriched with some headers (See docs about context-propagation [here](https://github.com/netcracker/qubership-core-lib-go/blob/main/context-propagation/README.md)). Context object can have request scope values from which can be used to build classifier, for example tenantId.

```go
    ctx := ctxmanager.InitContext(context.Background(), propagateHeaders()) // preferred way
    // ctx := context.Background() // also possible for service client, but not recommended
    dbArangoConnection, err := database.GetConnectionProperties(ctx)
```

### Find connection for existing database

Func `FindConnectionProperties(ctx context.Context) (*model.ArangoConnProperties, error)`
returns connection properties in the form of [ArangoConnProperties](model/agroal_conn_properties.go). Unlike `GetConnectionProperties`
this function won't create database if it doesn't exist and just return nil value.

_Parameters:_
* ctx - context, enriched with some headers. (See docs about context-propagation [here](https://github.com/netcracker/qubership-core-lib-go/blob/main/context-propagation/README.md)). Context object can have request scope values from which can be used to build classifier, for example tenantId.

```go
    ctx := ctxmanager.InitContext(context.Background(), propagateHeaders()) // preferred way
    // ctx := context.Background() // also possible for service client, but not recommended
    dbArangoConnection, err := database.FindConnectionProperties(ctx)
```

### Arango multiusers
For specifying connection properties user role you should add this role in BaseDbParams structure:

```go
params := model.DbParams{
        Classifier:   Classifier, //database classifier
        BaseDbParams: rest.BaseDbParams{Role: "admin"}, //for example "admin", "rw", "ro"
    }
dbPool := dbaasbase.NewDbaaSPool()
client := mongodbaas.NewClient(dbPool)
serviceDB := client.ServiceDatabase(params)
arangoClient, err := serviceDB.GetArangoDbClient()
dbId := "sample-db"
arangoDb, err := arangoClient.GetArangoDatabase(ctx, dbId)
```

Requests to DbaaS will contain the role you specify in this structure.

## Classifier

Classifier and dbType should be unique combination for each database. Fields "tenantId" or "scope" must be into users' custom classifiers.

User can use default service or tenant classifier. It will be used if user doesn't specify Classifier in DbParams. 
This is recommended approach, and we don't recommend using custom classifier because it can lead to some problems. 
Use can be reasonable if you migrate to this module and before used custom and not default classifier.


Default service classifier looks like:
```json
{
    "scope": "service",
    "microserviceName": "<ms-name>"
}
```

Default tenant classifier looks like

```json
{
  "scope": "tenant",
  "tenantId": "<tenant-external-id>",
  "microserviceName": "<ms-name>"
}
```
Note, that if user doesn't set `MICROSERVICE_NAME` (or `microservice.name`) property, there will be panic during default classifier creation.
Also, if there are no tenantId in tenantContext, **panic will be thrown**.

## SSL/TLS support

This library supports work with secured connections to arangodb. Connection will be secured if TLS mode is enabled in
arangodb-adapter.

For correct work with secured connections, the library requires having a truststore with certificate.
It may be public cloud certificate, cert-manager's certificate or any type of certificates related to database.
We do not recommend use self-signed certificates. Instead, use default NC-CA.

To start using TLS feature user has to enable it on the physical database (adapter's) side and add certificate to service truststore.

### Physical database switching
To enable TLS support in physical database redeploy arangodb with mandatory parameters
```yaml
tls.enabled=true;
```

In case of using cert-manager as certificates source add extra parameters
```yaml
ISSUER_NAME=<cluster issuer name>;
tls.generateCerts.enabled=true;
tls.generateCerts.clusterIssuerName=<cluster issuer name>;
```

ClusterIssuerName identifies which Certificate Authority cert-manager will use to issue a certificate.
It can be obtained from the person in charge of the cert-manager on the environment.

## Quick example

Here we create ArangoDB tenant client, then get ArangoDbClient and execute a query.

application.yaml
```yaml
  microservice.name=sample-microservice
```

```go
package main

import (
	"context"
	"net/http"
	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
	"github.com/netcracker/qubership-core-lib-go/v3/context-propagation/ctxmanager"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	arangodbaas "github.com/netcracker/qubership-core-lib-go-dbaas-arangodb-client"
	dbaasbase "github.com/netcracker/qubership-core-lib-go-dbaas-base-client/v3"
	"github.com/netcracker/qubership-core-lib-go/v3/context-propagation/baseproviders/tenant"
	"github.com/arangodb/go-driver/v2/connection"
	"time"
)

func init() {
	configloader.InitWithSourcesArray(configloader.BasePropertySources())
	logger = logging.GetLogger("main")
	ctxmanager.Register([]ctxmanager.ContextProvider{tenant.TenantProvider{}})
}

func main() {

	// some context initialization
	ctx := ctxmanager.InitContext(context.Background(), map[string]interface{}{tenant.TenantHeader: "123"})

	// create tenant client
	dbaasPool := dbaasbase.NewDbaaSPool()
	dbaasArangoClient := arangodbaas.NewClient(dbaasPool)
	database := dbaasArangoClient.TenantDatabase()

	// create arangoClient
	httpConfig := connection.HttpConfiguration{
		Transport: &http.Transport{
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
    }
	arangoClient, err := database.GetArangoDbClient(&httpConfig) // singleton for tenant databases. This object must be used to get connection in the entire application.
	if err != nil {
		logger.Error("Error during arango client creation")
	}

	// get database
	dbId := "sample-db"
	arangoDb, err := arangoClient.GetArangoDatabase(ctx, dbId)
	if err != nil {
		logger.Error("Error during arango database creation")
	}

	// sample DB operations
	collectionName := "sample-collection"
	found, err := arangoDb.CollectionExists(ctx, collectionName)
	if err != nil {
		logger.Error("Cannot check collection existence: %+v", err)
	}
	if !found {
		_, err = arangoDb.CreateCollection(ctx, collectionName, &arangodb.CreateCollectionProperties{})
		if err != nil {
			logger.Error("Cannot create collection: %+v", err)
		}
	}

	collection, err := arangoDb.Collection(ctx, collectionName)
	if err != nil {
		logger.Error("Cannot get collection: %+v", err)
	}
	document := Document{Text: "sample"}
	meta, err := collection.CreateDocument(ctx, document)
	if err != nil {
		logger.Error("Cannot create new document: %+v", err)
	}
	key := meta.Key

	var readDocument Document
	_, err = collection.ReadDocument(ctx, key, &readDocument)
	if err != nil {
		logger.Error("Cannot read document: %+v", err)
	}

	_, err = collection.DeleteDocument(ctx, key)
	if err != nil {
		logger.Error("Cannot remove document: %+v", err)
	}
}

type Document struct {
	Text string `json:"text"`
}
```
