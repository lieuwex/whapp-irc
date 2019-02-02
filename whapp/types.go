package whapp

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

func resolveMentionsInString(body string, mentionedIDs []ID, participants []Participant, ownName string) string {
	for _, id := range mentionedIDs {
		for _, c := range participants {
			if c.ID == id {
				oldMention := "@" + id.User
				newMention := "@" + c.Contact.GetName()
				if c.Contact.IsMe {
					newMention = "@" + ownName
				}

				body = strings.Replace(body, oldMention, newMention, -1)

				break
			}
		}
	}

	return body
}

var numberRegex = regexp.MustCompile(`^\+[\d ]+$`)

// ID contains an ID of an user.
type ID struct {
	Server string `json:"server"`
	User   string `json:"user"`
}

func (id ID) String() string {
	return id.User + "@" + id.Server
}

// PhoneInfo contains info about the connected phone.
type PhoneInfo struct {
	WhatsAppVersion    string `json:"wa_version"`
	OsVersion          string `json:"os_version"`
	DeviceManufacturer string `json:"device_manufacturer"`
	DeviceModel        string `json:"device_model"`
	OsBuildNumber      string `json:"os_build_number"`
}

// Me contains info about the user logged in and their phone.
type Me struct {
	LoginCode         string    `json:"ref"`
	LoginCodeTTL      int       `json:"refTTL"`
	Connected         bool      `json:"connected"`
	SelfID            ID        `json:"me"`
	ProtocolVersion   []int     `json:"protoVersion"`
	ClientToken       string    `json:"clientToken"`
	ServerToken       string    `json:"serverToken"`
	BatteryPercentage int       `json:"battery"`
	PluggedIn         bool      `json:"plugged"`
	Location          string    `json:"lc"`
	Language          string    `json:"lg"`
	Uses24HourTime    bool      `json:"is24h"`
	Platform          string    `json:"platform"`
	Pushname          string    `json:"pushname"`
	Phone             PhoneInfo `json:"phone"`
}

// Contact represents a contact in the users contacts list.
type Contact struct {
	ID                ID     `json:"id"`
	Name              string `json:"name"`
	Type              string `json:"type"`
	PlaintextDisabled bool   `json:"plaintextDisabled"`
	StatusMute        bool   `json:"statusMute"`

	IsMe        bool `json:"isMe"`
	IsMyContact bool `json:"isMyContact"`
	IsPSA       bool `json:"isPSA"`
	IsUser      bool `json:"isUser"`
	IsWAContact bool `json:"isWAContact"`
	IsBusiness  bool `json:"isBusiness"`

	ProfilePictureURL string `json:"profilePictureUrl"`

	ShortName     string `json:"formattedShortName"`
	PushName      string `json:"pushname"`
	FormattedName string `json:"formattedName"`
}

// GetName returns the name of the current contact, tries to get the best
// matching name possible.
// When the contact doesn't have a name in the user's contacts, this function
// will try to use the PushName, if the contact has one.
func (c Contact) GetName() string {
	// REVIEW: better name

	str := c.FormattedName
	if numberRegex.MatchString(str) && c.PushName != "" {
		str = c.PushName
	}
	return str
}

// GetCommonGroups gets the groups both the logged-in user and the contact c are
// in.
func (c Contact) GetCommonGroups(ctx context.Context, wi *Instance) ([]Chat, error) {
	var res []Chat

	if wi.LoginState != Loggedin {
		return res, ErrLoggedOut
	}

	if err := wi.inject(ctx); err != nil {
		return res, err
	}

	str := fmt.Sprintf("whappGo.getCommonGroups(%s)", strconv.Quote(c.ID.String()))

	err := wi.cdp.Run(ctx, chromedp.Evaluate(str, &res, awaitPromise))
	return res, err
}

