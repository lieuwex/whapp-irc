package whapp

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/runner"
)

// LoginState represents the login state of an Instance.
type LoginState int

const (
	// Loggedout is the state of a logged out instance.
	Loggedout LoginState = iota
	// Loggedin is the state of a logged in instance.
	Loggedin = iota
)

// Instance is an instance to Whatsapp Web.
type Instance struct {
	LoginState LoginState

	cdp      *chromedp.CDP
	injected bool
}

// MakeInstance makes a new Instance.
func MakeInstance(ctx context.Context, chromePath string) (*Instance, error) {
	options := chromedp.WithRunnerOptions(
		runner.Path(chromePath),
		runner.Port(9222),
		runner.KillProcessGroup,

		// runner.Flag("headless", true),
		runner.DisableGPU,
		runner.NoSandbox,

		runner.NoFirstRun,
		runner.NoDefaultBrowserCheck,
	)

	cdp, err := chromedp.New(ctx, options)
	if err != nil {
		return nil, err
	}

	return &Instance{
		LoginState: Loggedout,

		cdp:      cdp,
		injected: false,
	}, nil
}

// Open opens a tab with Whatsapp Web and returns the current login state.
func (wi *Instance) Open(ctx context.Context) (LoginState, error) {
	var state LoginState
	var loggedIn bool

	err := wi.cdp.Run(ctx, chromedp.Tasks{
		chromedp.Navigate(url),
		chromedp.WaitVisible("._2EZ_m, ._3ZW2E"),
		chromedp.Evaluate("document.getElementsByClassName('_3ZW2E').length > 0", &loggedIn),
	})
	if err != nil {
		return state, err
	}

	if loggedIn {
		state = Loggedin
	} else {
		state = Loggedout
	}

	wi.LoginState = state
	return state, nil
}

// GetLocalStorage retrieves and returns the localstorage of the current
// instance on the current tab.
// This method expects you to already have Whatsapp Web open.
func (wi *Instance) GetLocalStorage(ctx context.Context) (map[string]string, error) {
	var str string

	err := wi.cdp.Run(ctx, chromedp.Evaluate("JSON.stringify(localStorage)", &str))
	if err != nil {
		return nil, err
	}

	var res map[string]string

	if err := json.Unmarshal([]byte(str), &res); err != nil {
		return nil, err
	}

	return res, err
}

// SetLocalStorage adds all keys given by `localStorage` to the localStorage of
// the current instance.
func (wi *Instance) SetLocalStorage(ctx context.Context, localStorage map[string]string) error {
	var idc []byte

	tasks := chromedp.Tasks{chromedp.Navigate(url)}

	for key, val := range localStorage {
		str := fmt.Sprintf("localStorage.setItem(%s, %s)", strconv.Quote(key), strconv.Quote(val))
		tasks = append(tasks, chromedp.Evaluate(str, &idc))
	}

	return wi.cdp.Run(ctx, tasks)
}

// GetLoginCode retrieves the login code for the current instance.
// This can be used to generate a QR code which can be scanned using the
// Whatsapp mobile app.
func (wi *Instance) GetLoginCode(ctx context.Context) (string, error) {
	if wi.LoginState == Loggedin {
		return "", ErrLoggedIn
	}

	var code string
	var ok bool

	err := wi.cdp.Run(ctx, chromedp.Tasks{
		chromedp.WaitVisible("[alt='Scan me!']"), // wait for QR
		chromedp.AttributeValue("._2EZ_m", "data-ref", &code, &ok),
	})
	if err != nil {
		return "", err
	}

	if !ok {
		return "", ErrCDPUnknown
	}

	return code, nil
}

// WaitLogin waits until the current instance has been done logging in. (the
// user scanned the QR code and is accepted)
func (wi *Instance) WaitLogin(ctx context.Context) error {
	err := wi.cdp.Run(ctx, chromedp.WaitVisible("._3ZW2E"))
	if err != nil {
		panic(err)
	}
	wi.LoginState = Loggedin
	return nil
}

// GetMe returns the Me object for the current instance.
func (wi *Instance) GetMe(ctx context.Context) (Me, error) {
	var res Me

	if wi.LoginState != Loggedin {
		return res, ErrLoggedOut
	}

	err := wi.cdp.Run(ctx, chromedp.Evaluate("Store.Conn.toJSON()", &res))
	if err != nil {
		return res, err
	}

	return res, nil
}

func (wi *Instance) getLoggedIn(ctx context.Context) (bool, error) {
	var res bool
	action := chromedp.Evaluate("Store.Conn.clientToken != null", &res)
	return res, wi.cdp.Run(ctx, action)
}

