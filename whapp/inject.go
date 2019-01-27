package whapp

import (
	"context"

	"github.com/chromedp/chromedp"
)

func (wi *Instance) inject(ctx context.Context) error {
	if wi.injected {
		return nil
	}

	script := `
	function ideq (a, b) {
		return a._serialized === b._serialized;
	}
	function idFromString (str) {
		const [ user, server ] = str.split('@');
		return { user, server, _serialized: str };
	}

	var whappGo = {};

	whappGo.setupStore = async function () {
		const fetchWebpack = function (id) {
			return new Promise(function (resolve) {
				var obj = {};
				obj[id] = function (x, y, z) {
					resolve(z('"' + id + '"'));
				};
				webpackJsonp([], obj, id);
			});
		};

		window.Store = await fetchWebpack('bcihgfbdeb');
		window.Store.Wap = await fetchWebpack('dgfhfgbdeb');
		window.Store.Conn = (await fetchWebpack('jfefjijii')).default;
		window.Store.Stream = (await fetchWebpack('djddhaidag')).default;
	};

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

		if (typeof res.body !== 'string') {
			res.body = '';
		}

		res.senderObj = whappGo.contactToJSON(msg.senderObj);
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
		res.recipients = msg.recipients;

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

		const metadata = chat.groupMetadata && chat.groupMetadata.toJSON();

		let description = null;
		if (metadata != null && metadata.desc) {
			description = {
				id: metadata.descId,
				desc: metadata.desc,
				owner: metadata.descOwner,
				time: metadata.descTime,
			};
		}

		return {
			id: chat.id,
			pendingMsgs: chat.pendingMsgs,
			lastReceivedKey: chat.lastReceivedKey,
			t: chat.t,
			unreadCount: chat.unreadCount,
			archive: chat.archive,
			isReadOnly: chat.isReadOnly,
			muteInfo: {
				isMuted: chat.mute.isMuted,
				expiration: chat.mute.expiration,
			},
			name: chat.name,
			notSpam: chat.notSpam,
			pin: chat.pin,

			kind: chat.kind,
			isGroup: chat.isGroup,
			contact: whappGo.contactToJSON(chat.contact),
			groupMetadata: metadata,
			description: description,
			presence: chat.presence && whappGo.presenceToJSON(chat.presence),
		};
	};

	whappGo.presenceToJSON = function (presence) {
		if (presence == null) {
			return presence;
		}

		return {
			timestamp: presence.chatstate && presence.chatstate.t,
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
			if (chat == null) {
				continue;
			}

			const messages = chat.msgs.models;
			for (let i = messages.length - 1; i >= 0; i--) {
				let msg = messages[i];
				if (msg == null) {
					continue;
				}

				if (!msg.isNewMsg) {
					break;
				}
				msg.isNewMsg = false;

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

		id = idFromString(id);

		const chat = Store.Chat.models.find(c => ideq(c.id, id));
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
		id = idFromString(id);
		const res = Store.GroupMetadata.models.find(md => ideq(md.id, id));

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
		chatId = idFromString(chatId);
		const res = Store.Presence.models.find(p => ideq(p.id, chatId));
		await res.update();
		return whappGo.presenceToJSON(res);
	}

	whappGo.getPhoneActive = function () {
		return Store.Stream.phoneActive;
	};

	whappGo.getMessagesFromChatTillDate = async function (chatId, timestamp) {
		chatId = idFromString(chatId);
		const chat = Store.Chat.models.find(c => ideq(c.id, chatId));

		while (
			chat.msgs.models[0].t > timestamp &&
			!chat.msgs.msgLoadState.noEarlierMsgs
		) {
			await chat.loadEarlierMsgs();
		}

		// TODO: optimize this
		return chat.msgs.models
			.filter(m => m.t >= timestamp)
			.map(whappGo.msgToJSON);
	};

	whappGo.getCommonGroups = async function (contactId) {
		contactId = idFromString(contactId);

		const contact = Store.Contact.models.find(c => ideq(c.id, contactId));
		await contact.findCommonGroups();

		if (contact.commonGroups == null) {
			return [];
		}

		return contact.commonGroups.models.map(whappGo.chatToJSON);
	};

	whappGo.setAdmin = function (chatId, userId, admin) {
		chatId = idFromString(chatId);
		userId = idFromString(userId);

		const fn = admin ? 'promoteParticipant' : 'demoteParticipant';
		return Store.Wap[fn](chatId, userId);
	}

	whappGo.addParticipant = function (chatId, userId) {
		chatId = idFromString(chatId);
		userId = idFromString(userId);
		return Store.Wap.addParticipant(chatId, userId);
	}

	whappGo.removeParticipant = function (chatId, userId) {
		chatId = idFromString(chatId);
		userId = idFromString(userId);
		return Store.Wap.removeParticipant(chatId, userId);
	}
	`

	var idc []byte
	if err := wi.cdp.Run(ctx, chromedp.Evaluate(script, &idc)); err != nil {
		return err
	}

	if err := wi.cdp.Run(
		ctx,
		chromedp.Evaluate("whappGo.setupStore()", &idc, awaitPromise),
	); err != nil {
		return err
	}

	wi.injected = true
	return nil
}
