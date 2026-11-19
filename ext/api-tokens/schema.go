package apitokens

import (
	"errors"
	"fmt"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"pocketone.dev/ext/extkit"
)

// The schema is a contract: the collection and field ids never change, so other code (a legacy migration, the
// mothership) can create the same collection first and [EnsureSchema] accepts it.
const (
	Collection   = "_api_tokens_tokens"
	CollectionId = "pbc_tokens"
)

// Migrations implements extkit.Migrator. 0001 moves the collections to this extension's prefix: the extension was
// named tokens, and its tables were _tokens, _tokens_oauth_clients and _tokens_oauth_codes. The ids stay.
func (p *Extension) Migrations() []extkit.Migration {
	return []extkit.Migration{
		extkit.RenameCollection("0001_rename_tokens", CollectionId, "_tokens", Collection),
		extkit.RenameCollection("0002_rename_oauth_clients", ClientsCollectionId, "_tokens_oauth_clients", ClientsCollection),
		extkit.RenameCollection("0003_rename_oauth_codes", CodesCollectionId, "_tokens_oauth_codes", CodesCollection),
	}
}

// ownRule lets an auth record list, view and delete its own tokens through the records API.
const ownRule = `@request.auth.id != "" && owner = @request.auth.id && owner_collection = @request.auth.collectionId`

func tokensCollection() *core.Collection {
	c := core.NewBaseCollection(Collection, CollectionId)
	c.System = true
	own := ownRule
	c.ListRule, c.ViewRule, c.DeleteRule = &own, &own, &own
	c.CreateRule, c.UpdateRule = nil, nil // superusers only; tokens are created through POST /api/_c/api-tokens/v1
	c.Fields.Add(
		&core.TextField{Id: "tok_owner_collection", Name: "owner_collection", Required: true, Max: 100,
			Help: "Id of the owner's auth collection."},
		&core.TextField{Id: "tok_owner", Name: "owner", Required: true, Max: 100, Help: "Id of the owner record."},
		&core.TextField{Id: "tok_name", Name: "name", Max: 200},
		&core.TextField{Id: "tok_hash", Name: "hash", Required: true, Hidden: true, Max: 100,
			Help: "sha256 of the secret (hex). The secret itself is never stored."},
		&core.TextField{Id: "tok_prefix", Name: "prefix", Max: 40, Help: "The first characters of the secret, for display."},
		&core.JSONField{Id: "tok_scopes", Name: "scopes", MaxSize: 1 << 12, Help: `["deploy"]; empty means the owner's full rights.`},
		&core.DateField{Id: "tok_expires", Name: "expires"},
		&core.DateField{Id: "tok_last_used", Name: "last_used"},
		&core.TextField{Id: "tok_last_ip", Name: "last_ip", Max: 100},
		&core.TextField{Id: "tok_client_id", Name: "client_id", Max: 200, Help: "The OAuth client that issued it, if any."},
		&core.TextField{Id: "tok_audience", Name: "audience", Max: 2000,
			Help: "RFC 8707 resource the token is bound to (it only works under that URL's path); empty: anywhere."},
		&core.AutodateField{Id: "tok_created", Name: "created", OnCreate: true, System: true},
		&core.AutodateField{Id: "tok_updated", Name: "updated", OnCreate: true, OnUpdate: true, System: true},
	)
	c.AddIndex("idx_tokens_hash", true, "hash", "")
	c.AddIndex("idx_tokens_owner", false, "owner_collection, owner", "")
	return c
}

// EnsureSchema creates the _api_tokens_tokens collection if it doesn't exist, and checks an existing one has the expected ids
// and fields. Safe on every start.
func EnsureSchema(app core.App) error {
	want := tokensCollection()
	have, err := app.FindCollectionByNameOrId(want.Id)
	if err != nil {
		if byName, nameErr := app.FindCollectionByNameOrId(want.Name); nameErr == nil {
			return fmt.Errorf("api-tokens: collection %q exists with id %q, expected %q", want.Name, byName.Id, want.Id)
		}
		if err := app.Save(want); err != nil {
			return fmt.Errorf("api-tokens: create %s: %w", want.Name, err)
		}
		return nil
	}
	if have.Name != want.Name {
		return fmt.Errorf("api-tokens: collection %q is named %q, expected %q", want.Id, have.Name, want.Name)
	}
	var errs []error
	added := false
	for _, wf := range want.Fields {
		hf := have.Fields.GetById(wf.GetId())
		if hf == nil {
			have.Fields.Add(wf) // a field a newer version added
			added = true
			continue
		}
		if hf.Type() != wf.Type() || hf.GetName() != wf.GetName() {
			errs = append(errs, fmt.Errorf("field %s is %s %q, expected %s %q",
				wf.GetId(), hf.Type(), hf.GetName(), wf.Type(), wf.GetName()))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("api-tokens: %s doesn't match the tokens schema: %w", want.Name, errors.Join(errs...))
	}
	if added {
		if err := app.Save(have); err != nil {
			return fmt.Errorf("api-tokens: add fields to %s: %w", want.Name, err)
		}
	}
	return nil
}