// Participant represents a participants in a group chat.
type Participant struct {
	ID           ID      `json:"id"`
	IsAdmin      bool    `json:"isAdmin"`
	IsSuperAdmin bool    `json:"isSuperAdmin"`
	Contact      Contact `json:"contact"`
}

// MessageID contains various IDs for a message.
type MessageID struct {
	FromMe     bool   `json:"fromMe"`
	ChatID     ID     `json:"remote"`
	ID         string `json:"id"`
	Serialized string `json:"_serialized"`
}

// MediaPreview contains information about the preview of a media message.
type MediaPreview struct {
	RetainCount       int    `json:"_retainCount"`
	InAutoreleasePool bool   `json:"_inAutoreleasePool"`
	Released          bool   `json:"released"`
	Base64            string `json:"_b64"` // TODO bytes
	Mimetype          string `json:"_mimetype"`
}

// MediaData contains information about the media of a media message.
type MediaData struct {
	Type        string       `json:"type"`
	MediaStage  string       `json:"mediaStage"`
	Size        int64        `json:"size"`
	Filehash    string       `json:"filehash"`
	Mimetype    string       `json:"mimetype"`
	FullHeight  int          `json:"fullHeight"`
	FullWidth   int          `json:"fullWidth"`
	AspectRatio float64      `json:"aspectRatio"`
	Preview     MediaPreview `json:"preview"`

	// TODO
	// MediaBlob   interface{}  `json:"mediaBlob"`
	// Streamable  bool         `json:"streamable"`
}

// LocationData contains information specific to a location message.
type LocationData struct {
	Latitude   float64 `json:"latitude"`
	Longitude  float64 `json:"longitude"`
	InfoString string  `json:"string"`
}

func (loc LocationData) String() string {
	return loc.InfoString
}

// Message represents any kind of message on Whatsapp.
// This also means the stuff like notifications (in the sense of e2e
// notifications, for example) are also represented by this struct.
type Message struct {
	ID         MessageID `json:"id"`
	Timestamp  int64     `json:"t"`
	NotifyName string    `json:"notifyName"`
	Sender     *Contact  `json:"senderObj"`
	From       ID        `json:"from"`
	To         ID        `json:"to"`
	Body       string    `json:"body"`
	Self       string    `json:"self"`
	Ack        int       `json:"ack"`
	Invis      bool      `json:"invis"`
	Starred    bool      `json:"star"`

	Type    string `json:"type"`
	Subtype string `json:"subtype"`

	RecipientIDs []ID `json:"recipients"`
	MentionedIDs []ID `json:"mentionedJidList"`

	IsGIF          bool `json:"isGif"`
	IsLive         bool `json:"isLive"`
	IsNewMessage   bool `json:"isNewMsg"`
	IsGroupMessage bool `json:"isGroupMsg"`
	IsLink         bool `json:"isLink"`
	IsMMS          bool `json:"isMMS"`
	IsNotification bool `json:"isNotification"`
	IsPSA          bool `json:"isPSA"`

	IsSentByMe        bool `json:"isSentByMe"`
	IsSentByMeFromWeb bool `json:"isSentByMeFromWeb"`

	IsMedia        bool      `json:"isMedia"`
	MediaData      MediaData `json:"mediaData"`
	MediaKey       string    `json:"mediaKey"` // make this nicer
	MimeType       string    `json:"mimetype"`
	MediaClientURL string    `json:"clientUrl"`
	MediaFileHash  string    `json:"filehash"`
	MediaFilename  string    `json:"filename"`
	Caption        string    `json:"caption"`

	Location *LocationData `json:"location"`

	PDFPageCount uint `json:"pageCount"`

	QuotedMessage *Message `json:"quotedMsgObj"`

	Chat Chat `json:"chat"`
}

// DownloadMedia downloads the media included in this message, if any
func (msg Message) DownloadMedia() ([]byte, error) {
	if !msg.IsMMS {
		return []byte{}, nil
	}

	fileBytes, err := downloadFile(msg.MediaClientURL)
	if err != nil {
		return []byte{}, err
	}

	return decryptFile(fileBytes, msg.MediaKey, getCryptKey(msg.Type))
}

