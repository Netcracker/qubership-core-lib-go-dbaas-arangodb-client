package arangodbaas

import (
	"context"
	"os"
	"testing"

	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
	"github.com/netcracker/qubership-core-lib-go/v3/serviceloader"
	"github.com/netcracker/qubership-core-lib-go/v3/security"
	"github.com/netcracker/qubership-core-lib-go/v3/context-propagation/baseproviders/tenant"
	"github.com/netcracker/qubership-core-lib-go/v3/context-propagation/ctxmanager"
	"github.com/netcracker/qubership-core-lib-go-dbaas-arangodb-client/v4/model"
	dbaasbase "github.com/netcracker/qubership-core-lib-go-dbaas-base-client/v3"
	"github.com/netcracker/qubership-core-lib-go-dbaas-base-client/v3/model/rest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

const (
	namespaceEnvName = "microservice.namespace"
)

type ArangoClientTestSuite struct {
	suite.Suite
}

func (suite *ArangoClientTestSuite) SetupSuite() {
	serviceloader.Register(1, &security.DummyToken{})
	os.Setenv(propMicroserviceName, "test_service")
	os.Setenv(namespaceEnvName, "test_space")
	configloader.Init(configloader.EnvPropertySource())
	ctxmanager.Register([]ctxmanager.ContextProvider{tenant.TenantProvider{}})
}

func (suite *ArangoClientTestSuite) TearDownSuite() {
	os.Unsetenv(propMicroserviceName)
	os.Unsetenv(namespaceEnvName)
}

func (suite *ArangoClientTestSuite) TestNewServiceDbaasClient_WithoutParams() {
	dbaasPool := dbaasbase.NewDbaaSPool()
	commonClient := NewClient(dbaasPool)
	serviceDB := commonClient.ServiceDatabase()
	assert.NotNil(suite.T(), serviceDB)
	db := serviceDB.(*arangoDatabase)
	ctx := context.Background()
	assert.Equal(suite.T(), ServiceClassifier(ctx), db.params.Classifier(ctx))
}

func (suite *ArangoClientTestSuite) TestNewServiceDbaasClient_WithParams() {
	dbaasPool := dbaasbase.NewDbaaSPool()
	commonClient := NewClient(dbaasPool)
	params := model.DbParams{
		Classifier:   stubClassifier,
		BaseDbParams: rest.BaseDbParams{},
	}
	serviceDB := commonClient.ServiceDatabase(params)
	assert.NotNil(suite.T(), serviceDB)
	db := serviceDB.(*arangoDatabase)
	ctx := context.Background()
	assert.Equal(suite.T(), stubClassifier(ctx), db.params.Classifier(ctx))
}

func (suite *ArangoClientTestSuite) TestNewTenantDbaasClient_WithoutParams() {
	dbaasPool := dbaasbase.NewDbaaSPool()
	commonClient := NewClient(dbaasPool)
	tenantDb := commonClient.TenantDatabase()
	assert.NotNil(suite.T(), tenantDb)
	db := tenantDb.(*arangoDatabase)
	ctx := createTenantContext()
	assert.Equal(suite.T(), TenantClassifier(ctx), db.params.Classifier(ctx))
}

func (suite *ArangoClientTestSuite) TestNewTenantDbaasClient_WithParams() {
	dbaasPool := dbaasbase.NewDbaaSPool()
	commonClient := NewClient(dbaasPool)
	params := model.DbParams{
		Classifier:   stubClassifier,
		BaseDbParams: rest.BaseDbParams{},
	}
	tenantDb := commonClient.TenantDatabase(params)
	assert.NotNil(suite.T(), tenantDb)
	db := tenantDb.(*arangoDatabase)
	ctx := context.Background()
	assert.Equal(suite.T(), stubClassifier(ctx), db.params.Classifier(ctx))
}

func (suite *ArangoClientTestSuite) TestCreateServiceClassifier() {
	expected := map[string]interface{}{
		"microserviceName": "test_service",
		"dbId":             "default",
		"namespace":        "test_space",
		"scope":            "service",
	}
	actual := ServiceClassifier(context.Background())
	assert.Equal(suite.T(), expected, actual)
}

func (suite *ArangoClientTestSuite) TestCreateTenantClassifier() {
	ctx := createTenantContext()
	expected := map[string]interface{}{
		"microserviceName": "test_service",
		"dbId":             "default",
		"namespace":        "test_space",
		"tenantId":         "123",
		"scope":            "tenant",
	}
	actual := TenantClassifier(ctx)
	assert.Equal(suite.T(), expected, actual)
}

func (suite *ArangoClientTestSuite) TestCreateTenantClassifier_WithoutTenantId() {
	ctx := context.Background()

	assert.Panics(suite.T(), func() {
		TenantClassifier(ctx)
	})
}

func TestArangoClient(t *testing.T) {
	suite.Run(t, new(ArangoClientTestSuite))
}

func stubClassifier(ctx context.Context) map[string]interface{} {
	return map[string]interface{}{
		"scope":            "service",
		"microserviceName": "service_test",
	}
}

func createTenantContext() context.Context {
	incomingHeaders := map[string]interface{}{tenant.TenantHeader: "123"}
	return ctxmanager.InitContext(context.Background(), incomingHeaders)
}
