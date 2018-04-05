package whapp

import (
	"context"
	"fmt"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/runner"
)

type LoginState int

const (
	Loggedout LoginState = iota
	Loggedin             = iota
)

type WhappInstance struct {
	CDP        *chromedp.CDP
	LoginState LoginState
	Messages   <-chan Message

	injected bool
	id       string
}

func MakeWhappInstance(ctx context.Context) (*WhappInstance, error) {
	cdp, err := chromedp.New(ctx, chromedp.WithRunnerOptions(runner.ExecPath("/Applications/Chromium.app/Contents/MacOS/Chromium"), runner.Port(9222)))
	if err != nil {
		return nil, err
	}

	return &WhappInstance{
		CDP:        cdp,
		LoginState: Loggedout,

		injected: false,
		id:       "TODO",
	}, nil
}

func (wi *WhappInstance) Open(ctx context.Context) (LoginState, error) {
	var state LoginState
	var loggedIn bool

	err := wi.CDP.Run(ctx, chromedp.Tasks{
		chromedp.Navigate(URL),
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

func (wi *WhappInstance) GetLoginCode(ctx context.Context) (string, error) {
	// REVIEW: check if not loggedin?

	var code string
	var ok bool

	err := wi.CDP.Run(ctx, chromedp.Tasks{
		chromedp.WaitVisible("._2EZ_m"), // wait for QR
		chromedp.AttributeValue("._2EZ_m", "data-ref", &code, &ok),
	})
	if err != nil {
		return "", err
	}

	if !ok {
		return "", fmt.Errorf("not ok")
	}

	return code, nil
}

func (wi *WhappInstance) WaitLogin(ctx context.Context) error {
	err := wi.CDP.Run(ctx, chromedp.WaitVisible("._3ZW2E"))
	if err != nil {
		panic(err)
	}
	wi.LoginState = Loggedin
	return nil
}

func (wi *WhappInstance) GetMe(ctx context.Context) (Me, error) {
	var res Me

	if wi.LoginState != Loggedin {
		return res, fmt.Errorf("not logged in")
	}

	err := wi.CDP.Run(ctx, chromedp.Evaluate("Store.Conn.toJSON()", &res))
	if err != nil {
		return res, err
	}

	return res, nil
}

/*
func (wi *WhappInstance) ListenConnectionState(ctx context.Context, stateCh chan ConnectionState) error {

}
*/

func (wi *WhappInstance) inject(ctx context.Context) error {
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
		res.isLink = msg.isLink;
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
			presence: chat.presence && chat.presence.toJSON(),
			msgs: null,
		});
	};

	whappGo.buildEvent = function (event, args) {
		return JSON.stringify({
			event: event,
			args: args,
		});
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
				msg = whappGo.msgToJSON(msg);
				console.log(msg);
				res.push(msg);
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

		const chat = Store.Chat.models.find(c => c.__x_id === id);

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
		console.log(id, res);

		if (res != null && res.stale) {
			console.log('updating');
			await res.update();
		}

		const x= res.participants.map(p => whappGo.participantToJSON(p));
		console.log(x);
		return x;
	};

	whappGo.getAllChats = function () {
		return Store.Chat.models.map(c => whappGo.chatToJSON(c));
	};
	`

	var idc []byte
	err := wi.CDP.Run(ctx, chromedp.Evaluate(script, &idc))
	if err != nil {
		return err
	}

	wi.injected = true
	return nil
}

func (wi *WhappInstance) ListenForMessages(ctx context.Context, messagesCh chan Message, interval time.Duration) error {
	// REVIEW: is this still correct when we get logged out?

	if wi.LoginState != Loggedin {
		return fmt.Errorf("not logged in")
	}

	err := wi.inject(ctx)
	if err != nil {
		return err
	}

	for {
		if ctx.Err() != nil {
			return nil
		}

		var res []Message

		err := wi.CDP.Run(ctx, chromedp.Evaluate("whappGo.getNewMessages()", &res))
		if err != nil {
			return err
		}

		for _, msg := range res {
			messagesCh <- msg
		}

		time.Sleep(interval)
	}
}

func (wi *WhappInstance) SendMessageToChatID(ctx context.Context, chatID string, message string) error {
	// REVIEW: make this safe.
	// REVIEW: find some better way than 'idc'

	if wi.LoginState != Loggedin {
		return fmt.Errorf("not logged in")
	}

	err := wi.inject(ctx)
	if err != nil {
		return err
	}

	str := fmt.Sprintf("whappGo.sendMessage('%s', '%s')", chatID, message)

	var idc []byte
	err = wi.CDP.Run(ctx, chromedp.Evaluate(str, &idc))
	if err != nil {
		return err
	}

	return nil
}

func (wi *WhappInstance) GetAllChats(ctx context.Context) ([]*Chat, error) {
	var res []*Chat

	if wi.LoginState != Loggedin {
		return res, fmt.Errorf("not logged in")
	}

	err := wi.inject(ctx)
	if err != nil {
		return res, err
	}

	err = wi.CDP.Run(ctx, chromedp.Evaluate("whappGo.getAllChats()", &res))
	if err != nil {
		return res, err
	}

	return res, nil
}