// FormatBody returns the body of the current message, with mentions correctly
// resolved.
func (msg Message) FormatBody(participants []Participant, ownName string) string {
	if !msg.Chat.IsGroupChat {
		return msg.Body
	}

	return resolveMentionsInString(
		msg.Body,
		msg.MentionedIDs,
		participants,
		ownName,
	)
}

// FormatCaption returns the body of the current message, with mentions
// correctly resolved.
func (msg Message) FormatCaption(participants []Participant, ownName string) string {
	if !msg.Chat.IsGroupChat {
		return msg.Caption
	}

	return resolveMentionsInString(
		msg.Caption,
		msg.MentionedIDs,
		participants,
		ownName,
	)
}

// Content returns the body of the current message, with mentions correctly
// resolved with support for files (just prints "--file--") and their captions.
func (msg Message) Content(participants []Participant, ownName string) string {
	if msg.IsMMS {
		res := "--file--"

		if msg.Caption != "" {
			res += " " + msg.FormatCaption(participants, ownName)
		}

		return res
	}

	return msg.FormatBody(participants, ownName)
}

// Time returns the timestamp of the current message converted to a time.Time
// instance.
func (msg Message) Time() time.Time {
	return time.Unix(msg.Timestamp, 0)
}

// Presence contains information about the presence of a contact of the user.
type Presence struct {
	ID        ID     `json:"id"`
	Timestamp int64  `json:"timestamp"`
	Type      string `json:"type"`

	ChatActive bool `json:"chatActive"`
	HasData    bool `json:"hasData"`
	IsGroup    bool `json:"isGroup"`
	IsOnline   bool `json:"isOnline"`
	IsUser     bool `json:"isUser"`
}

// Time returns the timestamp of the current presence converted to a time.Time
// instance.
func (p Presence) Time() time.Time {
	return time.Unix(p.Timestamp, 0)
}

// A Description tells more about a group chat.
type Description struct {
	ID          string `json:"id"`
	Description string `json:"desc"`
	SetBy       ID     `json:"owner"`
	Timestamp   int64  `json:"time"`
}

// Time returns the timestamp of the current description converted to a
// time.Time instance.
func (d Description) Time() time.Time {
	return time.Unix(d.Timestamp, 0)
}

// MuteInfo contains information about the mute state of a chat.
type MuteInfo struct {
	IsMuted             bool  `json:"isMuted"`
	ExpirationTimestamp int64 `json:"expiration"`
}

// Expiration returns the time when the mute is removed.
func (i MuteInfo) Expiration() time.Time {
	return time.Unix(i.ExpirationTimestamp, 0)
}

// Chat represents a chat in WhatsApp.
type Chat struct {
	ID                    ID        `json:"id"`
	HasPendingMessages    bool      `json:"pendingMsgs"`
	LastReceivedMessageID MessageID `json:"lastReceivedKey"`
	Timestamp             int64     `json:"t"`
	UnreadCount           int       `json:"unreadCount"`
	Archived              bool      `json:"archive"`
	IsReadOnly            bool      `json:"isReadOnly"`
	MuteInfo              MuteInfo  `json:"muteInfo"`

	Name        string       `json:"name"`
	Description *Description `json:"description"`

	PinTimestamp int64 `json:"pin"`

	NotSpam  bool     `json:"notSpam"`
	Kind     string   `json:"kind"`
	Contact  Contact  `json:"contact"`
	Presence Presence `json:"presence"`

	IsGroupChat bool `json:"isGroup"`
}

// Title returns the name of the current chat, with support for contacts without
// a name.
func (c Chat) Title() string {
	res := c.Name
	if res == "" {
		res = c.Contact.GetName()
	}
	return res
}

