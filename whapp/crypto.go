package whapp

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"

	"golang.org/x/crypto/hkdf"
)

func decryptFile(fileBytes []byte, mediaKeyb64, cryptKey string) ([]byte, error) {
	mediaKey, err := base64.StdEncoding.DecodeString(mediaKeyb64)
	if err != nil {
		return []byte{}, err
	}

	cryptKeyBytes, err := hex.DecodeString(cryptKey)
	if err != nil {
		return []byte{}, err
	}

	hash := hkdf.New(sha256.New, mediaKey, nil, cryptKeyBytes)
	bytes := make([]byte, 112)
	if _, err = hash.Read(bytes); err != nil {
		return []byte{}, err
	}

	iv := bytes[:16]
	chiperKey := bytes[16 : 16+32]
	eFile := fileBytes[:len(fileBytes)-10]

	block, err := aes.NewCipher(chiperKey)
	if err != nil {
		return []byte{}, err
	}

	rawSize := len(eFile)
	for len(eFile)%aes.BlockSize != 0 {
		eFile = append(eFile, 0)
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(eFile, eFile)

	return eFile[:rawSize], nil
}
