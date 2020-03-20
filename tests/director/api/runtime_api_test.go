package api

import (
	"context"
	"fmt"
	"testing"

	"github.com/kyma-incubator/compass/tests/director/pkg/ptr"

	"github.com/kyma-incubator/compass/components/director/pkg/graphql"
	gcli "github.com/machinebox/graphql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	scenariosLabel          = "scenarios"
	queryRuntimesCategory   = "query runtimes"
	registerRuntimeCategory = "register runtime"
)

func TestRuntimeRegisterUpdateAndUnregister(t *testing.T) {
	// GIVEN
	ctx := context.Background()
	givenInput := graphql.RuntimeInput{
		Name:        "runtime-create-update-delete",
		Description: ptr.String("runtime-1-description"),
		Labels:      &graphql.Labels{"ggg": []interface{}{"hhh"}},
	}
	runtimeInGQL, err := tc.graphqlizer.RuntimeInputToGQL(givenInput)
	require.NoError(t, err)
	actualRuntime := graphql.RuntimeExt{}

	// WHEN
	registerReq := gcli.NewRequest(
		fmt.Sprintf(`mutation {
			result: registerRuntime(in: %s) {
					%s
				}
			}`, runtimeInGQL, tc.gqlFieldsProvider.ForRuntime()))
	saveExampleInCustomDir(t, registerReq.Query(), registerRuntimeCategory, "register runtime")
	err = tc.RunOperation(ctx, registerReq, &actualRuntime)

	//THEN
	require.NoError(t, err)
	require.NotEmpty(t, actualRuntime.ID)
	assertRuntime(t, givenInput, actualRuntime)

	// add Label
	actualLabel := graphql.Label{}

	// WHEN
	addLabelReq := gcli.NewRequest(
		fmt.Sprintf(`mutation {
			result: setRuntimeLabel(runtimeID: "%s", key: "%s", value: %s) {
					%s
				}
			}`, actualRuntime.ID, "new_label", "[\"bbb\"]", tc.gqlFieldsProvider.ForLabel()))
	err = tc.RunOperation(ctx, addLabelReq, &actualLabel)

	//THEN
	require.NoError(t, err)
	assert.Equal(t, "new_label", actualLabel.Key)
	assert.Len(t, actualLabel.Value, 1)
	assert.Contains(t, actualLabel.Value, "bbb")

	// get runtime and validate runtimes
	getRuntimeReq := gcli.NewRequest(
		fmt.Sprintf(`query {
			result: runtime(id: "%s") {
					%s
				}
			}`, actualRuntime.ID, tc.gqlFieldsProvider.ForRuntime()))
	err = tc.RunOperation(ctx, getRuntimeReq, &actualRuntime)
	require.NoError(t, err)
	assert.Len(t, actualRuntime.Labels, 3)

	// add agent auth
	// GIVEN
	in := fixSampleApplicationRegisterInput("app")

	appInputGQL, err := tc.graphqlizer.ApplicationRegisterInputToGQL(in)
	require.NoError(t, err)
	createAppReq := gcli.NewRequest(
		fmt.Sprintf(`mutation {
  				result: registerApplication(in: %s) {
    					%s
					}
				}`, appInputGQL, tc.gqlFieldsProvider.ForApplication()))
	actualApp := graphql.ApplicationExt{}

	//WHEN
	err = tc.RunOperation(ctx, createAppReq, &actualApp)

	//THEN
	require.NoError(t, err)
	require.NotEmpty(t, actualApp.ID)
	defer unregisterApplication(t, actualApp.ID)

	// set Auth
	// GIVEN
	authIn := graphql.AuthInput{
		Credential: &graphql.CredentialDataInput{
			Basic: &graphql.BasicCredentialDataInput{
				Username: "x-men",
				Password: "secret",
			}}}
	actualAPIRuntimeAuth := graphql.APIRuntimeAuth{}

	authInStr, err := tc.graphqlizer.AuthInputToGQL(&authIn)
	require.NoError(t, err)
	setAuthReq := gcli.NewRequest(
		fmt.Sprintf(`mutation {
			result: setAPIAuth(apiID: "%s", runtimeID: "%s", in: %s) {
					%s
				}
			}`, actualApp.APIDefinitions.Data[0].ID, actualRuntime.ID, authInStr, tc.gqlFieldsProvider.ForAPIRuntimeAuth()))

	//WHEN
	err = tc.RunOperation(ctx, setAuthReq, &actualAPIRuntimeAuth)

	//THEN
	require.NoError(t, err)
	require.NotNil(t, actualAPIRuntimeAuth.Auth)
	assert.Equal(t, actualRuntime.ID, actualAPIRuntimeAuth.RuntimeID)
	actualBasic, ok := actualAPIRuntimeAuth.Auth.Credential.(*graphql.BasicCredentialData)
	require.True(t, ok)
	assert.Equal(t, "x-men", actualBasic.Username)
	assert.Equal(t, "secret", actualBasic.Password)

	// update runtime, check if only simple values are updated
	//GIVEN
	givenInput.Name = "updated-name"
	givenInput.Description = ptr.String("updated-description")
	givenInput.Labels = &graphql.Labels{
		"key": []interface{}{"values", "aabbcc"},
	}
	runtimeInGQL, err = tc.graphqlizer.RuntimeInputToGQL(givenInput)
	require.NoError(t, err)
	//actualRuntime = RuntimeExt{ID: actualRuntime.ID}
	updateRuntimeReq := gcli.NewRequest(
		fmt.Sprintf(`mutation {
				result: updateRuntime(id: "%s", in: %s) {
					%s
				}
		}
		`, actualRuntime.ID, runtimeInGQL, tc.gqlFieldsProvider.ForRuntime()))
	saveExample(t, updateRuntimeReq.Query(), "update runtime")
	//WHEN
	err = tc.RunOperation(ctx, updateRuntimeReq, &actualRuntime)

	//THEN
	require.NoError(t, err)
	assert.Equal(t, givenInput.Name, actualRuntime.Name)
	assert.Equal(t, *givenInput.Description, *actualRuntime.Description)

	// delete runtime

	// WHEN
	delReq := gcli.NewRequest(fmt.Sprintf(`mutation{result: unregisterRuntime(id: "%s") {%s}}`, actualRuntime.ID, tc.gqlFieldsProvider.ForRuntime()))
	saveExample(t, delReq.Query(), "unregister runtime")
	err = tc.RunOperation(ctx, delReq, nil)

	//THEN
	require.NoError(t, err)
}

