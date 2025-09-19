package arangodbaas

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/arangodb/go-driver/v2/arangodb"
	"github.com/arangodb/go-driver/v2/connection"
	"github.com/docker/go-connections/nat"
	"github.com/netcracker/qubership-core-lib-go-dbaas-arangodb-client/v4/model"
	dbaasbase "github.com/netcracker/qubership-core-lib-go-dbaas-base-client/v3"
	"github.com/netcracker/qubership-core-lib-go-dbaas-base-client/v3/cache"
	basemodel "github.com/netcracker/qubership-core-lib-go-dbaas-base-client/v3/model"
	"github.com/netcracker/qubership-core-lib-go-dbaas-base-client/v3/model/rest"
	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
	constants "github.com/netcracker/qubership-core-lib-go/v3/const"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	arangoDBImage        = "arangodb/arangodb:3.11.7"
	arangoDBPort         = "8529"
	baseArangoDBUsername = "test-username-"
	baseArangoDBPassword = "test-password-"
	baseArangoDBName     = "db-test-name-"

	arangoDBInitFileLocation = "/docker-entrypoint-initdb.d/init.js"
)

var (
	arangoDBNatPort, _ = nat.NewPort("tcp", arangoDBPort)
)

func TestSuite(t *testing.T) {
	suite.Run(t, new(ArangoDbClientTestSuite))
	suite.Run(t, new(MockArangoDbClientTestSuite))
}

func (suite *ArangoDbClientTestSuite) TestGetDatabase() {
	ctx := context.Background()
	database, err := suite.arangoDbClient.GetArangoDatabase(ctx, "1")
	assert.Nil(suite.T(), err)
	info, err := database.Info(ctx)
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), info)
}

func (suite *ArangoDbClientTestSuite) TestGetDatabaseFromCache() {
	ctx := context.Background()
	firstDatabase := suite.checkDatabaseConnection(ctx, "1")
	secondDatabase := suite.checkDatabaseConnection(ctx, "1")
	assert.Equal(suite.T(), firstDatabase.Name(), secondDatabase.Name())

	assert.Equal(suite.T(), 1, suite.dbProvider.GetOrCreateDbCalls)
	assert.Equal(suite.T(), 0, suite.dbProvider.GetConnectionCalls)
}

func (suite *ArangoDbClientTestSuite) TestPasswordChange() {
	ctx := context.Background()
	suite.checkDatabaseConnection(ctx, "1")
	assert.Equal(suite.T(), 1, suite.dbProvider.GetOrCreateDbCalls)
	assert.Equal(suite.T(), 0, suite.dbProvider.GetConnectionCalls)

	user, _ := suite.rootClient.User(ctx, baseArangoDBUsername+"1")
	newPassword := "new-password"
	user, err := suite.rootClient.UpdateUser(ctx, user.Name(), &arangodb.UserOptions{Password: newPassword})
	suite.dbProvider.PasswordOverride = newPassword
	assert.NotNil(suite.T(), user)
	assert.Nil(suite.T(), err)

	suite.checkDatabaseConnection(ctx, "1")
	assert.Equal(suite.T(), 1, suite.dbProvider.GetOrCreateDbCalls)
	assert.Equal(suite.T(), 1, suite.dbProvider.GetConnectionCalls)
}

func (suite *ArangoDbClientTestSuite) TestGetDifferentDatabases() {
	ctx := context.Background()
	firstDatabase := suite.checkDatabaseConnection(ctx, "1")
	secondDatabase := suite.checkDatabaseConnection(ctx, "2")
	assert.NotEqual(suite.T(), firstDatabase.Name(), secondDatabase.Name())
}

func (suite *ArangoDbClientTestSuite) checkDatabaseConnection(ctx context.Context, dbName string) arangodb.Database {
	database, err := suite.arangoDbClient.GetArangoDatabase(ctx, dbName)
	assert.Nil(suite.T(), err)
	info, err := database.Info(ctx)
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), info)
	return database
}

