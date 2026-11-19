package agentchat

import (
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"github.com/pockethost/pocketone/ext/extkit"
)

// The collections, named `_<extension>_<table>`. Both are system collections with no API rules: only superusers and
// Go/JS code reach them; the app decides what its users see through its own routes.
const (
	ChatsCollection      = "_agent_chat_chats"
	ChatsCollectionID    = "pbc_agent_chat_chats"
	MessagesCollection   = "_agent_chat_messages"
	MessagesCollectionID = "pbc_agent_chat_messages"
)

// Migrations implements extkit.Migrator: 0001 creates the chats and their messages.
func (*Extension) Migrations() []extkit.Migration {
	return []extkit.Migration{{Name: "0001_chats", Up: createChats, Down: dropChats}}
}

func createChats(tx core.App) error {
	chats := core.NewBaseCollection(ChatsCollection, ChatsCollectionID)
	chats.System = true
	chats.Fields.Add(
		&core.TextField{Id: "ach_title", Name: "title", Max: 500},
		&core.TextField{Id: "ach_summary", Name: "summary", Max: 20000, Help: "The messages folded out of the window, summarized."},
		&core.JSONField{Id: "ach_meta", Name: "meta", MaxSize: 1 << 16, Help: "The app's own JSON (a user id, a ticket id, ...)."},
		&core.NumberField{Id: "ach_folded", Name: "folded", OnlyInt: true, Help: "How many of the oldest messages the summary covers."},
		&core.AutodateField{Id: "ach_created", Name: "created", OnCreate: true, System: true},
		&core.AutodateField{Id: "ach_updated", Name: "updated", OnCreate: true, OnUpdate: true, System: true},
	)
	chats.AddIndex("idx_agent_chat_chats_updated", false, "updated", "")
	if err := tx.Save(chats); err != nil {
		return err
	}
	msgs := core.NewBaseCollection(MessagesCollection, MessagesCollectionID)
	msgs.System = true
	msgs.Fields.Add(
		&core.RelationField{Id: "acm_chat", Name: "chat", CollectionId: ChatsCollectionID, MaxSelect: 1, Required: true, CascadeDelete: true},
		&core.SelectField{Id: "acm_role", Name: "role", MaxSelect: 1, Required: true, Values: []string{"user", "assistant"}},
		&core.TextField{Id: "acm_content", Name: "content", Max: 1 << 20},
		&core.TextField{Id: "acm_sent", Name: "sent", Max: 1 << 20, Help: "A question as it was sent, with its context; later turns send it again this way so the model's state for the chat still matches."},
		&core.NumberField{Id: "acm_seq", Name: "seq", OnlyInt: true, Help: "Its place in the chat (0, 1, 2, ...)."},
		&core.AutodateField{Id: "acm_created", Name: "created", OnCreate: true, System: true},
	)
	msgs.AddIndex("idx_agent_chat_messages_chat_seq", true, "chat, seq", "")
	return tx.Save(msgs)
}

func dropChats(tx core.App) error {
	for _, name := range []string{MessagesCollection, ChatsCollection} {
		c, err := tx.FindCollectionByNameOrId(name)
		if err != nil {
			continue
		}
		c.System = false // a system collection can't be deleted
		if err := tx.SaveNoValidate(c); err != nil {
			return err
		}
		if err := tx.Delete(c); err != nil {
			return err
		}
	}
	return nil
}