func TestRuntimeCreateUpdateDuplicatedNames(t *testing.T) {
	// GIVEN
	ctx := context.Background()
	firstRuntimeName := "unique-name-1"
	givenInput := graphql.RuntimeInput{
		Name:        firstRuntimeName,
		Description: ptr.String("runtime-1-description"),
		Labels:      &graphql.Labels{"ggg": []interface{}{"hhh"}},
	}
	runtimeInGQL, err := tc.graphqlizer.RuntimeInputToGQL(givenInput)
	require.NoError(t, err)
	firstRuntime := graphql.RuntimeExt{}
	registerReq := gcli.NewRequest(
		fmt.Sprintf(`mutation {
			result: registerRuntime(in: %s) {
					%s
				}
			}`, runtimeInGQL, tc.gqlFieldsProvider.ForRuntime()))
	// WHEN
	err = tc.RunOperation(ctx, registerReq, &firstRuntime)

	//THEN
	require.NoError(t, err)
	require.NotEmpty(t, firstRuntime.ID)
	assertRuntime(t, givenInput, firstRuntime)
	defer unregisterRuntime(t, firstRuntime.ID)

	// try to create second runtime with first runtime name
	//GIVEN
	givenInput = graphql.RuntimeInput{
		Name:        firstRuntimeName,
		Description: ptr.String("runtime-1-description"),
	}
	runtimeInGQL, err = tc.graphqlizer.RuntimeInputToGQL(givenInput)
	require.NoError(t, err)
	registerReq = gcli.NewRequest(
		fmt.Sprintf(`mutation {
			result: registerRuntime(in: %s) {
					%s
				}
			}`, runtimeInGQL, tc.gqlFieldsProvider.ForRuntime()))
	saveExampleInCustomDir(t, registerReq.Query(), registerRuntimeCategory, "register runtime")

	// WHEN
	err = tc.RunOperation(ctx, registerReq, nil)

	//THEN
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not unique")

	// create second runtime
	//GIVEN
	secondRuntimeName := "unique-name-2"
	givenInput = graphql.RuntimeInput{
		Name:        secondRuntimeName,
		Description: ptr.String("runtime-1-description"),
		Labels:      &graphql.Labels{"ggg": []interface{}{"hhh"}},
	}
	runtimeInGQL, err = tc.graphqlizer.RuntimeInputToGQL(givenInput)
	require.NoError(t, err)
	secondRuntime := graphql.RuntimeExt{}
	registerReq = gcli.NewRequest(
		fmt.Sprintf(`mutation {
			result: registerRuntime(in: %s) {
					%s
				}
			}`, runtimeInGQL, tc.gqlFieldsProvider.ForRuntime()))

	// WHEN
	err = tc.RunOperation(ctx, registerReq, &secondRuntime)

	//THEN
	require.NoError(t, err)
	require.NotEmpty(t, secondRuntime.ID)
	assertRuntime(t, givenInput, secondRuntime)
	defer unregisterRuntime(t, secondRuntime.ID)

	//Update first runtime with second runtime name, failed

	//GIVEN
	givenInput = graphql.RuntimeInput{
		Name:        secondRuntimeName,
		Description: ptr.String("runtime-1-description"),
	}
	runtimeInGQL, err = tc.graphqlizer.RuntimeInputToGQL(givenInput)
	require.NoError(t, err)
	registerReq = gcli.NewRequest(
		fmt.Sprintf(`mutation {
			result: updateRuntime(id: "%s", in :%s) {
					%s
				}
			}`, firstRuntime.ID, runtimeInGQL, tc.gqlFieldsProvider.ForRuntime()))

	// WHEN
	err = tc.RunOperation(ctx, registerReq, &secondRuntime)

	//THEN
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not unique")
}