type ArangoDbClientTestSuite struct {
	suite.Suite
	arangoDBContainer testcontainers.Container
	arangoDBHost      string
	arangoDBPort      float64
	arangoDbClient    ArangoDbClient
	dbProvider        *TestLogicalDbProvider
	rootClient        arangodb.Client
}

func (suite *ArangoDbClientTestSuite) SetupSuite() {
	ctx := context.Background()
	suite.prepareTestContainer(ctx)
}

func (suite *ArangoDbClientTestSuite) TearDownSuite() {
	err := suite.arangoDBContainer.Terminate(context.Background())
	if err != nil {
		suite.T().Fatal(err)
	}
}

func (suite *ArangoDbClientTestSuite) BeforeTest(suiteName, testName string) {
	suite.dbProvider = &TestLogicalDbProvider{suite.arangoDBHost, suite.arangoDBPort, 0, 0, ""}
	dbaasPool := dbaasbase.NewDbaaSPool(basemodel.PoolOptions{LogicalDbProviders: []basemodel.LogicalDbProvider{
		suite.dbProvider,
	}})
	suite.arangoDbClient = &ArangoDbClientImpl{
		httpConfig:    &connection.HttpConfiguration{},
		dbaasClient:   dbaasPool.Client,
		arangodbCache: &cache.DbaaSCache{LogicalDbCache: make(map[cache.Key]interface{})},
		params:        model.DbParams{Classifier: ServiceClassifier},
	}

	ctx := context.Background()
	user, _ := suite.rootClient.User(ctx, "test-username-1")
	suite.rootClient.UpdateUser(ctx, user.Name(), &arangodb.UserOptions{Password: "test-password-1"})

	user, _ = suite.rootClient.User(ctx, "test-username-2")
	suite.rootClient.UpdateUser(ctx, user.Name(), &arangodb.UserOptions{Password: "test-password-2"})
}

func (suite *ArangoDbClientTestSuite) prepareTestContainer(ctx context.Context) {
	arangoDBInitFile, _ := ioutil.TempFile("", "init.js")
	arangoDBInitScript, _ := os.ReadFile("./test-resources/init.js")
	arangoDBInitFile.Write(arangoDBInitScript)
	arangoDBInitFile.Close()

	env := make(map[string]string)
	env["ARANGO_ROOT_PASSWORD"] = "root"

	os.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true")

	req := testcontainers.ContainerRequest{
		Image:        arangoDBImage,
		ExposedPorts: []string{arangoDBNatPort.Port()},
		WaitingFor:   NewArangoDBWaitStrategy(time.Minute, time.Second),
		Mounts: testcontainers.Mounts(
			testcontainers.BindMount(arangoDBInitFile.Name(), arangoDBInitFileLocation),
		),
		Env: env,
	}
	var err error
	suite.arangoDBContainer, err = testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          false,
	})
	if err != nil {
		suite.T().Fatal(err)
	}
	err = suite.arangoDBContainer.Start(ctx)
	if err != nil {
		suite.T().Fatal(err)
	}
	arangoDBHost, err := suite.arangoDBContainer.Host(ctx)
	if err != nil {
		suite.T().Fatal(err)
	}
	arangoDBPort, err := suite.arangoDBContainer.MappedPort(ctx, arangoDBNatPort)
	if err != nil {
		suite.T().Fatal(err)
	}

	suite.arangoDBHost = arangoDBHost
	suite.arangoDBPort = float64(arangoDBPort.Int())
	suite.rootClient = getRootClient(arangoDBHost, arangoDBPort.Int())

	os.Unsetenv("TESTCONTAINERS_RYUK_DISABLED")
}

type arangoDBWaitStrategy struct {
	waitDuration  time.Duration
	checkInterval time.Duration
}

func (c arangoDBWaitStrategy) WaitUntilReady(ctx context.Context, target wait.StrategyTarget) (err error) {
	host, err := target.Host(ctx)
	if err != nil {
		return
	}
	port, err := target.MappedPort(ctx, arangoDBNatPort)
	if err != nil {
		return
	}
	return waitForArangoDBStart(ctx, c.waitDuration, c.checkInterval, host, port.Int())
}

