package whapp

import (
	"context"
	"encoding/base64"
	"fmt"
	"os/exec"
	"time"

	"github.com/chromedp/chromedp"
)

type PhoneInfo struct {
	WhatsAppVersion    string `json:"wa_version"`
	OsVersion          string `json:"os_version"`
	DeviceManufacturer string `json:"device_manufacturer"`
	DeviceModel        string `json:"device_model"`
	OsBuildNumber      string `json:"os_build_number"`
}

type Me struct {
	LoginCode         string    `json:"ref"`
	LoginCodeTTL      int       `json:"refTTL"`
	Connected         bool      `json:"connected"`
	SelfID            string    `json:"me"`
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

type MessageID struct {
	FromMe     bool   `json:"fromMe"`
	ChatID     string `json:"remote"`
	ID         string `json:"id"`
	Serialized string `json:"_serialized"`
}

type Contact struct {
	ID                string `json:"id"`
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

type Participant struct {
	ID      string  `json:"id"`
	IsAdmin bool    `json:"isAdmin"`
	Contact Contact `json:"contact"`
}

type MediaPreview struct {
	RetainCount       int    `json:"_retainCount"`
	InAutoreleasePool bool   `json:"_inAutoreleasePool"`
	Released          bool   `json:"released"`
	Base64            string `json:"_b64"` // TODO bytes
	Mimetype          string `json:"_mimetype"`
}

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

type Message struct {
	ID         MessageID `json:"id"`
	Subtype    string    `json:"subtype"`
	Timestamp  int64     `json:"t"`
	NotifyName string    `json:"notifyName"`
	Sender     Contact   `json:"senderObj"`
	From       string    `json:"from"`
	To         string    `json:"to"`
	Body       string    `json:"body"`
	Self       string    `json:"self"`
	Ack        int       `json:"ack"`
	Invis      bool      `json:"invis"`
	Starred    bool      `json:"star"`

	Recipients []string `json:"recipients"`
	Mentioned  []string `json:"mentionedJidList"`

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
	MediaType      string    `json:"type"`
	MediaClientURL string    `json:"clientUrl"`
	MediaFileHash  string    `json:"filehash"`
	MediaFilename  string    `json:"filename"`
	Caption        string    `json:"caption"`

	Latitude       float64 `json:"lat"`
	Longitude      float64 `json:"lng"`
	LocationString string  `json:"loc"`

	QuotedMessageObject *Message `json:"quotedMsgObj"`

	Chat Chat `json:"chat"`
}

func (msg *Message) DownloadMedia() ([]byte, error) {
	var res []byte

	if msg.Body != "" {
		_, err := base64.StdEncoding.Decode(res, []byte(msg.Body))
		if err != nil {
			return res, err
		}
		return res, nil
	}

	clientURL := msg.MediaClientURL
	mediaKey := msg.MediaKey
	cryptKey := CryptKeys[msg.MediaType]

	cmd := exec.Command("python3", "./download.py", clientURL, mediaKey, cryptKey)
	bytes, err := cmd.Output()
	if err != nil {
		return res, err
	}

	_, err = base64.StdEncoding.Decode(res, bytes)
	if err != nil {
		return res, err
	}

	return res, nil
}

func (msg *Message) Content() string {
	res := msg.Body

	if msg.IsMedia {
		res = "-- file --"

		if msg.Caption != "" {
			res += " " + msg.Caption
		}
	}

	return res
}

func (msg *Message) Time() time.Time {
	return time.Unix(msg.Timestamp, 0)
}

type Presence struct {
	ID         string        `json:"id"`
	Chatstates []interface{} `json:"chatstates"`
}

type Chat struct {
	ID                    string    `json:"id"`
	PendingMsgs           bool      `json:"pendingMsgs"`
	LastReceivedMessageID MessageID `json:"lastReceivedKey"`
	Timestamp             int64     `json:"t"`
	UnreadCount           int       `json:"unreadCount"`
	Archive               bool      `json:"archive"`
	IsReadOnly            bool      `json:"isReadOnly"`
	ModifyTag             int       `json:"modifyTag"`
	MuteExpiration        int       `json:"muteExpiration"`
	Name                  string    `json:"name"`
	NotSpam               bool      `json:"notSpam"`
	Pin                   int       `json:"pin"`
	Kind                  string    `json:"kind"`
	Contact               Contact   `json:"contact"`
	Presence              Presence  `json:"presence"`

	IsGroupChat bool `json:"isGroup"`
}

func (c *Chat) Participants(ctx context.Context, wi *WhappInstance) ([]Participant, error) {
	var res []Participant

	if !c.IsGroupChat {
		return res, nil
	}

	if wi.LoginState != Loggedin {
		return res, fmt.Errorf("not logged in")
	}

	err := wi.inject(ctx)
	if err != nil {
		return res, err
	}

	str := fmt.Sprintf("whappGo.getGroupParticipants('%s')", c.ID)

	err = wi.CDP.Run(ctx, chromedp.Evaluate(str, &res, AwaitPromise))
	if err != nil {
		return res, err
	}

	fmt.Printf("%#v\n", res)

	return res, nil
}