func TestSetAndDeleteAPIAuth(t *testing.T) {
	// GIVEN
	// create application
	ctx := context.Background()
	placeholder := "app"
	in := fixSampleApplicationRegisterInput(placeholder)

	appInputGQL, err := tc.graphqlizer.ApplicationRegisterInputToGQL(in)
	require.NoError(t, err)
	registerReq := gcli.NewRequest(
		fmt.Sprintf(`mutation {
  				result: registerApplication(in: %s) {
    					%s
					}
				}`, appInputGQL, tc.gqlFieldsProvider.ForApplication()))
	actualApp := graphql.ApplicationExt{}
	err = tc.RunOperation(ctx, registerReq, &actualApp)
	require.NoError(t, err)
	require.NotEmpty(t, actualApp.ID)
	defer unregisterApplication(t, actualApp.ID)

	// create runtime
	runtimeInput := graphql.RuntimeInput{
		Name:        "runtime-set-delete-api",
		Description: ptr.String("runtime-1-description"),
	}
	runtimeInGQL, err := tc.graphqlizer.RuntimeInputToGQL(runtimeInput)
	require.NoError(t, err)
	actualRuntime := graphql.Runtime{}
	registerRuntimeReq := gcli.NewRequest(
		fmt.Sprintf(`mutation {
				result: registerRuntime(in: %s) {
						%s
					}
				}`, runtimeInGQL, tc.gqlFieldsProvider.ForRuntime()))
	err = tc.RunOperation(ctx, registerRuntimeReq, &actualRuntime)
	require.NoError(t, err)
	require.NotEmpty(t, actualRuntime.ID)

	defer unregisterRuntime(t, actualRuntime.ID)

	actualAPIRuntimeAuth := graphql.APIRuntimeAuth{}

	// WHEN
	// set Auth
	authIn := graphql.AuthInput{
		Credential: &graphql.CredentialDataInput{
			Basic: &graphql.BasicCredentialDataInput{
				Username: "x-men",
				Password: "secret",
			}}}

	authInStr, err := tc.graphqlizer.AuthInputToGQL(&authIn)
	require.NoError(t, err)
	setAuthReq := gcli.NewRequest(
		fmt.Sprintf(`mutation {
			result: setAPIAuth(apiID: "%s", runtimeID: "%s", in: %s) {
					%s
				}
			}`, actualApp.APIDefinitions.Data[0].ID, actualRuntime.ID, authInStr, tc.gqlFieldsProvider.ForAPIRuntimeAuth()))
	err = tc.RunOperation(ctx, setAuthReq, &actualAPIRuntimeAuth)

	//THEN
	require.NoError(t, err)
	require.NotNil(t, actualAPIRuntimeAuth.Auth)
	assert.Equal(t, actualRuntime.ID, actualAPIRuntimeAuth.RuntimeID)
	actualBasic, ok := actualAPIRuntimeAuth.Auth.Credential.(*graphql.BasicCredentialData)
	require.True(t, ok)
	assert.Equal(t, "x-men", actualBasic.Username)
	assert.Equal(t, "secret", actualBasic.Password)

	// delete Auth
	delAuthReq := gcli.NewRequest(
		fmt.Sprintf(`mutation {
			result: deleteAPIAuth(apiID: "%s",runtimeID: "%s") {
					%s
				} 
			}`, actualApp.APIDefinitions.Data[0].ID, actualRuntime.ID, tc.gqlFieldsProvider.ForAPIRuntimeAuth()))
	err = tc.RunOperation(ctx, delAuthReq, nil)
	require.NoError(t, err)
}