// PinTime returns the timestamp when the current chat was pinned, and whether
// or not it is currently pinned.
func (c Chat) PinTime() (pinTime time.Time, set bool) {
	if t := c.PinTimestamp; t != 0 {
		return time.Unix(t, 0), true
	}
	return time.Time{}, false
}

// Participants retrieves and returns a slice containing all participants of the
// current group chat.
func (c Chat) Participants(ctx context.Context, wi *Instance) ([]Participant, error) {
	var res []Participant

	if !c.IsGroupChat {
		return res, nil
	}

	if wi.LoginState != Loggedin {
		return res, ErrLoggedOut
	}

	if err := wi.inject(ctx); err != nil {
		return res, err
	}

	str := fmt.Sprintf("whappGo.getGroupParticipants(%s)", strconv.Quote(c.ID.String()))

	err := wi.cdp.Run(ctx, chromedp.Evaluate(str, &res, awaitPromise))
	if err != nil {
		return res, err
	}

	return res, nil
}

// GetPresence retrieves and returns the presence of the current private chat.
func (c Chat) GetPresence(ctx context.Context, wi *Instance) (Presence, error) {
	// TODO REVIEW

	var res Presence

	if wi.LoginState != Loggedin {
		return res, ErrLoggedOut
	}

	if err := wi.inject(ctx); err != nil {
		return res, err
	}

	str := fmt.Sprintf("whappGo.getPresence(%s)", strconv.Quote(c.ID.String()))

	err := wi.cdp.Run(ctx, chromedp.Evaluate(str, &res, awaitPromise))
	if err != nil {
		return res, err
	}

	return res, nil
}

// SetAdmin sets the admin state of the user with given userID in the current
// chat.
func (c Chat) SetAdmin(ctx context.Context, wi *Instance, userID ID, setAdmin bool) error {
	str := fmt.Sprintf(
		"whappGo.setAdmin(%s, %s, %t)",
		strconv.Quote(c.ID.String()),
		strconv.Quote(userID.String()),
		setAdmin,
	)
	return runLoggedinWithoutRes(ctx, wi, str, false) // TODO: true?
}

// AddParticipant adds the user with the given userID to the current chat.
func (c Chat) AddParticipant(ctx context.Context, wi *Instance, userID ID) error {
	str := fmt.Sprintf(
		"whappGo.addParticipant(%s, %s)",
		strconv.Quote(c.ID.String()),
		strconv.Quote(userID.String()),
	)
	return runLoggedinWithoutRes(ctx, wi, str, false) // TODO: true?
}

// RemoveParticipant removes the user with the given userID from the current
// chat.
func (c Chat) RemoveParticipant(ctx context.Context, wi *Instance, userID ID) error {
	str := fmt.Sprintf(
		"whappGo.removeParticipant(%s, %s)",
		strconv.Quote(c.ID.String()),
		strconv.Quote(userID.String()),
	)
	return runLoggedinWithoutRes(ctx, wi, str, false) // TODO: true?
}

// GetMessagesFromChatTillDate returns messages in the current chat with a
// timestamp equal to or greater than `timestamp`.
func (c Chat) GetMessagesFromChatTillDate(
	ctx context.Context,
	wi *Instance,
	timestamp int64,
) ([]Message, error) {
	var res []Message

	if wi.LoginState != Loggedin {
		return res, ErrLoggedOut
	}

	if err := wi.inject(ctx); err != nil {
		return res, err
	}

	str := fmt.Sprintf(
		"whappGo.getMessagesFromChatTillDate(%s, %d)",
		strconv.Quote(c.ID.String()),
		timestamp,
	)
	if err := wi.cdp.Run(
		ctx,
		chromedp.Evaluate(str, &res, awaitPromise),
	); err != nil {
		return res, err
	}

	sort.SliceStable(res, func(i, j int) bool {
		return res[i].Timestamp < res[j].Timestamp
	})

	return res, nil
}
