package limits_test

import (
	"fmt"
	"strings"
	"testing"

	"OzonBankTestTask/internal/graphql/generated"
	"OzonBankTestTask/internal/graphql/limits"

	"github.com/99designs/gqlgen/graphql"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/gqlerror"
	"github.com/vektah/gqlparser/v2/validator"
)

func operation(t *testing.T, query string, variables map[string]any) *graphql.OperationContext {
	t.Helper()
	schema := generated.NewExecutableSchema(generated.Config{}).Schema()
	document, errs := gqlparser.LoadQuery(schema, query)
	require.Empty(t, errs)
	op := document.Operations[0]
	coerced, err := validator.VariableValues(schema, op, variables)
	require.NoError(t, err)
	return &graphql.OperationContext{Doc: document, Operation: op, Variables: coerced}
}

func assertCode(t *testing.T, err error, code string) {
	t.Helper()
	var gqlErr *gqlerror.Error
	require.ErrorAs(t, err, &gqlErr)
	require.Equal(t, code, gqlErr.Extensions["code"])
}

func TestDepthLimitIncludesFragments(t *testing.T) {
	op := operation(t, `query($id:ID!){ comment(id:$id){ ...Deep } }
		fragment Deep on Comment { replies(first:1){nodes{replies(first:1){nodes{id}}}} }`, map[string]any{"id": uuid.NewString()})
	err := limits.Check(op, limits.Config{MaxDepth: 5, MaxComplexity: 10_000})
	assertCode(t, err, "QUERY_TOO_DEEP")
}

func TestComplexityAccountsForPageProductsAndAliases(t *testing.T) {
	op := operation(t, `{ posts(first:100){nodes{comments(first:100){nodes{replies(first:100){nodes{id}}}}}} }`, nil)
	err := limits.Check(op, limits.Config{MaxDepth: 20, MaxComplexity: 10_000})
	assertCode(t, err, "QUERY_TOO_COMPLEX")
	op = operation(t, fmt.Sprintf(`{a:post(id:%q){id} b:post(id:%q){id}}`, uuid.NewString(), uuid.NewString()), nil)
	err = limits.Check(op, limits.Config{MaxDepth: 20, MaxComplexity: 3})
	assertCode(t, err, "QUERY_TOO_COMPLEX")
}

func TestComplexityDoesNotOverflow(t *testing.T) {
	query := fmt.Sprintf(`{comment(id:%q){`, uuid.NewString()) + strings.Repeat(`replies(first:100){nodes{`, 20) + `id` + strings.Repeat(`}}`, 20) + `}}`
	err := limits.Check(operation(t, query, nil), limits.Config{MaxDepth: 100, MaxComplexity: 10_000})
	assertCode(t, err, "QUERY_TOO_COMPLEX")
}

func TestRepeatedFragmentsHaveBoundedCostEvenWhenFieldsAreSkipped(t *testing.T) {
	query := fmt.Sprintf(`{post(id:%q){...F20}} fragment F0 on Post {id @skip(if:true)}`, uuid.NewString())
	for i := 1; i <= 20; i++ {
		query += fmt.Sprintf(` fragment F%d on Post {...F%d ...F%d}`, i, i-1, i-1)
	}
	err := limits.Check(operation(t, query, nil), limits.Config{MaxDepth: 20, MaxComplexity: 100})
	assertCode(t, err, "QUERY_TOO_COMPLEX")
}

func TestLimitsHonorConditionalDirectives(t *testing.T) {
	query := `query($id:ID!, $skip:Boolean!, $include:Boolean!){
		post(id:$id){id}
		posts(first:100) @skip(if:$skip){nodes{comments(first:100){nodes{id}}}}
		...Extra @include(if:$include)
	} fragment Extra on Query {posts(first:100){nodes{id}}}`
	op := operation(t, query, map[string]any{"id": uuid.NewString(), "skip": true, "include": false})
	require.NoError(t, limits.Check(op, limits.Config{MaxDepth: 2, MaxComplexity: 5}))
}