func NewArangoDBWaitStrategy(waitDuration time.Duration, checkInterval time.Duration) *arangoDBWaitStrategy {
	return &arangoDBWaitStrategy{waitDuration, checkInterval}
}

func waitForArangoDBStart(ctx context.Context, waitDuration, checkInterval time.Duration, host string, port int) error {
	ctx, cancelContext := context.WithTimeout(ctx, waitDuration)
	defer cancelContext()

	client := getRootClient(host, port)

	err := waitForDb(ctx, client, "db-test-name-1", checkInterval)
	if err != nil {
		return err
	}
	err = waitForDb(ctx, client, "db-test-name-2", checkInterval)
	if err != nil {
		return err
	}
	return nil
}

func getRootClient(host string, port int) arangodb.Client {
	endpoints := []string{fmt.Sprintf("http://%s:%d", host, port)}
	httpConfig := connection.HttpConfiguration{
		Endpoint:       connection.NewRoundRobinEndpoints(endpoints),
		Authentication: connection.NewBasicAuth("root", "root"),
	}
	conn := connection.NewHttpConnection(httpConfig)
	client := arangodb.NewClient(conn)

	return client
}

func waitForDb(ctx context.Context, client arangodb.Client, dbName string, checkInterval time.Duration) error {
	_, err := client.GetDatabase(ctx, dbName, nil)
	for err != nil {
		select {
		case <-ctx.Done():
			return fmt.Errorf("%s:%w", ctx.Err(), err)
		case <-time.After(checkInterval):
			_, err = client.GetDatabase(ctx, dbName, nil)
		}
	}
	return nil
}

type TestLogicalDbProvider struct {
	host               string
	port               float64
	GetOrCreateDbCalls int
	GetConnectionCalls int
	PasswordOverride   string
}

func (p *TestLogicalDbProvider) GetOrCreateDb(dbType string, classifier map[string]interface{}, params rest.BaseDbParams) (*basemodel.LogicalDb, error) {
	p.GetOrCreateDbCalls++
	return &basemodel.LogicalDb{
		Id:                   "123",
		ConnectionProperties: p.getConnectionProperties(classifier),
	}, nil
}

func (p *TestLogicalDbProvider) GetConnection(dbType string, classifier map[string]interface{}, params rest.BaseDbParams) (map[string]interface{}, error) {
	p.GetConnectionCalls++
	return p.getConnectionProperties(classifier), nil
}

func (p TestLogicalDbProvider) getConnectionProperties(classifier map[string]interface{}) map[string]interface{} {
	dbId := classifier["dbId"].(string)
	connectionProperties := map[string]interface{}{
		"host":     p.host,
		"port":     p.port,
		"username": baseArangoDBUsername + dbId,
		"dbName":   baseArangoDBName + dbId,
	}
	if p.PasswordOverride != "" {
		connectionProperties["password"] = p.PasswordOverride
	} else {
		connectionProperties["password"] = baseArangoDBPassword + dbId
	}
	return connectionProperties
}

//go:generate mockery --with-expecter --srcpkg github.com/arangodb/go-driver/v2/arangodb --name=Client --output mocks --filename arangodb_driver_client.go
//go:generate mockery --with-expecter --srcpkg github.com/arangodb/go-driver/v2/arangodb --name=Database --output mocks --filename arangodb_driver_database.go
type MockArangoDbClientTestSuite struct {
	suite.Suite
}

func (suite *MockArangoDbClientTestSuite) SetupSuite() {
	os.Setenv(constants.MicroserviceNameProperty, "test_service")
	os.Setenv(constants.NamespaceProperty, "test_space")

	configloader.Init(configloader.EnvPropertySource())
}

func (suite *MockArangoDbClientTestSuite) TearDownSuite() {
	os.Unsetenv(constants.MicroserviceNameProperty)
	os.Unsetenv(constants.NamespaceProperty)
}
