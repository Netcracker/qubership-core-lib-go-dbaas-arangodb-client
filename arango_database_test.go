package arangodbaas

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"

	"github.com/arangodb/go-driver/v2/connection"
	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
	"github.com/netcracker/qubership-core-lib-go/v3/serviceloader"
	"github.com/netcracker/qubership-core-lib-go/v3/security"
	dbaasbase "github.com/netcracker/qubership-core-lib-go-dbaas-base-client/v3"
	"github.com/netcracker/qubership-core-lib-go-dbaas-base-client/v3/model"
	. "github.com/netcracker/qubership-core-lib-go-dbaas-base-client/v3/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

const (
	dbaasAgentUrlEnvName     = "dbaas.agent"
	testServiceName          = "service_test"
	createDatabaseV3         = "/api/v3/dbaas/test_namespace/databases"
	getDatabaseV3            = "/api/v3/dbaas/test_namespace/databases/get-by-classifier/arango"
	username                 = "service_test"
	password                 = "qwerty127"
	dbName                   = "arango_db_name"
	host                     = "arango.host"
	port                     = 8529
	dbIdParamValue           = "db-id-1"
)

type ArangoDatabaseTestSuite struct {
	suite.Suite
	database Database
}

func (suite *ArangoDatabaseTestSuite) SetupSuite() {
	serviceloader.Register(1, &security.DummyToken{})

	StartMockServer()
	os.Setenv(propMicroserviceName, "test_service")
	os.Setenv(namespaceEnvName, "test_space")
	os.Setenv(dbaasAgentUrlEnvName, GetMockServerUrl())
	os.Setenv(namespaceEnvName, "test_namespace")
	os.Setenv(propMicroserviceName, testServiceName)

	configloader.Init(configloader.EnvPropertySource())
}

func (suite *ArangoDatabaseTestSuite) TearDownSuite() {
	os.Unsetenv(dbaasAgentUrlEnvName)
	os.Unsetenv(namespaceEnvName)
	os.Unsetenv(propMicroserviceName)
	StopMockServer()
}

func (suite *ArangoDatabaseTestSuite) BeforeTest(suiteName, testName string) {
	suite.T().Cleanup(ClearHandlers)
	dbaasPool := dbaasbase.NewDbaaSPool()
	client := NewClient(dbaasPool)
	suite.database = client.ServiceDatabase()
}

func (suite *ArangoDatabaseTestSuite) TestGetArangoDbClient_WithoutConfig() {
	db := prepareArangoDatabase()
	arangoDbClient, err := db.GetArangoDbClient()
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), arangoDbClient)
}

func (suite *ArangoDatabaseTestSuite) TestGetArangoDbClient_WithConfig() {
	db := prepareArangoDatabase()
	config := &connection.HttpConfiguration{}
	arangoDbClient, err := db.GetArangoDbClient(config)
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), arangoDbClient)
}

func prepareArangoDatabase() Database {
	dbaasPool := dbaasbase.NewDbaaSPool()
	commonClient := NewClient(dbaasPool)
	return commonClient.ServiceDatabase()
}

func defaultDbaasResponseHandler(writer http.ResponseWriter, request *http.Request) {
	body, _ := io.ReadAll(request.Body)
	var requestObject map[string]interface{}
	json.Unmarshal(body, &requestObject)

	var dbResponse model.LogicalDb
	if dbIdParamValue == requestObject["classifier"].(map[string]interface{})[DbIdKey] {
		connectionProperties := map[string]interface{}{
			"dbName":   "arango_db_name",
			"host":     "arango.host",
			"port":     float64(8529),
			"password": "qwerty127",
			"username": "service_test",
		}
		dbResponse = model.LogicalDb{
			Id:                   "123",
			ConnectionProperties: connectionProperties,
		}
	} else {
		fmt.Errorf("wrong value dbId !!!")
		dbResponse = model.LogicalDb{
			ConnectionProperties: make(map[string]interface{}),
		}
	}
	writer.WriteHeader(http.StatusOK)
	jsonResponse, _ := json.Marshal(dbResponse)
	writer.Write(jsonResponse)
}

func TestArangoDatabase(t *testing.T) {
	suite.Run(t, new(ArangoDatabaseTestSuite))
}

func (suite *ArangoDatabaseTestSuite) TestGetConnectionProperties() {
	AddHandler(Contains(createDatabaseV3), defaultDbaasResponseHandler)

	db := prepareArangoDatabase()

	ctx := context.Background()
	connectionProperties, err := db.GetConnectionPropertiesWithDbId(ctx, dbIdParamValue)
	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), password, connectionProperties.Password)
	assert.Equal(suite.T(), username, connectionProperties.Username)
	assert.Equal(suite.T(), dbName, connectionProperties.DbName)
	assert.Equal(suite.T(), host, connectionProperties.Host)
	assert.Equal(suite.T(), port, connectionProperties.Port)
}

func (suite *ArangoDatabaseTestSuite) TestFindConnectionPropertiesWithDbId() {
	AddHandler(Contains(getDatabaseV3), defaultDbaasResponseHandler)

	db := prepareArangoDatabase()
	connectionProperties, err := db.FindConnectionPropertiesWithDbId(context.Background(), dbIdParamValue)
	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), password, connectionProperties.Password)
	assert.Equal(suite.T(), username, connectionProperties.Username)
	assert.Equal(suite.T(), dbName, connectionProperties.DbName)
	assert.Equal(suite.T(), host, connectionProperties.Host)
	assert.Equal(suite.T(), port, connectionProperties.Port)
}
