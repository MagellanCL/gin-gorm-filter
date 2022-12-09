<!--
 Copyright (c) 2022 MagellanCL

 This software is released under the MIT License.
 https://opensource.org/licenses/MIT
-->

# Gin GORM filter
![GitHub](https://img.shields.io/github/license/Magellancl/gin-gorm-filter_v2)
![GitHub Workflow Status (branch)](https://img.shields.io/github/workflow/status/Magellancl/gin-gorm-filter_v2/CI/master)
![GitHub release (latest by date)](https://img.shields.io/github/v/release/Magellancl/gin-gorm-filter_v2)

Scope function for GORM queries provides easy filtering with query parameters

Fork of https://github.com/ActiveChooN/gin-gorm-filter

## Usage

```(shell)
go get github.com/magellancl/gin-gorm-filter_v2
```

## Model definition
```go
type UserModel struct {
    gorm.Model
    Username string `gorm:"uniqueIndex" filter:"filterable"`
    FullName string `filter:"param:full_name"`
    Role     string `filter:"filterable"`
	CreatedAt               time.Time      `filter:"filterable"`
	UpdatedAt               time.Time      `filter:"filterable"`
}
```
`param` tag defines custom column name and query param name.

## Controller Example
```go
func GetUsers(c *gin.Context) {
	var users []UserModel
	var usersCount int64
	db, err := gorm.Open(sqlite.Open("gorm.db"), &gorm.Config{})
	err := db.Model(&UserModel{}).Scopes(
		filter.FilterByQuery(c, filter.ALL),
	).Scan(&users).Error
	if err != nil {
		c.JSON(http.StatusBadRequest, err.Error())
		return
	}

	c.JSON(http.StatusOK, users)
}
```
Any filter combination can be used here `filter.PAGINATION|filter.ORDER_BY` e.g. **Important note:** GORM model should be initialized first for DB, otherwise filters won't work.

## FILTER

Using the tag `filter:"filterable"` on your gorm object, and activating it with `filter.FILTER`, you can make a field filterable.

The standard filter will use this format : `?username=john`.

You can use more complex filters with the separators <, >, >=, <=, !=. eg :

`?created_at>=2022-10-18&created_at<2022-10-21` (be careful of your timezone. You should be able to input any date format readable by your DBMS)

`?city!=grenoble`

`?price>10&created_at<2022-10-21`

## PAGINATE

Activating pagination with `filter.PAGINATE` will allow you to use the filters page and limit(eg : `?page=2&limit=50`). Limit maximum is 100, so you can request a maximum of 100 items at once. The default value is 20.
It will also renseign the following headers :

"X-Paginate-Items" -> total number of items\
"X-Paginate-Pages" -> total number of pages\
"X-Paginate-Current" -> current page\
"X-Paginate-Limit" -> limit of items per page

## ORDER BY



## Request example
```(shell)
curl -X GET http://localhost:8080/users?page=1&limit=10&order_by=username&order_direction=asc&name=John
```

## TODO list
- [x] Write tests for the lib with CI integration
- [ ] Add ILIKE integration for PostgreSQL database
- [X] Add other filters, like > or !=
