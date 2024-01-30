// Copyright (c) 2021 MagellanCL
//
// This software is released under the MIT License.
// https://opensource.org/licenses/MIT

package filter

import (
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type QueryParams struct {
	Filter         string `form:"filter"`
	Page           int    `form:"page,default=1"`
	Limit          int    `form:"limit,default=20"`
	All            bool   `form:"all,default=false"`
	OrderBy        string `form:"order_by,default=created_at"`
	OrderDirection string `form:"order_direction,default=desc,oneof=desc asc"`
}

const (
	//SEARCH   = 1  // NOT IMPLEMENTED // Filter response with LIKE query "search={search_phrase}"
	FILTER   = 2  // Filter response by column name values "{column_name}={value}"
	PAGINATE = 4  // Paginate response with page and page_size
	ORDER_BY = 8  // Order response by column name
	ALL      = 15 // Equivalent to SEARCH|FILTER|PAGINATE|ORDER_BY
	tagKey   = "filter"
)

var (
	columnNameRegexp = regexp.MustCompile(`(?m)column:(\w{1,}).*`)
	paramNameRegexp  = regexp.MustCompile(`(?m)param:(\w{1,}).*`)
)

func orderBy(db *gorm.DB, params QueryParams, table string) *gorm.DB {
	return db.Order(clause.OrderByColumn{
		Column: clause.Column{Name: table + "." + params.OrderBy},
		Desc:   params.OrderDirection == "desc"},
	)
}

var matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
var matchAllCap = regexp.MustCompile("([a-z0-9])([A-Z])")

func ToSnakeCase(str string) string {
	snake := matchFirstCap.ReplaceAllString(str, "${1}_${2}")
	snake = matchAllCap.ReplaceAllString(snake, "${1}_${2}")
	return strings.ToLower(snake)
}

func getColumnNameForField(field reflect.StructField) string {
	fieldTag := field.Tag.Get("gorm")
	res := columnNameRegexp.FindStringSubmatch(fieldTag)
	if len(res) == 2 {
		return ToSnakeCase(res[1])
	}
	return ToSnakeCase(field.Name)
}

func filterField(field reflect.StructField, key string, value string, separator string) clause.Expression {
	var paramName string
	if !strings.Contains(field.Tag.Get(tagKey), "filterable") {
		return nil
	}
	columnName := getColumnNameForField(field)
	paramMatch := paramNameRegexp.FindStringSubmatch(field.Tag.Get(tagKey))

	if len(paramMatch) == 2 {
		paramName = paramMatch[1]
		columnName = paramName
	} else {
		paramName = columnName
	}

	if paramName != key {
		return nil
	}

	switch separator {
	case eq:
		return clause.Eq{Column: columnName, Value: value}
	case neq:
		return clause.Neq{Column: columnName, Value: value}
	case gt:
		return clause.Gt{Column: columnName, Value: value}
	case gte:
		return clause.Gte{Column: columnName, Value: value}
	case lt:
		return clause.Lt{Column: columnName, Value: value}
	case lte:
		return clause.Lte{Column: columnName, Value: value}
	}

	return nil
}

func expressionByField(
	db *gorm.DB, values url.Values, modelType reflect.Type,
) *gorm.DB {
	numFields := modelType.NumField()
	expressions := make([]clause.Expression, 0, numFields)
	for key, array := range values {
		if key != "limit" && key != "page" && key != "order_by" && key != "desc" {
			for _, value := range array {
				key, value, separator := getSeparator(key, value)
				for i := 0; i < numFields; i++ {
					field := modelType.Field(i)
					expression := filterField(field, key, value, separator)
					if expression != nil {
						expressions = append(expressions, expression)
					}
				}
				if len(expressions) == 1 {
					db = db.Where(expressions[0])
				} else if len(expressions) > 1 {
					db = db.Where(clause.And(expressions...))
				}
			}
		}
	}

	return db
}

const (
	gte = ">="
	gt  = ">"
	lte = "<="
	lt  = "<"
	neq = "!="
	eq  = "="
)

var Separators = []string{
	gte,
	gt,
	lte,
	lt,
	neq,
	eq,
}

func getSeparator(key, value string) (string, string, string) {
	var arg string
	if value == "" {
		arg = key
	} else {
		arg = key + "=" + value
	}

	for _, separator := range Separators {
		res := strings.SplitN(arg, separator, 2)
		if len(res) > 1 {
			return res[0], res[1], separator
		}
	}

	return "", "", ""
}

// Use this function to paginate custom query.
// Example :
//
// db := vd.db.Model(&Customer{}).
//
//	Select("id, COUNT(orders.id)").
//	Joins("JOIN orders ON orders.customer_id = customers.id").
//	Where("orders.type = ?", orderSold)
//
// err = Paginate(c, db, params).
// Scan(&customType).Error
//
//	if err != nil {
//		return ret, err
//	}
func Paginate(c *gin.Context, db *gorm.DB, params QueryParams) *gorm.DB {
	var count int64
	db.Count(&count)
	if params.Page == 0 {
		params.Page = 1
	}

	switch {
	case params.Limit > 100:
		params.Limit = 100
	case params.Limit <= 0:
		params.Limit = 10
	}

	maxPage := count / int64(params.Limit)
	if count%int64(params.Limit) != 0 {
		maxPage++
	}

	c.Header("X-Paginate-Items", strconv.FormatInt(count, 10))
	c.Header("X-Paginate-Pages", strconv.FormatInt(maxPage, 10))
	c.Header("X-Paginate-Current", strconv.Itoa(params.Page))
	c.Header("X-Paginate-Limit", strconv.Itoa(params.Limit))

	offset := (params.Page - 1) * params.Limit
	return db.Offset(offset).Limit(params.Limit)
}

// Filter DB request with query parameters.
// Note: Don't forget to initialize DB Model first, otherwise filter and search won't work
// Example:
//
//	db.Model(&UserModel).Scope(filter.FilterByQuery(ctx, filter.ALL)).Find(&users)
//
// Or if only pagination and order is needed:
//
//	db.Model(&UserModel).Scope(filter.FilterByQuery(ctx, filter.PAGINATION|filter.ORDER_BY)).Find(&users)
//
// And models should have appropriate`filter` tags:
//
//	type User struct {
//		gorm.Model
//		Username string `gorm:"uniqueIndex" filter:"param:login;searchable;filterable"`
//		// `param` defines custom column name for the query param
//		FullName string `filter:"searchable"`
//	}
func FilterByQuery(c *gin.Context, config int) func(db *gorm.DB) *gorm.DB {
	return FilterByQueryWithCustomDefault(c, config, QueryParams{})
}

func FilterByQueryWithCustomDefault(c *gin.Context, config int, params QueryParams) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		setDefault(&params)
		err := c.BindQuery(&params)
		if err != nil {
			return nil
		}
		cleanParams := c.Request.URL.Query()

		model := db.Statement.Model
		modelType := reflect.TypeOf(model)
		if model != nil && modelType.Kind() == reflect.Ptr && modelType.Elem().Kind() == reflect.Struct {
			if config&FILTER > 0 {
				db = expressionByField(db, cleanParams, modelType.Elem())
			}
		}

		stmt := &gorm.Statement{DB: db}
		err = stmt.Parse(model)
		if err != nil {
			return nil
		}
		table := stmt.Schema.Table
		if config&PAGINATE > 0 {
			db = Paginate(c, db, params)
		}

		if config&ORDER_BY > 0 {
			db = orderBy(db, params, table)
		}

		return db
	}
}

func setDefault(p *QueryParams) {

	if p.Limit == 0 {
		p.Limit = 20
	}

	if p.Page == 0 {
		p.Page = 1
	}

	if p.OrderBy == "" {
		p.OrderBy = "created_at"
	}

	if p.OrderDirection == "" {
		p.OrderDirection = "desc"
	}
}