// ListenLoggedIn listens for login state changes by polling it every
// `interval`.
func (wi *Instance) ListenLoggedIn(ctx context.Context, interval time.Duration) (<-chan bool, <-chan error) {
	// TODO: we could make this nicer with waiting on divs

	errCh := make(chan error)
	resCh := make(chan bool)

	go func() {
		defer close(errCh)
		defer close(resCh)

		prev := false
		isFirst := true

		for {
			if err := ctx.Err(); err != nil {
				errCh <- err
				return
			}

			res, err := wi.getLoggedIn(ctx)
			if err != nil {
				errCh <- err
				return
			}

			if res != prev && !isFirst {
				resCh <- res
			}

			prev = res
			isFirst = false

			time.Sleep(interval)
		}
	}()

	return resCh, errCh
}

func (wi *Instance) inject(ctx context.Context) error {
	if wi.injected {
		return nil
	}

	script := `
	var whappGo = {};

	whappGo.contactToJSON = function (contact) {
		if (contact == null) {
			return contact;
		}

		var res = contact.toJSON();

		res.id = contact.id;
		res.isMe = contact.isMe;
		res.formattedName = contact.formattedName;
		res.formattedShortName = contact.formattedShortName;
		res.profilePictureUrl = contact.profilePicThumb && contact.profilePicThumb.eurl;

		return res;
	};

	whappGo.participantToJSON = function (participant) {
		if (participant == null) {
			return participant;
		}

		return {
			id: participant.id,
			isAdmin: participant.isAdmin,
			contact: whappGo.contactToJSON(participant.contact),
		};
	}

	whappGo.msgToJSON = function (msg) {
		if (msg == null) {
			return msg;
		}

		var res = msg.toJSON();

		res.senderObj = whappGo.contactToJSON(msg.senderObj);
		res.content = msg.body;
		res.caption = msg.caption;
		res.isGroupMsg = msg.isGroupMsg;
		res.isLink = !!msg.isLink; // REVIEW
		res.isMMS = msg.isMMS;
		res.isMedia = msg.isMedia;
		res.isNotification = msg.isNotification;
		res.isPSA = msg.isPSA;
		res.isSentByMe = msg.isSentByMe;
		res.isSentByMeFromWeb = msg.isSentByMeFromWeb;
		res.type = msg.type;
		res.chat = whappGo.chatToJSON(msg.chat);
		res.chatId = msg.id.remote;
		res.quotedMsgObj = whappGo.msgToJSON(msg.quotedMsgObj());
		res.mediaData = msg.mediaData && msg.mediaData.toJSON();

		if (res.lat != null || res.lng != null) {
			res.location = {
				latitude: res.lat,
				longitude: res.lng,
				string: res.loc,
			};
		}

		return res;
	};

	whappGo.chatToJSON = function (chat) {
		if (chat == null) {
			return chat;
		}

		return Object.assign(chat.toJSON(), {
			kind: chat.kind,
			isGroup: chat.isGroup,
			contact: whappGo.contactToJSON(chat.contact),
			groupMetadata: chat.groupMetadata && chat.groupMetadata.toJSON(),
			presence: chat.presence && whappGo.presenceToJSON(chat.presence),
			msgs: null,
		});
	};

	whappGo.presenceToJSON = function (presence) {
		if (presence == null) {
			return presence;
		}

		return {
			timestamp: presence.t,
			type: presence.type,
			id: presence.id,
			chatActive: presence.chatActive,
			hasData: presence.hasData,
			isGroup: presence.isGroup,
			isOnline: presence.isOnline,
			isUser: presence.isUser,
		};
	}

	whappGo.getNewMessages = function () {
		const chats = Store.Chat.models;
		let res = [];

		for (const chat of chats) {
			const messages = chat.msgs.models;
			for (let i = messages.length - 1; i >= 0; i--) {
				let msg = messages[i];
				if (!msg.__x_isNewMsg) {
					break;
				}

				msg.__x_isNewMsg = false;

				if (
					(msg.isMedia && !msg.clientUrl) ||
					(msg.type === 'location' && !msg.body)
				) {
					continue;
				}

				msg = whappGo.msgToJSON(msg);

				console.log(msg);
				res.unshift(msg);
			}
		}

		return res;
	};

	whappGo.sendMessage = function (id, message, replyID) {
		/*
		var splitted = replyID.split('_');
		var replyObj = {
			contextInfo: function () {
				return {
					"quotedMsg": {
						"type": "gp2",
						"subtype": "leave",
					},
					"quotedStanzaID": splitted[2],
					"quotedRemoteJid": splitted[1],
				}
			},
		};
		*/

		const chat = Store.Chat.models.find(c => c.id === id);
		if (chat == null) {
			throw new Error('no chat with id ' + id + ' found.');
		}

		function sleep (ms) {
			return new Promise(resolve => setTimeout(resolve, ms));
		}

		chat.sendMessage(message/*, {}, replyObj*/).then(function () {
			var trials = 0;

			function trySend() {
				for (let i = chat.msgs.models.length - 1; i >= 0; i--) {
					let msg = chat.msgs.models[i];

					if (!msg.senderObj.isMe || msg.body != message) {
						continue;
					}

					return true;
				}

				if (++trials > 40) { // 20s
					// too much trials
					return;
				}

				sleep(500).then(trySend); // try again
			}

			trySend();
		});
	};

	whappGo.getGroupParticipants = async function (id) {
		const res = Store.GroupMetadata.models.find(md => md.id === id);

		// TODO: user should be able to just get the stale one and call
		// .Update() in the go code.
		if (res != null && res.stale) {
			await res.update();
		}

		return res.participants.map(p => whappGo.participantToJSON(p));
	};

	whappGo.getAllChats = function () {
		return Store.Chat.models.map(c => whappGo.chatToJSON(c));
	};

	whappGo.getPresence = async function (chatId) {
		const res = Store.Presence.models.find(p => p.id === chatId);
		await res.update();
		return whappGo.presenceToJSON(res);
	}

	whappGo.getPhoneActive = function () {
		return Store.Stream.phoneActive;
	};
	`

	var idc []byte
	if err := wi.cdp.Run(ctx, chromedp.Evaluate(script, &idc)); err != nil {
		return err
	}

	wi.injected = true
	return nil
}

