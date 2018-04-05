import time
import sys
import logging
import json
import base64
import asyncio
import os

sys.path.append(os.getcwd() + "/webwhatsapi")
from webwhatsapi.async_driver import WhatsAPIDriverAsync
from webwhatsapi.objects.message import Message, MediaMessage, MessageGroup, NotificationMessage
from webwhatsapi.objects.chat import UserChat, GroupChat

logger = logging.getLogger('whapp-irc')
logger.setLevel(logging.DEBUG)

evloop = asyncio.get_event_loop()
driver = WhatsAPIDriverAsync(headless=False, logger=logger, loop=evloop, profile="./profile")
print(dir(driver))

reader = None
writer = None

driverLock = asyncio.Lock()


def ev(event, *arg):
    str = json.dumps({
        "event": event,
        "args": arg,
    }) + "\n"
    writer.write(str.encode())


def format_contact(contact):
    return {
        "id": contact.id,
        "names": {
            "short": contact.short_name,
            "push": contact.push_name,
            "formatted": contact.formatted_name,
        }
    }


def format_chat(chat):
    if isinstance(chat, UserChat):
        return {
            "id": chat.id,
            "name": chat.name,
            "isGroupChat": False,
        }
    elif isinstance(chat, GroupChat):
        return {
            "id": chat.id,
            "name": chat.name,
            "isGroupChat": True,
            "participants": [format_contact(c) for c in chat.get_participants()],
            "admins": [format_contact(c) for c in chat.get_admins()],
        }


def format_date(date):
    return date.isoformat()


async def format_msg(msg):
    body = "--- removed message ---"
    if hasattr(msg, 'content'):
        body = msg.content

    if isinstance(msg, MediaMessage):
        try:
            data = await driver.download_media(msg)
            str = base64.b64encode(data.getbuffer()).decode()
        except:
            str = ""

        caption = msg._js_obj["caption"]

        return {
            "id": msg.id,
            "isMedia": True,
            "isSentByMe": msg._js_obj["isSentByMe"],
            "isSentByMeFromWeb": msg._js_obj["isSentByMeFromWeb"],
            "quotedMsgObj": msg._js_obj["quotedMsgObj"],
            "timestamp": msg.timestamp.timestamp(),
            "sender": format_contact(msg.sender),
            "filename": msg.filename,
            "body": str,
            "caption": caption,
            # "keys": {
            #     "client_url": msg.client_url,
            #     "media_key": msg.media_key,
            #     "crypt_keys": msg.crypt_keys,
            # },
        }
    elif isinstance(msg, NotificationMessage):
        # TODO
        return {
            "id": msg.id,
            "isNotif": True,
            "isSentByMe": msg._js_obj["isSentByMe"],
            "isSentByMeFromWeb": msg._js_obj["isSentByMeFromWeb"],
            "quotedMsgObj": msg._js_obj["quotedMsgObj"],
            "timestamp": msg.timestamp.timestamp(),
            "sender": format_contact(msg.sender),
            "body": repr(msg),
        }
    elif isinstance(msg, Message):
        return {
            "id": msg.id,
            "isText": True,
            "isSentByMe": msg._js_obj["isSentByMe"],
            "isSentByMeFromWeb": msg._js_obj["isSentByMeFromWeb"],
            "quotedMsgObj": msg._js_obj["quotedMsgObj"],
            "timestamp": msg.timestamp.timestamp(),
            "sender": format_contact(msg.sender),
            "body": body,
        }


async def format_msg_group(msgGroup):
    res, _ = await asyncio.wait([format_msg(m) for m in msgGroup.messages])
    res = [x.result() for x in list(res)]
    res.sort(key=lambda x: x['timestamp'])

    return {
        "chat": format_chat(msgGroup.chat),
        "messages": res,
    }


async def loopReceive(reader, writer):
    while True:
        res = await reader.readline()
        msg = json.loads(res)
        cmd = msg['command']

        if cmd == "send":
            chatId = msg['args'][0]
            content = msg['args'][1]
            replyID = msg['args'][2]

            with (await driverLock):
                await driver.chat_send_message(chatId, content, replyID)
                # ev("ok", {"id": "msg_send"})

        elif cmd == 'download':
            id = msg['args'][0]
            downloadInfo = msg['args'][1]
            data = await driver.download_media(downloadInfo)
            str = base64.b64encode(data.getbuffer())
            ev("download-ready", id, str.decode())

        elif cmd == 'eval':
            eval(msg['args'][0])


async def handleMessageGroup(msgGroup):
    ev("unread-messages", await format_msg_group(msgGroup))


async def loopMessages(reader, writer):
    while True:
        with (await driverLock):
            # res = await driver.get_unread(include_me=True, include_notifications=True)
            res = await driver.get_unread(include_me=True, include_notifications=False)

        for msgGroup in res:
            asyncio.ensure_future(handleMessageGroup(msgGroup), loop=evloop)
            handleMessageGroup(msgGroup)

        await asyncio.sleep(.5, loop=evloop)


async def get_qr_plain():
    try:
        fut = driver.loop.run_in_executor(driver._pool_executor, driver._driver.get_qr_plain)
        return await fut
    except asyncio.CancelledError:
        fut.cancel()
        raise


async def setup():
    global reader, writer

    port = sys.argv[1]
    id = sys.argv[2] + '\n'

    reader, writer = await asyncio.open_connection('127.0.0.1', port, loop=evloop)
    writer.write(id.encode())

    print(driver)

    await driver.connect()
    qr = await get_qr_plain()
    ev("qr", {"code": qr})
    await driver.wait_for_login()
    ev("ok", {"id": "login"})
    async for c in driver.get_all_chats():
        ev("chat", format_chat(c))

    await asyncio.wait([
        loopReceive(reader, writer),
        loopMessages(reader, writer),
    ], return_when=asyncio.FIRST_COMPLETED)

evloop.run_until_complete(setup())