func TestQueryRuntimes(t *testing.T) {
	// GIVEN
	ctx := context.Background()

	idsToRemove := make([]string, 0)
	defer func() {
		for _, id := range idsToRemove {
			if id != "" {
				unregisterRuntime(t, id)
			}
		}
	}()

	inputRuntimes := []*graphql.Runtime{
		{Name: "runtime-query-1", Description: ptr.String("test description")},
		{Name: "runtime-query-2", Description: ptr.String("another description")},
		{Name: "runtime-query-3"},
	}

	for _, rtm := range inputRuntimes {
		givenInput := graphql.RuntimeInput{
			Name:        rtm.Name,
			Description: rtm.Description,
		}
		runtimeInGQL, err := tc.graphqlizer.RuntimeInputToGQL(givenInput)
		require.NoError(t, err)
		createReq := fixRegisterRuntimeRequest(runtimeInGQL)
		actualRuntime := graphql.Runtime{}
		err = tc.RunOperation(ctx, createReq, &actualRuntime)
		require.NoError(t, err)
		require.NotEmpty(t, actualRuntime.ID)
		rtm.ID = actualRuntime.ID
		idsToRemove = append(idsToRemove, actualRuntime.ID)
	}
	actualPage := graphql.RuntimePage{}

	// WHEN
	queryReq := gcli.NewRequest(
		fmt.Sprintf(`query {
			result: runtimes {
					%s
				}
			}`, tc.gqlFieldsProvider.Page(tc.gqlFieldsProvider.ForRuntime())))
	err := tc.RunOperation(ctx, queryReq, &actualPage)
	saveExampleInCustomDir(t, queryReq.Query(), queryRuntimesCategory, "query runtimes")

	//THEN
	require.NoError(t, err)
	assert.Len(t, actualPage.Data, len(inputRuntimes))
	assert.Equal(t, len(inputRuntimes), actualPage.TotalCount)

	for _, inputRtm := range inputRuntimes {
		found := false
		for _, actualRtm := range actualPage.Data {
			if inputRtm.ID == actualRtm.ID {
				found = true
				assert.Equal(t, inputRtm.Name, actualRtm.Name)
				assert.Equal(t, inputRtm.Description, actualRtm.Description)
				break
			}
		}
		assert.True(t, found)
	}
}

