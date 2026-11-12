package limits

import (
	"OzonBankTestTask/internal/domain"
	"OzonBankTestTask/internal/domain/page"
	"OzonBankTestTask/internal/graphql/gqlerrors"

	"github.com/99designs/gqlgen/graphql"
	"github.com/vektah/gqlparser/v2/ast"
)

// Config содержит уже проверенные положительные бюджеты одной операции.
type Config struct {
	MaxDepth      int
	MaxComplexity int
}

// Check выполняется после GraphQL-валидации и coercion переменных, до резолверов.
// Ранний выход при превышении бюджета не даёт fragments развернуться экспоненциально.
func Check(op *graphql.OperationContext, config Config) error {
	_, err := selectionCost(op.Operation.SelectionSet, op.Variables, 1, config)
	return err
}

func selectionCost(set ast.SelectionSet, variables map[string]any, depth int, config Config) (int, error) {
	total := 0
	for _, selection := range set {
		var cost int
		var err error
		switch value := selection.(type) {
		case *ast.Field:
			if skipSelection(value.Directives, variables) {
				continue
			}
			if depth > config.MaxDepth {
				return 0, gqlerrors.New("QUERY_TOO_DEEP", "превышена допустимая глубина запроса")
			}
			multiplier := 1
			if value.Name == "posts" || value.Name == "comments" || value.Name == "replies" {
				first, parseErr := graphql.UnmarshalInt(value.ArgumentMap(variables)["first"])
				if parseErr != nil || first < 1 || first > page.MaxPageSize {
					return 0, domain.ErrInvalidPageSize
				}
				multiplier = first
			}
			var child int
			child, err = selectionCost(value.SelectionSet, variables, depth+1, config)
			if err == nil {
				// Проверить перед умножением: сложность не может переполнить int.
				if child > (config.MaxComplexity-1)/multiplier {
					return 0, complexityError()
				}
				cost = 1 + multiplier*child
			}
		case *ast.InlineFragment:
			if skipSelection(value.Directives, variables) {
				continue
			}
			cost, err = selectionCost(value.SelectionSet, variables, depth, config)
			cost++
		case *ast.FragmentSpread:
			if skipSelection(value.Directives, variables) {
				continue
			}
			cost, err = selectionCost(value.Definition.SelectionSet, variables, depth, config)
			cost++
		}
		if err != nil {
			return 0, err
		}
		if cost > config.MaxComplexity-total {
			return 0, complexityError()
		}
		total += cost
	}
	return total, nil
}

func skipSelection(directives ast.DirectiveList, variables map[string]any) bool {
	for _, directive := range directives {
		if directive.Name != "skip" && directive.Name != "include" {
			continue
		}
		value, _ := directive.Arguments.ForName("if").Value.Value(variables)
		condition, _ := value.(bool)
		if directive.Name == "skip" && condition || directive.Name == "include" && !condition {
			return true
		}
	}
	return false
}

func complexityError() error {
	return gqlerrors.New("QUERY_TOO_COMPLEX", "превышена допустимая стоимость запроса")
}
