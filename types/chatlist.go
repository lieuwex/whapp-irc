package types

import (
	"fmt"
	"strings"
	"sync"
	"whapp-irc/whapp"
)

// getIdentifierPrefix returns the given identifier, but stripping the last _
// and everything after it. This is useful to strip the number from the
// identifier and thus getting the original identifier.
func getIdentifierPrefix(identifier string) string {
	// REVIEW: this luckily works since we don't have _ in channel names, but
	// actually we should have them (since they're perfectly legal in IRC), so
	// we have to find a way better way of doing this.

	i := strings.LastIndex(identifier, "_")
	if i > -1 {
		identifier = identifier[:i]
	}
	return identifier
}

// ChatListItem is the struct stored in a connection per chat item. It is also
// used to persist the Identifier<->ID mapping on disk.
type ChatListItem struct {
	Identifier string   `json:"identifier"`
	ID         whapp.ID `json:"id"`

	Chat *Chat `json:"-"`
}

// ChatList is a list of chats with some info.
type ChatList struct {
	mu    sync.RWMutex
	chats []ChatListItem
}

// ChatListFromSlice returns a new ChatList based on the given ChatListItem slice.
func ChatListFromSlice(chats []ChatListItem) *ChatList {
	return &ChatList{
		chats: chats,
	}
}

// Add adds the given chat to the current list.
func (l *ChatList) Add(chat *Chat) (res ChatListItem, isNew bool) {
	identifier := chat.Identifier()
	identifierLower := strings.ToLower(identifier)
	n := 0 // number of other chats with the same identifier

	l.mu.Lock()
	defer l.mu.Unlock()

	for i, item := range l.chats {
		// same chat as we already have, overwrite
		if item.ID == chat.ID {
			item.Chat = chat
			l.chats[i] = item
			return item, false
		}

		ident := getIdentifierPrefix(item.Identifier)
		if strings.ToLower(ident) == identifierLower {
			n++
		}
	}

	// if there's another chat with the same identifier, append an unique
	// number.
	if n > 0 {
		identifier = fmt.Sprintf("%s_%d", identifier, n+1)
	}

	// chat is new, append it to the list
	item := ChatListItem{
		Identifier: identifier,
		ID:         chat.ID,

		Chat: chat,
	}
	l.chats = append(l.chats, item)

	return item, true
}

// List returns a slice containing all the chats in the current list, if
// includeNil is true also items where the chat instance is nil will be
// returned.
func (l *ChatList) List(includeNil bool) []ChatListItem {
	l.mu.RLock()
	defer l.mu.RUnlock()

	res := make([]ChatListItem, 0, len(l.chats))
	for _, c := range l.chats {
		if c.Chat == nil && !includeNil {
			continue
		}

		res = append(res, c)
	}
	return res
}

// ByID returns the chat with the given ID, if any.
func (l *ChatList) ByID(id whapp.ID, allowNil bool) (item ChatListItem, found bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	for _, item := range l.chats {
		if item.ID != id {
			continue
		}

		if item.Chat != nil || allowNil {
			return item, true
		}
	}
	return ChatListItem{}, false
}

// ByIdentifier returns the chat with the given identifier, if any.
func (l *ChatList) ByIdentifier(identifier string, allowNil bool) (item ChatListItem, found bool) {
	identifier = strings.ToLower(identifier)

	l.mu.RLock()
	defer l.mu.RUnlock()

	for _, item := range l.chats {
		if strings.ToLower(item.Identifier) != identifier {
			continue
		}

		if item.Chat != nil || allowNil {
			return item, true
		}
	}
	return ChatListItem{}, false
}