func TestQuerySpecificRuntime(t *testing.T) {
	// GIVEN
	ctx := context.Background()
	givenInput := graphql.RuntimeInput{
		Name: "runtime-specific-runtime",
	}
	runtimeInGQL, err := tc.graphqlizer.RuntimeInputToGQL(givenInput)
	require.NoError(t, err)
	registerReq := gcli.NewRequest(
		fmt.Sprintf(`mutation {
			result: registerRuntime(in: %s) {
					%s
				}
			}`, runtimeInGQL, tc.gqlFieldsProvider.ForRuntime()))
	createdRuntime := graphql.Runtime{}
	err = tc.RunOperation(ctx, registerReq, &createdRuntime)
	require.NoError(t, err)
	require.NotEmpty(t, createdRuntime.ID)

	defer unregisterRuntime(t, createdRuntime.ID)
	queriedRuntime := graphql.Runtime{}

	// WHEN
	queryReq := gcli.NewRequest(
		fmt.Sprintf(`query {
			result: runtime(id: "%s") {
					%s
				}
			}`, createdRuntime.ID, tc.gqlFieldsProvider.ForRuntime()))
	err = tc.RunOperation(ctx, queryReq, &queriedRuntime)
	saveExample(t, queryReq.Query(), "query runtime")

	//THEN
	require.NoError(t, err)
	assert.Equal(t, createdRuntime.ID, queriedRuntime.ID)
	assert.Equal(t, createdRuntime.Name, queriedRuntime.Name)
	assert.Equal(t, createdRuntime.Description, queriedRuntime.Description)
}

func TestApplicationsForRuntime(t *testing.T) {
	//GIVEN
	ctx := context.Background()
	tenantID := testTenants.GetIDByName(t, "Test1")
	otherTenant := testTenants.GetIDByName(t, "Test2")
	tenantApplications := []*graphql.Application{}
	defaultValue := "DEFAULT"
	scenarios := []string{defaultValue, "black-friday-campaign", "christmas-campaign", "summer-campaign"}

	jsonSchema := map[string]interface{}{
		"type":        "array",
		"minItems":    1,
		"uniqueItems": true,
		"items": map[string]interface{}{
			"type": "string",
			"enum": scenarios,
		},
	}
	var schema interface{} = jsonSchema

	createLabelDefinitionWithinTenant(t, ctx, scenariosLabel, schema, tenantID)
	createLabelDefinitionWithinTenant(t, ctx, scenariosLabel, schema, otherTenant)

	applications := []struct {
		ApplicationName string
		Tenant          string
		WithinTenant    bool
		Scenarios       []string
	}{
		{
			Tenant:          tenantID,
			ApplicationName: "first",
			WithinTenant:    true,
			Scenarios:       []string{defaultValue},
		},
		{
			Tenant:          tenantID,
			ApplicationName: "second",
			WithinTenant:    true,
			Scenarios:       []string{defaultValue, "black-friday-campaign"},
		},
		{
			Tenant:          tenantID,
			ApplicationName: "third",
			WithinTenant:    true,
			Scenarios:       []string{"black-friday-campaign", "christmas-campaign", "summer-campaign"},
		},
		{
			Tenant:          tenantID,
			ApplicationName: "allscenarios",
			WithinTenant:    true,
			Scenarios:       []string{defaultValue, "black-friday-campaign", "christmas-campaign", "summer-campaign"},
		},
		{
			Tenant:          otherTenant,
			ApplicationName: "test",
			WithinTenant:    false,
			Scenarios:       []string{defaultValue, "black-friday-campaign"},
		},
	}

	for _, testApp := range applications {
		applicationInput := fixSampleApplicationRegisterInput(testApp.ApplicationName)
		applicationInput.Labels = &graphql.Labels{scenariosLabel: testApp.Scenarios}
		appInputGQL, err := tc.graphqlizer.ApplicationRegisterInputToGQL(applicationInput)
		require.NoError(t, err)

		createApplicationReq := fixRegisterApplicationRequest(appInputGQL)
		application := graphql.Application{}

		err = tc.RunOperationWithCustomTenant(ctx, testApp.Tenant, createApplicationReq, &application)

		require.NoError(t, err)
		require.NotEmpty(t, application.ID)

		defer unregisterApplicationInTenant(t, application.ID, testApp.Tenant)
		if testApp.WithinTenant {
			tenantApplications = append(tenantApplications, &application)
		}
	}

	//create runtime
	runtimeInput := fixRuntimeInput("runtime")
	(*runtimeInput.Labels)[scenariosLabel] = scenarios
	runtimeInputGQL, err := tc.graphqlizer.RuntimeInputToGQL(runtimeInput)
	require.NoError(t, err)
	registerRuntimeRequest := fixRegisterRuntimeRequest(runtimeInputGQL)
	runtime := graphql.Runtime{}
	err = tc.RunOperationWithCustomTenant(ctx, tenantID, registerRuntimeRequest, &runtime)
	require.NoError(t, err)
	require.NotEmpty(t, runtime.ID)
	defer unregisterRuntimeWithinTenant(t, runtime.ID, tenantID)

	//WHEN
	request := fixApplicationForRuntimeRequest(runtime.ID)
	applicationPage := graphql.ApplicationPage{}

	err = tc.RunOperationWithCustomTenant(ctx, tenantID, request, &applicationPage)
	saveExample(t, request.Query(), "query applications for runtime")

	//THEN
	require.NoError(t, err)
	require.Len(t, applicationPage.Data, len(tenantApplications))
	assert.ElementsMatch(t, tenantApplications, applicationPage.Data)
}

