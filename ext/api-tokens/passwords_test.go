package apitokens

import (
	"testing"

	"ganigeorgiev.com/pocketbase/dbx"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"golang.org/x/crypto/bcrypt"
)

// fastPasswords rehashes the test data's passwords at bcrypt's lowest cost. Every auth record tests.NewTestApp copies
// has the password "1234567890" hashed at cost 13, which takes ~0.6 s to check, and ~6 s under -race (CI's tests), on
// every password sign-in. Direct SQL, so no hook runs and no token key changes.
func fastPasswords(t testing.TB, app core.App) {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte("1234567890"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	cols, err := app.FindAllCollections(core.CollectionTypeAuth)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cols {
		_, err := app.DB().NewQuery("UPDATE {{" + c.Name + "}} SET [[password]] = {:hash} WHERE [[password]] LIKE '$2a$13$%'").
			Bind(dbx.Params{"hash": string(hash)}).Execute()
		if err != nil {
			t.Fatal(err)
		}
	}
}
