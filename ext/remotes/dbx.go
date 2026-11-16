package remotes

import "ganigeorgiev.com/pocketbase/dbx"

func dbxGTE(col, v string) dbx.Expression { return dbx.NewExp(col+" >= {:v}", dbx.Params{"v": v}) }