func TestQueryRuntimesWithPagination(t *testing.T) {
	//GIVEN
	ctx := context.Background()
	runtimes := make(map[string]*graphql.Runtime)
	runtimesAmount := 10
	for i := 0; i < runtimesAmount; i++ {
		runtimeInput := graphql.RuntimeInput{
			Name: fmt.Sprintf("runtime-%d", i),
		}
		runtimeInputGQL, err := tc.graphqlizer.RuntimeInputToGQL(runtimeInput)
		require.NoError(t, err)

		registerReq := fixRegisterRuntimeRequest(runtimeInputGQL)

		runtime := graphql.Runtime{}
		err = tc.RunOperation(ctx, registerReq, &runtime)

		require.NoError(t, err)
		require.NotEmpty(t, runtime.ID)
		defer unregisterRuntime(t, runtime.ID)
		runtimes[runtime.ID] = &runtime
	}

	after := 3
	cursor := ""
	queriesForFullPage := int(runtimesAmount / after)

	for i := 0; i < queriesForFullPage; i++ {
		runtimesRequest := fixRuntimeRequestWithPaginationRequest(after, cursor)

		//WHEN
		runtimePage := graphql.RuntimePage{}
		err := tc.RunOperation(ctx, runtimesRequest, &runtimePage)
		require.NoError(t, err)

		//THEN
		assert.Equal(t, cursor, string(runtimePage.PageInfo.StartCursor))
		assert.True(t, runtimePage.PageInfo.HasNextPage)
		assert.Len(t, runtimePage.Data, after)
		assert.Equal(t, runtimesAmount, runtimePage.TotalCount)
		for _, runtime := range runtimePage.Data {
			assert.Equal(t, runtime, runtimes[runtime.ID])
			delete(runtimes, runtime.ID)
		}
		cursor = string(runtimePage.PageInfo.EndCursor)
	}

	//WHEN get last page with last runtime
	runtimesRequest := fixRuntimeRequestWithPaginationRequest(after, cursor)
	lastRuntimePage := graphql.RuntimePage{}
	err := tc.RunOperation(ctx, runtimesRequest, &lastRuntimePage)
	require.NoError(t, err)
	saveExampleInCustomDir(t, runtimesRequest.Query(), queryRuntimesCategory, "query runtimes with pagination")

	//THEN
	assert.False(t, lastRuntimePage.PageInfo.HasNextPage)
	assert.Empty(t, lastRuntimePage.PageInfo.EndCursor)
	require.Len(t, lastRuntimePage.Data, 1)
	assert.Equal(t, lastRuntimePage.Data[0], runtimes[lastRuntimePage.Data[0].ID])
	delete(runtimes, lastRuntimePage.Data[0].ID)
	assert.Len(t, runtimes, 0)
}

func TestRegisterRuntimeWithoutLabels(t *testing.T) {
	//GIVEN
	ctx := context.TODO()
	name := "test-create-runtime-without-labels"
	runtimeInput := graphql.RuntimeInput{Name: name}

	runtime := registerRuntimeFromInput(t, ctx, &runtimeInput)
	defer unregisterRuntime(t, runtime.ID)

	//WHEN
	fetchedRuntime := getRuntime(t, ctx, runtime.ID)

	//THEN
	require.Equal(t, runtime.ID, fetchedRuntime.ID)
	assertRuntime(t, runtimeInput, *fetchedRuntime)
}