func (wi *Instance) getNewMessages(ctx context.Context) ([]Message, error) {
	var res []Message

	if wi.LoginState != Loggedin {
		return res, ErrLoggedOut
	}

	if err := wi.inject(ctx); err != nil {
		return res, err
	}

	err := wi.cdp.Run(ctx, chromedp.Evaluate("whappGo.getNewMessages()", &res))
	if err != nil {
		return res, err
	}

	sort.SliceStable(res, func(i, j int) bool {
		return res[i].Timestamp < res[j].Timestamp
	})

	return res, err
}

// ListenForMessages listens for new messages by polling every `interval`.
func (wi *Instance) ListenForMessages(ctx context.Context, interval time.Duration) (<-chan Message, <-chan error) {
	// REVIEW: is this still correct when we get logged out?

	errCh := make(chan error)
	messageCh := make(chan Message)

	go func() {
		defer close(errCh)
		defer close(messageCh)

		for {
			if err := ctx.Err(); err != nil {
				errCh <- err
				return
			}

			res, err := wi.getNewMessages(ctx)
			if err != nil {
				errCh <- err
				return
			}

			for _, msg := range res {
				if msg.IsNotification {
					// TODO
					continue
				}

				messageCh <- msg
			}

			time.Sleep(interval)
		}
	}()

	return messageCh, errCh
}

// SendMessageToChatID sends the given `message` to the chat with the given
// `chatID`.
func (wi *Instance) SendMessageToChatID(ctx context.Context, chatID string, message string) error {
	// REVIEW: make this safe.
	// REVIEW: find some better way than 'idc'

	if wi.LoginState != Loggedin {
		return ErrLoggedOut
	}

	if err := wi.inject(ctx); err != nil {
		return err
	}

	str := fmt.Sprintf("whappGo.sendMessage(%s, %s)", strconv.Quote(chatID), strconv.Quote(message))

	var idc []byte
	return wi.cdp.Run(ctx, chromedp.Evaluate(str, &idc))
}

// GetAllChats returns a slice containing all the chats the user has
// participated in.
func (wi *Instance) GetAllChats(ctx context.Context) ([]Chat, error) {
	var res []Chat

	if wi.LoginState != Loggedin {
		return res, ErrLoggedOut
	}

	if err := wi.inject(ctx); err != nil {
		return res, err
	}

	err := wi.cdp.Run(ctx, chromedp.Evaluate("whappGo.getAllChats()", &res))
	if err != nil {
		return res, err
	}

	return res, nil
}

// GetPhoneActive returns Whether or not the user's phone is active.
func (wi *Instance) GetPhoneActive(ctx context.Context) (bool, error) {
	var res bool

	if wi.LoginState != Loggedin {
		return res, ErrLoggedOut
	}

	if err := wi.inject(ctx); err != nil {
		return res, err
	}

	err := wi.cdp.Run(ctx, chromedp.Evaluate("whappGo.getPhoneActive()", &res))
	if err != nil {
		return res, err
	}

	return res, nil
}

// ListenForPhoneActiveChange listens for changes in the user's phone
// activity.
func (wi *Instance) ListenForPhoneActiveChange(ctx context.Context, interval time.Duration) (<-chan bool, <-chan error) {
	// REVIEW: is this still correct when we get logged out?

	errCh := make(chan error)
	resCh := make(chan bool)

	go func() {
		defer close(errCh)
		defer close(resCh)

		prev := false
		new := true

		for {
			if err := ctx.Err(); err != nil {
				errCh <- err
				return
			}

			res, err := wi.GetPhoneActive(ctx)
			if err != nil {
				errCh <- err
				return
			}

			if new || res != prev {
				prev = res
				new = false
				resCh <- res
			}

			time.Sleep(interval)
		}
	}()

	return resCh, errCh
}

// Shutdown shuts down the current Instance.
func (wi *Instance) Shutdown(ctx context.Context) error {
	if err := wi.cdp.Shutdown(ctx); err != nil {
		return err
	}
	return wi.cdp.Wait()
}
