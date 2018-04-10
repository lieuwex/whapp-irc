import binascii
import aiohttp
from cryptography.hazmat.primitives.ciphers import Cipher, algorithms, modes
from cryptography.hazmat.backends import default_backend
from axolotl.kdf.hkdfv3 import HKDFv3
from axolotl.util.byteutil import ByteUtil
from base64 import b64decode, b64encode
from io import BytesIO
import asyncio
import sys


async def download_file(url):
    async with aiohttp.ClientSession() as session:
        async with session.get(url) as resp:
            return await resp.read()


async def download_media(client_url, media_key, crypt_key):
    file_data = await download_file(client_url)

    media_key = b64decode(media_key)
    derivative = HKDFv3().deriveSecrets(media_key,
                                        binascii.unhexlify(crypt_key),
                                        112)

    parts = ByteUtil.split(derivative, 16, 32)
    iv = parts[0]
    cipher_key = parts[1]
    e_file = file_data[:-10]

    cr_obj = Cipher(algorithms.AES(cipher_key), modes.CBC(iv), backend=default_backend())
    decryptor = cr_obj.decryptor()

    res = decryptor.update(e_file) + decryptor.finalize()
    sys.stdout.buffer.write(res)
    sys.stdout.buffer.flush()


loop = asyncio.get_event_loop()
loop.run_until_complete(download_media(sys.argv[1], sys.argv[2], sys.argv[3]))
