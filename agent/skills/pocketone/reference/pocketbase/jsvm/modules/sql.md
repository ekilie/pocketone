Source: https://pocketbase.io/jsvm/modules/sql.html

# Namespace sql

Package sql provides a generic interface around SQL (or SQL-like) databases.

The sql package must be used in conjunction with a database driver. See [https://golang.org/s/sqldrivers](https://golang.org/s/sqldrivers) for a list of drivers.

Drivers that do not support context cancellation will not return until after the query is completed.

For usage examples, see the wiki page at [https://golang.org/s/sqlwiki](https://golang.org/s/sqlwiki).
